package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr              string
	DatabaseURL           string
	AdminAPIKey           string
	AutoMigrate           bool
	TelegramBotToken      string
	TelegramDefaultChatID string
	MaxBodyBytes          int64
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:              env("HTTP_ADDR", ":8080"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		AdminAPIKey:           os.Getenv("ADMIN_API_KEY"),
		AutoMigrate:           envBool("AUTO_MIGRATE", false),
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramDefaultChatID: os.Getenv("TELEGRAM_DEFAULT_CHAT_ID"),
		MaxBodyBytes:          envInt64("MAX_BODY_BYTES", 1<<20),
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
