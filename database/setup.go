package database

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Roy6lty/minibank/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, env config.EnvVariables) (*pgxpool.Pool, error) {
	if env.DB_HOST == "" || env.DB_PORT == "" || env.DB_USER == "" || env.DB_PASSWORD == "" || env.DB_NAME == "" {
		return nil, fmt.Errorf("database environment variables are not set")
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		env.DB_HOST,
		env.DB_USER,
		env.DB_PASSWORD,
		env.DB_NAME,
		env.DB_PORT,
		env.DB_SSLMODE,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	poolConfig.ConnConfig.ConnectTimeout = env.DB_CONNECT_TIMEOUT
	if poolConfig.ConnConfig.RuntimeParams == nil {
		poolConfig.ConnConfig.RuntimeParams = make(map[string]string)
	}
	poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(env.DB_QUERY_TIMEOUT.Milliseconds(), 10)
	poolConfig.MaxConns = env.DB_MAX_CONNS
	poolConfig.MinConns = env.DB_MIN_CONNS
	poolConfig.MaxConnLifetime = env.DB_MAX_CONN_LIFETIME
	poolConfig.MaxConnIdleTime = env.DB_MAX_CONN_IDLE_TIME
	poolConfig.HealthCheckPeriod = env.DB_HEALTH_CHECK_PERIOD

	connectCtx, cancel := context.WithTimeout(ctx, env.DB_CONNECT_TIMEOUT)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
