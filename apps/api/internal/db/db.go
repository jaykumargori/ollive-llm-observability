package db

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://ollive:ollive@localhost:5432/ollive?sslmode=disable"
	}
	return pgxpool.New(ctx, url)
}
