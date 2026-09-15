package database

import (
	"context"
	"fmt"
	"log"

	"github.com/Roy6lty/minibank/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, env config.EnvVariables) (*pgxpool.Pool, error) {
	if env.DB_HOST == "" || env.DB_PORT == "" || env.DB_USER == "" || env.DB_PASSWORD == "" || env.DB_NAME == "" {
		log.Fatal("Database environment variables are not set")
	}
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable", env.DB_HOST, env.DB_USER, env.DB_PASSWORD, env.DB_NAME, env.DB_PORT)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
