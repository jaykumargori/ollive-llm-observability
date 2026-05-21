package sdk

import "time"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	ConversationID string            `json:"conversation_id"`
	Provider       string            `json:"provider"`
	Model          string            `json:"model"`
	Messages       []Message         `json:"messages"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type ChatResponse struct {
	Content      string `json:"content"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

type StreamChunk struct {
	Delta string `json:"delta,omitempty"`
	Done  bool   `json:"done,omitempty"`
	Error string `json:"error,omitempty"`
}

type InferenceEvent struct {
	ID             string            `json:"id"`
	ConversationID string            `json:"conversation_id"`
	Provider       string            `json:"provider"`
	Model          string            `json:"model"`
	Status         string            `json:"status"`
	Error          string            `json:"error,omitempty"`
	LatencyMS      int64             `json:"latency_ms"`
	InputTokens    int               `json:"input_tokens"`
	OutputTokens   int               `json:"output_tokens"`
	InputPreview   string            `json:"input_preview"`
	OutputPreview  string            `json:"output_preview"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	StartedAt      time.Time         `json:"started_at"`
	CompletedAt    time.Time         `json:"completed_at"`
}
