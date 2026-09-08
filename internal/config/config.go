package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Env                   string
	TelegramMode          string
	HTTPAddr              string
	PublicBaseURL         string
	TelegramToken         string
	WebhookSecret         string
	WebhookPath           string
	PaymentSupportContact string
	BotUsername           string
	DatabaseURL           string
	MigrationsDir         string
	GameEngineAddr        string
	AIProvider            string
	AIAPIKey              string
	AIBaseURL             string
	AIModel               string
	AITimeout             time.Duration
	CodexBridgeSocket     string
}

func Load() (Config, error) {
	c := Config{
		PaymentSupportContact: strings.TrimSpace(getenv("PAYMENT_SUPPORT_CONTACT", "")),
		Env:                   getenv("APP_ENV", "prod"),
		TelegramMode:          strings.ToLower(strings.TrimSpace(getenv("TELEGRAM_MODE", "webhook"))),
		HTTPAddr:              getenv("HTTP_ADDR", "0.0.0.0:8080"),
		PublicBaseURL:         strings.TrimRight(getenv("PUBLIC_BASE_URL", ""), "/"),
		TelegramToken:         getenv("TELEGRAM_TOKEN", ""),
		WebhookSecret:         strings.TrimSpace(getenv("TELEGRAM_WEBHOOK_SECRET", "")),
		WebhookPath:           getenv("WEBHOOK_PATH", "/tg/webhook"),
		BotUsername:           strings.TrimPrefix(strings.TrimSpace(getenv("BOT_USERNAME", "")), "@"),
		DatabaseURL:           getenv("DATABASE_URL", ""),
		MigrationsDir:         getenv("MIGRATIONS_DIR", "./migrations"),
		GameEngineAddr:        strings.TrimSpace(getenv("GAME_ENGINE_ADDR", "")),
		AIProvider:            strings.ToLower(strings.TrimSpace(getenv("AI_PROVIDER", "procedural"))),
		AIAPIKey:              strings.TrimSpace(getenv("AI_API_KEY", "")),
		AIBaseURL:             strings.TrimRight(strings.TrimSpace(getenv("AI_BASE_URL", "https://api.openai.com/v1")), "/"),
		AIModel:               strings.TrimSpace(getenv("AI_MODEL", "")),
		CodexBridgeSocket:     strings.TrimSpace(getenv("CODEX_BRIDGE_SOCKET", "/run/codex-bridge/bridge.sock")),
	}
	aiTimeout, err := time.ParseDuration(strings.TrimSpace(getenv("AI_TIMEOUT", "5s")))
	if err != nil || aiTimeout <= 0 || aiTimeout > 30*time.Second {
		return Config{}, fmt.Errorf("AI_TIMEOUT must be a duration between 1ns and 30s")
	}
	c.AITimeout = aiTimeout
	if c.TelegramMode != "webhook" && c.TelegramMode != "polling" {
		return Config{}, fmt.Errorf("TELEGRAM_MODE must be webhook or polling")
	}
	if c.TelegramMode == "webhook" && c.Env != "dev" && c.PublicBaseURL == "" {
		return Config{}, fmt.Errorf("PUBLIC_BASE_URL is required")
	}
	if c.TelegramToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_TOKEN is required")
	}
	if c.TelegramMode == "webhook" && c.Env != "dev" && c.WebhookSecret == "" {
		return Config{}, fmt.Errorf("TELEGRAM_WEBHOOK_SECRET is required outside dev")
	}
	if len(c.WebhookSecret) > 256 {
		return Config{}, fmt.Errorf("TELEGRAM_WEBHOOK_SECRET must not exceed 256 characters")
	}
	for _, ch := range c.WebhookSecret {
		if !(ch >= 'A' && ch <= 'Z') && !(ch >= 'a' && ch <= 'z') && !(ch >= '0' && ch <= '9') && ch != '_' && ch != '-' {
			return Config{}, fmt.Errorf("TELEGRAM_WEBHOOK_SECRET contains an unsupported character")
		}
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if c.GameEngineAddr == "" {
		return Config{}, fmt.Errorf("GAME_ENGINE_ADDR is required; the C++ engine is the only game engine")
	}
	if c.AIProvider != "procedural" && c.AIProvider != "openai" && c.AIProvider != "codex" {
		return Config{}, fmt.Errorf("AI_PROVIDER must be procedural, openai or codex")
	}
	if c.AIProvider == "codex" && c.AIModel == "" {
		return Config{}, fmt.Errorf("AI_MODEL is required when AI_PROVIDER=codex")
	}
	if c.AIProvider == "openai" {
		if c.AIAPIKey == "" {
			return Config{}, fmt.Errorf("AI_API_KEY is required when AI_PROVIDER=openai")
		}
		if c.AIModel == "" {
			return Config{}, fmt.Errorf("AI_MODEL is required when AI_PROVIDER=openai")
		}
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
