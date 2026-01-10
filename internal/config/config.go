package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env           string
	HTTPAddr      string
	PublicBaseURL string
	TelegramToken string
	WebhookPath   string
	BotUsername   string
	DatabaseURL   string
	MigrationsDir string
}

func Load() (Config, error) {
	c := Config{
		Env:           getenv("APP_ENV", "prod"),
		HTTPAddr:      getenv("HTTP_ADDR", "0.0.0.0:8080"),
		PublicBaseURL: strings.TrimRight(getenv("PUBLIC_BASE_URL", ""), "/"),
		TelegramToken: getenv("TELEGRAM_TOKEN", ""),
		WebhookPath:   getenv("WEBHOOK_PATH", "/tg/webhook"),
		BotUsername:   strings.TrimPrefix(strings.TrimSpace(getenv("BOT_USERNAME", "")), "@"),
		DatabaseURL:   getenv("DATABASE_URL", ""),
		MigrationsDir: getenv("MIGRATIONS_DIR", "./migrations"),
	}
	if c.Env != "dev" && c.PublicBaseURL == "" {
		return Config{}, fmt.Errorf("PUBLIC_BASE_URL is required")
	}
	if c.TelegramToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_TOKEN is required")
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if !strings.HasPrefix(c.WebhookPath, "/") {
		return Config{}, fmt.Errorf("WEBHOOK_PATH must start with '/'")
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
