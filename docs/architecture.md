# Architecture Notes

## Request Lifecycle

The frontend creates or resumes a conversation, then posts a user message to the Fiber API. The API persists the user message, loads the most recent short context window, and calls the SDK wrapper with provider, model, conversation ID, and metadata.

## Streaming Lifecycle

Streaming uses Server-Sent Events only. The OpenAI provider reads streaming chat-completion frames and converts deltas into small `StreamChunk` values. The Fiber handler writes each chunk immediately to the browser as an SSE `chunk` event. Browser cancellation aborts the fetch, which cancels the request context and stops upstream work.

## Inference Logging Lifecycle

The SDK wraps provider calls and records:

- provider and model
- status and error text
- latency
- timestamps
- conversation ID
- token estimates
- input and output previews
- route metadata

It publishes one telemetry event to NATS after the call finishes, errors, or is cancelled. This keeps observability off the user-facing latency path.

## Async Ingestion Flow

The ingestion worker subscribes to `inference.events` with the `ingestion-workers` queue group. It validates JSON, applies regex PII redaction, stores the raw normalized event in `ingestion_events`, and writes query-friendly rows to `inference_logs`.

## Queue Decoupling Strategy

NATS gives the API a small, fast event bus without turning the demo into a distributed systems project. The API does not synchronously wait on Postgres logging writes. In production, JetStream could be enabled when replay and stronger delivery guarantees matter.

## Scaling Approach

- Add more API replicas for concurrent SSE streams.
- Add more ingestion workers for telemetry throughput.
- Keep Postgres as the system of record for conversations and logs.
- Add indexes only around observed query patterns.
- Keep provider adapters stateless.

## Retry Assumptions

The frontend supports retrying the last user request. The ingestion worker uses idempotent inserts with event IDs. There is no complex event sourcing or exactly-once pipeline because this project optimizes for clarity and low operational weight.

## Failure Handling

- Provider error: API returns an error and SDK emits error telemetry.
- Browser cancellation: API cancels context and SDK emits cancelled telemetry.
- NATS unavailable: inference can still complete, but telemetry may be dropped.
- Worker failure: NATS queue group can route future messages to another worker.
- Postgres unavailable: API fails conversation operations; worker logs persistence failures.

## Non-Goals

No auth, RBAC, LangChain, vector databases, CQRS, event sourcing, distributed tracing collector, or enterprise DDD layering. Those can be added later if the product shape justifies them.
