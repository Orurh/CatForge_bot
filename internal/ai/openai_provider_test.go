package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestOpenAIProviderUsesResponsesAPIWithoutStorage(t *testing.T) {
	t.Parallel()
	var requestBody map[string]any
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://example.test/v1/responses" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("unexpected request: url=%q auth=%q", r.URL.String(), r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		body := `{"model":"test-model","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"мяу"}]}],"usage":{"input_tokens":10,"output_tokens":2}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	provider, err := NewOpenAIProvider("secret", "https://example.test/v1", "test-model", client)
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}
	result, err := provider.Generate(context.Background(), Prompt{System: "system", User: "user"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Text != "мяу" || result.TokensIn != 10 || result.TokensOut != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if requestBody["store"] != false || requestBody["instructions"] != "system" || requestBody["input"] != "user" {
		t.Fatalf("unexpected request body: %+v", requestBody)
	}
}

func TestOpenAIProviderDoesNotLeakProviderMessage(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"error":{"message":"secret diagnostic","type":"auth_error","code":"bad_key"}}`
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	provider, _ := NewOpenAIProvider("secret", "https://example.test/v1", "test-model", client)
	_, err := provider.Generate(context.Background(), Prompt{})
	if err == nil || err.Error() != "OpenAI response status 401: bad_key" {
		t.Fatalf("Generate() error = %v", err)
	}
}
