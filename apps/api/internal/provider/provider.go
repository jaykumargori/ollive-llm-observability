package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ollive-llm-observability/packages/sdk"
)

type OpenAI struct {
	apiKey string
	client *http.Client
}

type Router struct {
	providers map[string]sdk.LLMProvider
}

func NewRouter(openAIKey string) *Router {
	return &Router{providers: map[string]sdk.LLMProvider{
		"openai": NewOpenAI(openAIKey),
		"claude": Claude{},
		"gemini": Gemini{},
	}}
}

func (r *Router) Chat(ctx context.Context, req sdk.ChatRequest) (*sdk.ChatResponse, error) {
	p, err := r.pick(req.Provider)
	if err != nil {
		return nil, err
	}
	return p.Chat(ctx, req)
}

func (r *Router) Stream(ctx context.Context, req sdk.ChatRequest) (<-chan sdk.StreamChunk, error) {
	p, err := r.pick(req.Provider)
	if err != nil {
		return nil, err
	}
	return p.Stream(ctx, req)
}

func (r *Router) pick(name string) (sdk.LLMProvider, error) {
	if name == "" {
		name = "openai"
	}
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return p, nil
}

func NewOpenAI(apiKey string) *OpenAI {
	return &OpenAI{apiKey: apiKey, client: &http.Client{Timeout: 90 * time.Second}}
}

func (o *OpenAI) Chat(ctx context.Context, req sdk.ChatRequest) (*sdk.ChatResponse, error) {
	stream, err := o.Stream(ctx, req)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	for chunk := range stream {
		if chunk.Error != "" {
			return nil, errors.New(chunk.Error)
		}
		b.WriteString(chunk.Delta)
	}
	return &sdk.ChatResponse{Content: b.String()}, nil
}

func (o *OpenAI) Stream(ctx context.Context, req sdk.ChatRequest) (<-chan sdk.StreamChunk, error) {
	if o.apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY is required")
	}
	model := req.Model
	if model == "" {
		model = "gpt-4.1-mini"
	}
	body := map[string]any{"model": model, "messages": req.Messages, "stream": true}
	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("openai status %d: %s", resp.StatusCode, string(b))
	}
	out := make(chan sdk.StreamChunk)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				out <- sdk.StreamChunk{Done: true}
				return
			}
			var parsed struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &parsed); err == nil {
				for _, choice := range parsed.Choices {
					if choice.Delta.Content != "" {
						out <- sdk.StreamChunk{Delta: choice.Delta.Content}
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			out <- sdk.StreamChunk{Error: err.Error()}
		}
	}()
	return out, nil
}

type Claude struct{}

func (Claude) Chat(context.Context, sdk.ChatRequest) (*sdk.ChatResponse, error) {
	return nil, errors.New("claude provider skeleton: not configured")
}
func (Claude) Stream(context.Context, sdk.ChatRequest) (<-chan sdk.StreamChunk, error) {
	return nil, errors.New("claude provider skeleton: streaming not implemented")
}

type Gemini struct{}

func (Gemini) Chat(context.Context, sdk.ChatRequest) (*sdk.ChatResponse, error) {
	return nil, errors.New("gemini provider skeleton: not configured")
}
func (Gemini) Stream(context.Context, sdk.ChatRequest) (<-chan sdk.StreamChunk, error) {
	return nil, errors.New("gemini provider skeleton: streaming not implemented")
}
