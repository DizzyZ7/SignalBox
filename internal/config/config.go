package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr                   string
	DatabaseURL                string
	AdminAPIKey                string
	AutoMigrate                bool
	TelegramBotToken           string
	TelegramDefaultChatID      string
	MaxBodyBytes               int64
	WebhookRateLimitRequests   int
	WebhookRateLimitWindow     time.Duration
	DeliveryWorkerEnabled      bool
	DeliveryWorkerInterval     time.Duration
	DeliveryWorkerBatchSize    int
	DeliveryWorkerLockDuration time.Duration
	DeliveryMaxAttempts        int
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:                   env("HTTP_ADDR", ":8080"),
		DatabaseURL:                os.Getenv("DATABASE_URL"),
		AdminAPIKey:                os.Getenv("ADMIN_API_KEY"),
		AutoMigrate:                envBool("AUTO_MIGRATE", false),
		TelegramBotToken:           os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramDefaultChatID:      os.Getenv("TELEGRAM_DEFAULT_CHAT_ID"),
		MaxBodyBytes:               envInt64("MAX_BODY_BYTES", 1<<20),
		WebhookRateLimitRequests:   envInt("WEBHOOK_RATE_LIMIT_REQUESTS", 120),
		WebhookRateLimitWindow:     envDuration("WEBHOOK_RATE_LIMIT_WINDOW", time.Minute),
		DeliveryWorkerEnabled:      envBool("DELIVERY_WORKER_ENABLED", true),
		DeliveryWorkerInterval:     envDuration("DELIVERY_WORKER_INTERVAL", 5*time.Second),
		DeliveryWorkerBatchSize:    envInt("DELIVERY_WORKER_BATCH_SIZE", 10),
		DeliveryWorkerLockDuration: envDuration("DELIVERY_WORKER_LOCK_DURATION", time.Minute),
		DeliveryMaxAttempts:        envInt("DELIVERY_MAX_ATTEMPTS", 8),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if len(cfg.AdminAPIKey) < 16 {
		return Config{}, errors.New("ADMIN_API_KEY must be at least 16 characters")
	}
	if cfg.MaxBodyBytes < 1024 || cfg.MaxBodyBytes > 10<<20 {
		return Config{}, errors.New("MAX_BODY_BYTES must be between 1024 and 10485760")
	}
	if cfg.WebhookRateLimitRequests < 0 {
		return Config{}, errors.New("WEBHOOK_RATE_LIMIT_REQUESTS must not be negative")
	}
	if cfg.WebhookRateLimitRequests > 0 && cfg.WebhookRateLimitWindow <= 0 {
		return Config{}, errors.New("WEBHOOK_RATE_LIMIT_WINDOW must be positive")
	}
	if cfg.DeliveryWorkerInterval <= 0 {
		return Config{}, errors.New("DELIVERY_WORKER_INTERVAL must be positive")
	}
	if cfg.DeliveryWorkerBatchSize <= 0 || cfg.DeliveryWorkerBatchSize > 100 {
		return Config{}, errors.New("DELIVERY_WORKER_BATCH_SIZE must be between 1 and 100")
	}
	if cfg.DeliveryWorkerLockDuration <= 0 {
		return Config{}, errors.New("DELIVERY_WORKER_LOCK_DURATION must be positive")
	}
	if cfg.DeliveryMaxAttempts <= 0 || cfg.DeliveryMaxAttempts > 100 {
		return Config{}, errors.New("DELIVERY_MAX_ATTEMPTS must be between 1 and 100")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
