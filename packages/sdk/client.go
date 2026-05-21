package sdk

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

type LLMProvider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	Stream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}

type Client struct {
	provider LLMProvider
	nats     *nats.Conn
	subject  string
}

func New(provider LLMProvider, nc *nats.Conn, subject string) *Client {
	if subject == "" {
		subject = "inference.events"
	}
	return &Client{provider: provider, nats: nc, subject: subject}
}

func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	start := time.Now()
	resp, err := c.provider.Chat(ctx, req)
	event := baseEvent(req, start)
	if err != nil {
		event.Status = "error"
		event.Error = err.Error()
	} else {
		event.Status = "ok"
		event.OutputPreview = preview(resp.Content)
		event.InputTokens = resp.InputTokens
		event.OutputTokens = resp.OutputTokens
	}
	event.CompletedAt = time.Now()
	event.LatencyMS = event.CompletedAt.Sub(start).Milliseconds()
	c.emit(event)
	return resp, err
}

func (c *Client) Stream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	start := time.Now()
	upstream, err := c.provider.Stream(ctx, req)
	if err != nil {
		event := baseEvent(req, start)
		event.Status = "error"
		event.Error = err.Error()
		event.CompletedAt = time.Now()
		event.LatencyMS = event.CompletedAt.Sub(start).Milliseconds()
		c.emit(event)
		return nil, err
	}
	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		var b strings.Builder
		status := "ok"
		errText := ""
		for chunk := range upstream {
			if chunk.Error != "" {
				status = "error"
				errText = chunk.Error
			}
			if chunk.Delta != "" {
				b.WriteString(chunk.Delta)
			}
			select {
			case out <- chunk:
			case <-ctx.Done():
				status = "cancelled"
				errText = ctx.Err().Error()
				emitStreamEvent(c, req, start, status, errText, b.String())
				return
			}
		}
		emitStreamEvent(c, req, start, status, errText, b.String())
	}()
	return out, nil
}

func emitStreamEvent(c *Client, req ChatRequest, start time.Time, status, errText, output string) {
	event := baseEvent(req, start)
	event.Status = status
	event.Error = errText
	event.OutputPreview = preview(output)
	event.OutputTokens = estimateTokens(output)
	event.CompletedAt = time.Now()
	event.LatencyMS = event.CompletedAt.Sub(start).Milliseconds()
	c.emit(event)
}

func baseEvent(req ChatRequest, start time.Time) InferenceEvent {
	input := ""
	if len(req.Messages) > 0 {
		input = req.Messages[len(req.Messages)-1].Content
	}
	return InferenceEvent{
		ID:             newID(),
		ConversationID: req.ConversationID,
		Provider:       req.Provider,
		Model:          req.Model,
		InputPreview:   preview(input),
		InputTokens:    estimateTokens(input),
		Metadata:       req.Metadata,
		StartedAt:      start,
	}
}

func (c *Client) emit(event InferenceEvent) {
	if c.nats == nil {
		return
	}
	payload, err := json.Marshal(event)
	if err == nil {
		_ = c.nats.Publish(c.subject, payload)
	}
}

func preview(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 512 {
		return s[:512]
	}
	return s
}

func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return len([]rune(s))/4 + 1
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b[:])
}
