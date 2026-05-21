package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
	"ollive-llm-observability/packages/sdk"
)

type Store struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Store { return &Store{db: db} }

func (s *Store) Save(ctx context.Context, event sdk.InferenceEvent, raw []byte) error {
	meta, _ := json.Marshal(event.Metadata)
	_, err := s.db.Exec(ctx, `
		insert into ingestion_events(event_id,subject,payload,status) values($1,$2,$3,'processed')
		on conflict(event_id) do nothing`,
		event.ID, "inference.events", raw)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `
		insert into inference_logs(
			id,conversation_id,provider,model,status,error,latency_ms,input_tokens,output_tokens,
			input_preview,output_preview,metadata,started_at,completed_at
		) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		on conflict(id) do nothing`,
		event.ID, event.ConversationID, event.Provider, event.Model, event.Status, event.Error,
		event.LatencyMS, event.InputTokens, event.OutputTokens, event.InputPreview, event.OutputPreview,
		meta, event.StartedAt, event.CompletedAt)
	return err
}
