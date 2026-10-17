package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

type Config struct {
    CatPhotoReviewEnabled bool
    CatPhotoReviewerID int64
	BanterInterval                   time.Duration
	BanterTimezone                   *time.Location
	BanterQuietStart, BanterQuietEnd int
	Env                              string
	TelegramMode                     string
	HTTPAddr                         string
	PublicBaseURL                    string
	TelegramToken                    string
	WebhookSecret                    string
	WebhookPath                      string
	PaymentSupportContact            string
	BotUsername                      string
	DatabaseURL                      string
	MigrationsDir                    string
	GameEngineAddr                   string
	AIProvider                       string
	AIAPIKey                         string
	AIBaseURL                        string
	AIModel                          string
	AITimeout                        time.Duration
	CodexBridgeSocket                string
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
	var err error
    c.CatPhotoReviewEnabled = strings.EqualFold(getenv("CAT_PHOTO_REVIEW_ENABLED","false"),"true")
    if c.CatPhotoReviewEnabled { c.CatPhotoReviewerID,err=strconv.ParseInt(getenv("CAT_PHOTO_REVIEWER_TELEGRAM_ID",""),10,64);if err!=nil||c.CatPhotoReviewerID<=0{return Config{},fmt.Errorf("CAT_PHOTO_REVIEWER_TELEGRAM_ID must be positive when cat photos are enabled")} }
	c.BanterInterval, err = time.ParseDuration(getenv("CAT_BANTER_INTERVAL", "2h"))
	if err != nil || c.BanterInterval < time.Minute {
		return Config{}, fmt.Errorf("CAT_BANTER_INTERVAL must be at least 1m")
	}
	c.BanterTimezone, err = time.LoadLocation(getenv("CAT_BANTER_TIMEZONE", "Europe/Moscow"))
	if err != nil {
		return Config{}, fmt.Errorf("CAT_BANTER_TIMEZONE: %w", err)
	}
	c.BanterQuietStart, err = strconv.Atoi(getenv("CAT_BANTER_QUIET_START", "0"))
	if err != nil || c.BanterQuietStart < 0 || c.BanterQuietStart > 23 {
		return Config{}, fmt.Errorf("CAT_BANTER_QUIET_START must be 0..23")
	}
	c.BanterQuietEnd, err = strconv.Atoi(getenv("CAT_BANTER_QUIET_END", "10"))
	if err != nil || c.BanterQuietEnd < 0 || c.BanterQuietEnd > 23 {
		return Config{}, fmt.Errorf("CAT_BANTER_QUIET_END must be 0..23")
	}
	aiTimeout, err := time.ParseDuration(strings.TrimSpace(getenv("AI_TIMEOUT", "5s")))
	if err != nil || aiTimeout <= 0 || aiTimeout > 90*time.Second {
		return Config{}, fmt.Errorf("AI_TIMEOUT must be a duration between 1ns and 90s")
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
	if c.AIProvider != "procedural" && c.AIProvider != "openai" && c.AIProvider != "codex" && c.AIProvider != "deepseek" {
		return Config{}, fmt.Errorf("AI_PROVIDER must be procedural, openai, codex or deepseek")
	}
	if c.AIProvider == "codex" && c.AIModel == "" {
		return Config{}, fmt.Errorf("AI_MODEL is required when AI_PROVIDER=codex")
	}
	if c.AIProvider == "openai" || c.AIProvider == "deepseek" {
		if c.AIAPIKey == "" {
			return Config{}, fmt.Errorf("AI_API_KEY is required when AI_PROVIDER=%s", c.AIProvider)
		}
		if c.AIModel == "" {
			return Config{}, fmt.Errorf("AI_MODEL is required when AI_PROVIDER=%s", c.AIProvider)
		}
	}
	if c.AIProvider == "deepseek" && strings.TrimSpace(os.Getenv("AI_BASE_URL")) == "" {
		return Config{}, fmt.Errorf("AI_BASE_URL is required when AI_PROVIDER=deepseek")
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
