package db

import (
	"context"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	log.Printf("Opening database connection")
	if strings.Contains(connString, "@") {
		log.Printf("Database connection string contains '@'; using pgx parser for provided value")
	}

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	log.Println("Database connection established")
	return pool, nil
}
