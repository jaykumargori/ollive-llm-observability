package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/nats-io/nats.go"
	"ollive-llm-observability/apps/api/internal/db"
	httpapi "ollive-llm-observability/apps/api/internal/http"
	"ollive-llm-observability/apps/api/internal/provider"
	"ollive-llm-observability/apps/api/internal/store"
	"ollive-llm-observability/packages/sdk"
)

func main() {
	log := httpapi.Logger()
	ctx := context.Background()
	pool, err := db.Connect(ctx)
	if err != nil {
		log.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	natsURL := env("NATS_URL", nats.DefaultURL)
	nc, err := nats.Connect(natsURL, nats.Timeout(5*time.Second))
	if err != nil {
		log.Error("connect nats", "err", err)
		os.Exit(1)
	}
	defer nc.Close()
	log.Info("api config", "openai_key_configured", os.Getenv("OPENAI_API_KEY") != "")
	llm := sdk.New(provider.NewRouter(os.Getenv("OPENAI_API_KEY")), nc, "inference.events")
	app := fiber.New(fiber.Config{DisableStartupMessage: true, ReadTimeout: 95 * time.Second})
	app.Use(cors.New(cors.Config{AllowOrigins: env("CORS_ORIGINS", "http://localhost:5173")}))
	app.Use(logger.New())
	httpapi.New(store.New(pool), llm, log).Register(app)
	go func() {
		if err := app.Listen(":" + env("PORT", "8080")); err != nil {
			log.Error("api stopped", "err", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = app.ShutdownWithTimeout(5 * time.Second)
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
