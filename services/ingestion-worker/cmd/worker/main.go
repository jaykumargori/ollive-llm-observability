package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"ollive-llm-observability/packages/sdk"
	"ollive-llm-observability/services/ingestion-worker/internal/redact"
	"ollive-llm-observability/services/ingestion-worker/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	dbURL := env("DATABASE_URL", "postgres://ollive:ollive@localhost:5432/ollive?sslmode=disable")
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	nc, err := nats.Connect(env("NATS_URL", nats.DefaultURL), nats.Timeout(5*time.Second))
	if err != nil {
		log.Error("connect nats", "err", err)
		os.Exit(1)
	}
	defer nc.Close()
	st := store.New(pool)
	_, err = nc.QueueSubscribe("inference.events", "ingestion-workers", func(msg *nats.Msg) {
		var event sdk.InferenceEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			log.Warn("invalid event", "err", err)
			return
		}
		event.InputPreview = redact.Text(event.InputPreview)
		event.OutputPreview = redact.Text(event.OutputPreview)
		event.Error = redact.Text(event.Error)
		raw, _ := json.Marshal(event)
		if err := st.Save(context.Background(), event, raw); err != nil {
			log.Error("persist event", "event_id", event.ID, "err", err)
			return
		}
		log.Info("ingested inference", "event_id", event.ID, "provider", event.Provider, "status", event.Status)
	})
	if err != nil {
		log.Error("subscribe", "err", err)
		os.Exit(1)
	}
	_ = nc.Flush()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
