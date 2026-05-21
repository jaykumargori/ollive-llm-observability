package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"ollive-llm-observability/apps/api/internal/store"
	"ollive-llm-observability/packages/sdk"
)

type Handler struct {
	store *store.Store
	llm   *sdk.Client
	log   *slog.Logger
}

func New(store *store.Store, llm *sdk.Client, log *slog.Logger) *Handler {
	return &Handler{store: store, llm: llm, log: log}
}

func (h *Handler) Register(app *fiber.App) {
	app.Get("/healthz", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	app.Get("/api/conversations", h.listConversations)
	app.Post("/api/conversations", h.createConversation)
	app.Get("/api/conversations/:id/messages", h.messages)
	app.Post("/api/conversations/:id/messages", h.streamMessage)
	app.Get("/api/metrics", h.metrics)
}

func (h *Handler) listConversations(c *fiber.Ctx) error {
	items, err := h.store.ListConversations(c.Context())
	if err != nil {
		return fiber.NewError(500, err.Error())
	}
	return c.JSON(items)
}

func (h *Handler) createConversation(c *fiber.Ctx) error {
	var req struct {
		Title    string `json:"title"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(400, err.Error())
	}
	if req.Provider == "" {
		req.Provider = "openai"
	}
	if req.Model == "" {
		req.Model = "gpt-4.1-mini"
	}
	if strings.TrimSpace(req.Title) == "" {
		req.Title = "New conversation"
	}
	id, err := h.store.CreateConversation(c.Context(), req.Provider, req.Model, req.Title)
	if err != nil {
		return fiber.NewError(500, err.Error())
	}
	return c.JSON(fiber.Map{"id": id})
}

func (h *Handler) messages(c *fiber.Ctx) error {
	items, err := h.store.Messages(c.Context(), c.Params("id"))
	if err != nil {
		return fiber.NewError(500, err.Error())
	}
	return c.JSON(items)
}

func (h *Handler) metrics(c *fiber.Ctx) error {
	m, err := h.store.Metrics(c.Context())
	if err != nil {
		return fiber.NewError(500, err.Error())
	}
	return c.JSON(m)
}

func (h *Handler) streamMessage(c *fiber.Ctx) error {
	var req struct {
		Content  string `json:"content"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(400, err.Error())
	}
	conversationID := c.Params("id")
	if req.Provider == "" {
		req.Provider = "openai"
	}
	if req.Model == "" {
		req.Model = "gpt-4.1-mini"
	}
	if err := h.store.AddMessage(c.Context(), conversationID, "user", req.Content); err != nil {
		return fiber.NewError(500, err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	contextMsgs, err := h.store.RecentContext(ctx, conversationID, 8)
	if err != nil {
		cancel()
		return fiber.NewError(500, err.Error())
	}
	chunks, err := h.llm.Stream(ctx, sdk.ChatRequest{
		ConversationID: conversationID,
		Provider:       req.Provider,
		Model:          req.Model,
		Messages:       contextMsgs,
		Metadata:       map[string]string{"route": "sse"},
	})
	if err != nil {
		cancel()
		return fiber.NewError(502, err.Error())
	}
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer cancel()
		var answer strings.Builder
		enc := json.NewEncoder(w)
		writeEvent := func(name string, data any) bool {
			if _, err := fmt.Fprintf(w, "event: %s\n", name); err != nil {
				return false
			}
			if _, err := fmt.Fprint(w, "data: "); err != nil {
				return false
			}
			if err := enc.Encode(data); err != nil {
				return false
			}
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return false
			}
			return w.Flush() == nil
		}
		for chunk := range chunks {
			if chunk.Error != "" {
				h.log.Warn("provider stream error", "conversation_id", conversationID, "provider", req.Provider, "model", req.Model, "err", chunk.Error)
				_ = writeEvent("error", fiber.Map{"error": chunk.Error})
				return
			}
			if chunk.Delta != "" {
				answer.WriteString(chunk.Delta)
			}
			if !writeEvent("chunk", chunk) {
				if answer.Len() > 0 {
					if err := h.store.AddMessage(context.Background(), conversationID, "assistant", answer.String()); err != nil {
						h.log.Error("persist partial assistant message", "err", err)
					}
					h.store.Touch(context.Background(), conversationID)
				}
				h.log.Warn("sse client disconnected", "conversation_id", conversationID, "provider", req.Provider, "model", req.Model)
				cancel()
				return
			}
		}
		if answer.Len() > 0 {
			if err := h.store.AddMessage(context.Background(), conversationID, "assistant", answer.String()); err != nil {
				h.log.Error("persist assistant message", "err", err)
			}
			h.store.Touch(context.Background(), conversationID)
		}
		_ = writeEvent("done", fiber.Map{"ok": true})
	})
	return nil
}

func Logger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
