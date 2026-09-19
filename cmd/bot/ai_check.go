package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"catforge/internal/ai"
	"catforge/internal/config"
	"catforge/internal/domain"
)

// A manual probe: no Telegram calls, game changes, or database writes.
// Unlike healthcheck, this spends one real generation from the provider quota.
func runAICheck() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var provider ai.Provider
	switch cfg.AIProvider {
	case "deepseek":
		provider, err = ai.NewDeepSeekProvider(cfg.AIAPIKey, cfg.AIBaseURL, cfg.AIModel, nil)
	case "codex":
		provider, err = ai.NewCodexProvider(cfg.CodexBridgeSocket, cfg.AIModel)
	case "openai":
		provider, err = ai.NewOpenAIProvider(cfg.AIAPIKey, cfg.AIBaseURL, cfg.AIModel, nil)
	default:
		return fmt.Errorf("aicheck requires a real AI provider")
	}
	if err != nil {
		return err
	}
	question := "Я устал на работе. Стоит ли отдохнуть?"
	if len(os.Args) > 2 {
		question = os.Args[2]
	}
	start := time.Now()
	usage := &probeUsage{}
	gateway := ai.NewGateway(provider, nil, usage, cfg.AITimeout)
	reply, err := gateway.GenerateCatReply(context.Background(), ai.GenerationRequest{
		Cat:       ai.CatContext{Name: "Проверочный кот", Trait: domain.TraitNeat},
		HumorMode: ai.HumorBold, UserMessage: question,
	})
	if err != nil {
		return err
	}
	if reply.Fallback {
		return fmt.Errorf("AI unavailable: probe used fallback; inspect AI usage and bridge logs")
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Provider  string `json:"provider"`
		Model     string `json:"model"`
		Text      string `json:"text"`
		LatencyMS int64  `json:"latency_ms"`
		TokensIn  int    `json:"tokens_in"`
		TokensOut int    `json:"tokens_out"`
	}{reply.Provider, reply.Model, reply.Text, time.Since(start).Milliseconds(), usage.tokensIn, usage.tokensOut})
}

type probeUsage struct{ tokensIn, tokensOut int }

func (u *probeUsage) RecordAIUsage(_ context.Context, record ai.UsageRecord) error {
	u.tokensIn += record.TokensIn
	u.tokensOut += record.TokensOut
	return nil
}
