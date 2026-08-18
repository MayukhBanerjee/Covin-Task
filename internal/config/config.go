// Package config loads service configuration from the environment.
package config

import (
	"os"
	"strconv"
)

// Config holds everything the service needs to start.
type Config struct {
	HTTPAddr    string
	PostgresDSN string
	RedisAddr   string
	// DBMaxConns bounds the Postgres connection pool.
	DBMaxConns int32
}

// Load reads configuration from the environment, falling back to the
// defaults used by docker-compose.yml.
func Load() Config {
	return Config{
		HTTPAddr:    env("HTTP_ADDR", ":8080"),
		PostgresDSN: env("DATABASE_URL", "postgres://webhook:webhook@localhost:5432/webhook?sslmode=disable"),
		RedisAddr:   env("REDIS_ADDR", "localhost:6379"),
		DBMaxConns:  int32(envInt("DB_MAX_CONNS", 20)),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
