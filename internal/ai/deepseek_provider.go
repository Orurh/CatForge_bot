package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeepSeekProvider uses Chat Completions, including compatible reseller gateways.
// It is separate from the Responses transport used by OpenAI and the Codex bridge.
type DeepSeekProvider struct{ *OpenAIProvider }

func NewDeepSeekProvider(apiKey, baseURL, model string, client *http.Client) (*DeepSeekProvider, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u == nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("DeepSeek requires an explicit HTTPS base URL without credentials or query")
	}
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	// Never forward credentials or chat content to a redirected endpoint.
	safeClient := *client
	safeClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	p, err := NewOpenAIProvider(apiKey, u.String(), model, &safeClient)
	if err != nil {
		return nil, err
	}
	return &DeepSeekProvider{p}, nil
}

func (p *DeepSeekProvider) Name() string { return "deepseek" }

func (p *DeepSeekProvider) Generate(ctx context.Context, prompt Prompt) (ProviderResult, error) {
	payload, err := json.Marshal(map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": prompt.System},
			{"role": "user", "content": prompt.User},
		},
		"max_tokens": 512,
		"stream":     false,
		"thinking":   map[string]string{"type": "disabled"},
	})
	if err != nil {
		return ProviderResult{}, fmt.Errorf("marshal DeepSeek request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return ProviderResult{}, fmt.Errorf("create DeepSeek request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return ProviderResult{}, fmt.Errorf("DeepSeek request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		// Do not log reseller error bodies: they can echo keys or chat content.
		return ProviderResult{}, fmt.Errorf("DeepSeek response status %d", resp.StatusCode)
	}
	var envelope struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&envelope); err != nil {
		return ProviderResult{}, fmt.Errorf("decode DeepSeek response: %w", err)
	}
	result := ProviderResult{Model: envelope.Model, TokensIn: envelope.Usage.PromptTokens, TokensOut: envelope.Usage.CompletionTokens}
	if result.Model == "" {
		result.Model = p.model
	}
	if len(envelope.Choices) == 0 {
		return result, fmt.Errorf("DeepSeek returned no choices")
	}
	choice := envelope.Choices[0]
	if choice.FinishReason != "stop" {
		result.Blocked = choice.FinishReason == "content_filter"
		return result, fmt.Errorf("DeepSeek returned an incomplete or blocked answer")
	}
	result.Text = strings.TrimSpace(choice.Message.Content)
	if result.Text == "" {
		return result, fmt.Errorf("DeepSeek returned an empty answer")
	}
	return result, nil
}
