package config

import (
	"strings"
	"testing"
	"time"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TELEGRAM_TOKEN", "test-token")
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("PUBLIC_BASE_URL", "https://example.test")
	t.Setenv("GAME_ENGINE_ADDR", "game-engine:50051")
}

func TestLoadCodexDoesNotRequireAPIKey(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "dev")
	t.Setenv("AI_PROVIDER", "codex")
	t.Setenv("AI_API_KEY", "")
	t.Setenv("AI_MODEL", "gpt-5.6-luna")
	t.Setenv("AI_TIMEOUT", "30s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CodexBridgeSocket != "/run/codex-bridge/bridge.sock" {
		t.Fatal(cfg.CodexBridgeSocket)
	}
	t.Setenv("AI_MODEL", "")
	if _, err := Load(); err == nil {
		t.Fatal("missing Codex model accepted")
	}
}

func TestLoadRequiresWebhookSecretInProduction(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "prod")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "TELEGRAM_WEBHOOK_SECRET") {
		t.Fatalf("Load() error = %v, want webhook secret error", err)
	}
}

func TestLoadAcceptsValidWebhookSecret(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "prod")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "valid_secret-123")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.WebhookSecret != "valid_secret-123" {
		t.Fatalf("WebhookSecret = %q", cfg.WebhookSecret)
	}
}

func TestLoadRejectsUnsupportedWebhookSecretCharacters(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "prod")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "spaces are invalid")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "unsupported character") {
		t.Fatalf("Load() error = %v, want unsupported character error", err)
	}
}

func TestLoadPollingDoesNotRequirePublicURLOrWebhookSecret(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("TELEGRAM_MODE", "polling")
	t.Setenv("TELEGRAM_TOKEN", "test-token")
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("GAME_ENGINE_ADDR", "game-engine:50051")
	t.Setenv("PUBLIC_BASE_URL", "")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.TelegramMode != "polling" {
		t.Fatalf("TelegramMode = %q, want polling", cfg.TelegramMode)
	}
}

func TestLoadRejectsUnknownTelegramMode(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("TELEGRAM_MODE", "carrier-pigeon")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "TELEGRAM_MODE") {
		t.Fatalf("Load() error = %v, want mode error", err)
	}
}

func TestLoadRequiresOpenAIConfiguration(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "dev")
	t.Setenv("AI_PROVIDER", "openai")
	t.Setenv("AI_API_KEY", "")
	t.Setenv("AI_MODEL", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "AI_API_KEY") {
		t.Fatalf("Load() error = %v, want AI_API_KEY error", err)
	}
}

func TestLoadAcceptsOpenAIConfiguration(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "dev")
	t.Setenv("AI_PROVIDER", "openai")
	t.Setenv("AI_API_KEY", "test-key")
	t.Setenv("AI_MODEL", "test-model")
	t.Setenv("AI_TIMEOUT", "4s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AIModel != "test-model" || cfg.AITimeout != 4*time.Second {
		t.Fatalf("unexpected AI config: model=%q timeout=%s", cfg.AIModel, cfg.AITimeout)
	}
}
