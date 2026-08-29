package telegram

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestTelegramRequestErrorDoesNotExposeTokenURL(t *testing.T) {
	t.Parallel()
	err := telegramRequestError("setWebhook", &url.Error{
		Op:  "Post",
		URL: "https://api.telegram.org/botSECRET_TOKEN/setWebhook",
		Err: errors.New("network unreachable"),
	})
	if strings.Contains(err.Error(), "SECRET_TOKEN") || strings.Contains(err.Error(), "/bot") {
		t.Fatalf("sanitized error leaked request URL: %v", err)
	}
	if !strings.Contains(err.Error(), "network unreachable") {
		t.Fatalf("sanitized error lost useful cause: %v", err)
	}
}
