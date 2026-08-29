package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxProviderErrorBody = 2048

type OpenAIProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewOpenAIProvider(apiKey, baseURL, model string, client *http.Client) (*OpenAIProvider, error) {
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}
	if model == "" {
		return nil, fmt.Errorf("OpenAI model is required")
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &OpenAIProvider{
		apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), model: model, client: client,
	}, nil
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Generate(ctx context.Context, prompt Prompt) (ProviderResult, error) {
	payload, err := json.Marshal(map[string]any{
		"model":             p.model,
		"instructions":      prompt.System,
		"input":             prompt.User,
		"max_output_tokens": 180,
		"store":             false,
	})
	if err != nil {
		return ProviderResult{}, fmt.Errorf("marshal OpenAI request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(payload))
	if err != nil {
		return ProviderResult{}, fmt.Errorf("create OpenAI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return ProviderResult{}, fmt.Errorf("OpenAI request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxProviderErrorBody))
		return ProviderResult{}, fmt.Errorf("OpenAI response status %d: %s", resp.StatusCode, sanitizedProviderError(body))
	}

	var envelope struct {
		Model  string `json:"model"`
		Status string `json:"status"`
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&envelope); err != nil {
		return ProviderResult{}, fmt.Errorf("decode OpenAI response: %w", err)
	}
	text := ""
	for _, output := range envelope.Output {
		if output.Type != "message" {
			continue
		}
		for _, content := range output.Content {
			if content.Type == "output_text" {
				text += content.Text
			}
		}
	}
	return ProviderResult{
		Text: text, Model: envelope.Model,
		TokensIn: envelope.Usage.InputTokens, TokensOut: envelope.Usage.OutputTokens,
	}, nil
}

func sanitizedProviderError(body []byte) string {
	var envelope struct {
		Error struct {
			Type string `json:"type"`
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		if envelope.Error.Code != "" {
			return envelope.Error.Code
		}
		if envelope.Error.Type != "" {
			return envelope.Error.Type
		}
	}
	return "provider_error"
}
