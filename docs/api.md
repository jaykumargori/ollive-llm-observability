# API Documentation

Base URL: `http://localhost:8080`

## Create Conversation

`POST /api/conversations`

```json
{"title":"Inference trace","provider":"openai","model":"gpt-4.1-mini"}
```

Response:

```json
{"id":"uuid"}
```

## List Conversations

`GET /api/conversations`

Returns the 100 most recently updated conversations.

## Get Messages

`GET /api/conversations/{id}/messages`

Returns ordered conversation messages.

## Stream Message

`POST /api/conversations/{id}/messages`

```json
{"content":"Explain SSE in one paragraph","provider":"openai","model":"gpt-4.1-mini"}
```

Response content type: `text/event-stream`

Events:

```text
event: chunk
data: {"delta":"partial"}

event: done
data: {"ok":true}
```

## Metrics

`GET /api/metrics`

```json
{
  "avg_latency_ms": 812,
  "throughput_1h": 42,
  "error_rate": 2.3,
  "tokens": 12000,
  "providers": {"openai": 42}
}
```
