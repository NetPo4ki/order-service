package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL       string
	RedisAddr         string
	HTTPAddr          string
	GRPCAddr          string
	AdminAddr         string
	RateLimitRequests int
	RateLimitWindow   time.Duration
}

func Load() Config {
	return Config{
		DatabaseURL:       getSecretEnv("DATABASE_URL", ""),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		HTTPAddr:          getEnv("HTTP_ADDR", ":8080"),
		GRPCAddr:          getEnv("GRPC_ADDR", ":9090"),
		AdminAddr:         getEnv("ADMIN_ADDR", ":9100"),
		RateLimitRequests: getEnvInt("RATE_LIMIT_REQUESTS", 20),
		RateLimitWindow:   getEnvDuration("RATE_LIMIT_WINDOW", 10*time.Second),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getSecretEnv(key, fallback string) string {
	if path := os.Getenv(key + "_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return fallback
		}
		return strings.TrimSpace(string(data))
	}
	return getEnv(key, fallback)
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fatal(key, v, err)
	}
	return n
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		fatal(key, v, err)
	}
	return d
}

func fatal(key, value string, err error) {
	slog.Error("invalid config value", "key", key, "value", value, "error", err)
	os.Exit(1)
}
