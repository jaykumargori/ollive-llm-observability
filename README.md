# Ollive LLM Observability

Lightweight LLM inference observability platform for streaming AI workloads. Ollive is closer to a tiny Langfuse/OpenTelemetry-style ingestion pipeline than a chatbot product: SSE streams stay on the hot path, telemetry goes through NATS asynchronously, and PostgreSQL stores normalized inference logs.

## Quick Start

```bash
cd infra
cp ../.env.example ../.env
OPENAI_API_KEY=sk-your-key docker compose up --build
```

Open the frontend at `http://localhost:5173` and the API at `http://localhost:8080`.

Do not commit real API keys. For repeatable local runs, put `OPENAI_API_KEY` in `../.env` and start with:

```bash
docker compose --env-file ../.env up --build
```

## Architecture

```text
SolidJS SPA
  -> Fiber API over SSE
  -> lightweight LLM SDK wrapper
  -> OpenAI GPT-4.1-mini

SDK inference event
  -> NATS subject inference.events
  -> ingestion-worker queue group
  -> PII redaction
  -> PostgreSQL inference_logs
```

## Features

- Multi-provider selection with OpenAI implemented and Claude/Gemini skeleton adapters.
- Server-Sent Events streaming with browser-side cancellation.
- Conversation list, resume, unique prompt-derived titles, optimistic messages, and retry.
- Lightweight latency, throughput, error rate, token, and provider usage dashboard with small native CSS bars.
- Async fire-and-forget ingestion over NATS.
- Regex PII redaction for emails, phone numbers, and common API key shapes.
- Docker Compose one-command setup.
- Lightweight Kubernetes manifests for API, worker, and web.

## API

- `GET /healthz` returns API health.
- `GET /api/conversations` lists recent conversations.
- `POST /api/conversations` creates a conversation.
- `GET /api/conversations/:id/messages` resumes message history.
- `POST /api/conversations/:id/messages` streams an assistant response as SSE.
- `GET /api/metrics` returns aggregate observability metrics.

SSE events:

```text
event: chunk
data: {"delta":"partial text"}

event: done
data: {"ok":true}
```

## Logging Strategy

The API logs structured request lifecycle events to stdout. The SDK captures provider, model, timestamps, latency, status, token estimates, conversation IDs, and input/output previews. The ingestion worker validates and redacts these events before persistence.

## Schema Design

The database intentionally uses a small relational schema with JSONB only where flexible metadata is useful:

- `conversations`: one row per session with provider/model defaults and an updated-at index for the sidebar.
- `messages`: append-only chat messages keyed by conversation. A `(conversation_id, id)` index supports fast resume in order.
- `ingestion_events`: raw normalized telemetry payloads from NATS, keyed by event ID for idempotent worker inserts.
- `inference_logs`: query-friendly inference records with provider, model, status, latency, token estimates, previews, timestamps, and JSONB metadata.

This avoids a heavy ORM and keeps dashboard queries simple SQL aggregations. Foreign keys preserve conversation/message integrity, while inference logs can survive conversation deletion with `on delete set null`.

## Scaling Considerations

- API instances are horizontally scalable because stream state lives in request context and durable state lives in Postgres.
- NATS queue groups let multiple ingestion workers share telemetry load.
- PostgreSQL indexes target conversation resume and recent metrics queries.
- Hot-path streaming avoids waiting for telemetry persistence.

## Tradeoffs

- Token counts are estimated for streamed responses to keep dependencies small.
- NATS core messaging is used for low latency; this demo does not attempt Kafka-level durability.
- Regex PII redaction is fast and pragmatic but not a full data-loss-prevention system.
- Metrics are simple SQL aggregations, not Prometheus/Grafana/OpenTelemetry collectors.

## Failure Handling

- If OpenAI fails, the stream returns an API error and the SDK emits an error telemetry event.
- If a browser cancels, request context cancellation stops upstream streaming and emits a cancelled event.
- If NATS publish fails, user response is not blocked.
- If ingestion persistence fails, the worker logs the failure; production hardening could add dead-letter handling.

## Screenshots

Suggested demo captures:

- `docs/screenshots/dashboard.png`
- `docs/screenshots/streaming-chat.png`

For a quick submission demo, record a short Loom showing Docker Compose startup, streaming response, cancel/retry, conversation resume, and the dashboard counters changing.

## Kubernetes

Lightweight manifests live in `infra/k8s`:

```bash
kubectl apply -f infra/k8s/configmap.yaml
kubectl create secret generic llm-secrets --from-literal=OPENAI_API_KEY=sk-your-key
kubectl apply -f infra/k8s/deployment.yaml
kubectl apply -f infra/k8s/service.yaml
```

The manifests assume Postgres and NATS services exist in-cluster. For a self-hosted demo, run them with your preferred lightweight setup such as k3s, k3d, or kind, then point `DATABASE_URL` and `NATS_URL` in the config map at those services.

## Future Improvements

- Real provider implementations for Claude and Gemini.
- Optional durable NATS JetStream mode.
- OpenTelemetry export bridge.
- Prometheus scrape endpoint.
- Better tokenizer integration per provider.
- Dead-letter queue and replay tooling.
