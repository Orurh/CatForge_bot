package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDeepSeekChatCompletions(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.URL.String() != "https://example.test/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatal("wrong endpoint or authentication")
		}
		var body struct {
			Model     string                           `json:"model"`
			Messages  []struct{ Role, Content string } `json:"messages"`
			MaxTokens int                              `json:"max_tokens"`
			Thinking  struct{ Type string }            `json:"thinking"`
			Stream    bool                             `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "deepseek-v4-flash" || body.MaxTokens != 512 || body.Thinking.Type != "disabled" || body.Stream || len(body.Messages) != 2 {
			t.Fatalf("wrong request: %+v", body)
		}
		if body.Messages[0].Role != "system" || body.Messages[0].Content != "rules" || body.Messages[1].Role != "user" || body.Messages[1].Content != "question" {
			t.Fatal("wrong messages")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"model":"deepseek-v4-flash-0731","choices":[{"finish_reason":"stop","message":{"content":"мяу","reasoning_content":"do not show this"}}],"usage":{"prompt_tokens":123,"completion_tokens":7}}`))}, nil
	})}
	p, err := NewDeepSeekProvider("secret", "https://example.test/v1/", "deepseek-v4-flash", client)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Generate(context.Background(), Prompt{System: "rules", User: "question"})
	if err != nil || got.Text != "мяу" || got.Model != "deepseek-v4-flash-0731" || got.TokensIn != 123 || got.TokensOut != 7 || p.Name() != "deepseek" {
		t.Fatalf("%+v, %v", got, err)
	}
	if !errors.Is(p.client.CheckRedirect(nil, nil), http.ErrUseLastResponse) {
		t.Fatal("redirects must be refused")
	}
}

func TestDeepSeekRejectsBadResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"auth", `{"error":{"message":"secret echoed key"}}`, 401},
		{"limit", `secret`, 429},
		{"redirect", `secret`, 302},
		{"json", `broken`, 200},
		{"empty", `{"choices":[]}`, 200},
		{"reasoning_only", `{"choices":[{"finish_reason":"stop","message":{"reasoning_content":"secret"}}]}`, 200},
		{"truncated", `{"choices":[{"finish_reason":"length","message":{"content":"half answer"}}],"usage":{"prompt_tokens":12,"completion_tokens":180}}`, 200},
		{"blocked", `{"choices":[{"finish_reason":"content_filter","message":{"content":"blocked"}}]}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := NewDeepSeekProvider("secret", "https://example.test/v1", "deepseek-v4-flash", &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})})
			got, err := p.Generate(context.Background(), Prompt{})
			if err == nil || strings.Contains(err.Error(), "secret") || got.Text != "" {
				t.Fatalf("%+v, %v", got, err)
			}
			if tc.name == "truncated" && (got.TokensIn != 12 || got.TokensOut != 180) {
				t.Fatal("failed attempt usage lost")
			}
			if tc.name == "blocked" && !got.Blocked {
				t.Fatal("missing block flag")
			}
		})
	}
}

func TestDeepSeekConfigurationAndCancellation(t *testing.T) {
	for _, base := range []string{"", "http://example.test", "https://user:secret@example.test", "https://example.test?key=secret"} {
		if _, err := NewDeepSeekProvider("secret", base, "flash", nil); err == nil {
			t.Fatalf("accepted %q", base)
		}
	}
	for _, args := range [][2]string{{"", "flash"}, {"secret", ""}} {
		if _, err := NewDeepSeekProvider(args[0], "https://example.test/v1", args[1], nil); err == nil {
			t.Fatal("accepted missing credentials/model")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p, _ := NewDeepSeekProvider("secret", "https://example.test/v1", "flash", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })})
	if _, err := p.Generate(ctx, Prompt{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
}
