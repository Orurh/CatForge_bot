package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const maxOutputRunes = 500

type Gateway struct {
	primary  Provider
	fallback Provider
	usage    UsageSink
	timeout  time.Duration
	now      func() time.Time
}

func NewGateway(primary Provider, fallback Provider, usage UsageSink, timeout time.Duration) *Gateway {
	if fallback == nil {
		value := NewFallbackProvider()
		fallback = value
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Gateway{primary: primary, fallback: fallback, usage: usage, timeout: timeout, now: time.Now}
}

func (g *Gateway) GenerateCatReply(ctx context.Context, request GenerationRequest) (Generation, error) {
	request.Type = GenerationCatReply
	return g.Generate(ctx, request)
}

func (g *Gateway) GenerateEventNarrative(ctx context.Context, request GenerationRequest) (Generation, error) {
	request.Type = GenerationEventNarrative
	return g.Generate(ctx, request)
}

func (g *Gateway) GenerateTrainingNarrative(ctx context.Context, request GenerationRequest) (Generation, error) {
	request.Type = GenerationTrainingNarrative
	return g.Generate(ctx, request)
}

func (g *Gateway) GenerateArenaBanter(ctx context.Context, request GenerationRequest) (Generation, error) {
	request.Type = GenerationArenaBanter
	return g.Generate(ctx, request)
}

func (g *Gateway) GenerateWeeklySummary(ctx context.Context, request GenerationRequest) (Generation, error) {
	request.Type = GenerationWeeklySummary
	return g.Generate(ctx, request)
}

func (g *Gateway) Generate(ctx context.Context, request GenerationRequest) (Generation, error) {
	started := g.now()
	prompt := BuildPrompt(request)
	provider := g.primary
	usingFallback := provider == nil
	usedFallback := usingFallback
	if provider == nil {
		provider = g.fallback
	}

	result, err := g.call(ctx, provider, prompt)
	primaryBlocked := result.Blocked
	if err != nil || result.Blocked || !validOutput(result.Text) {
		if usingFallback {
			g.record(ctx, request, provider, result, started, true, result.Blocked, err)
			return Generation{}, fmt.Errorf("AI fallback generation failed: %w", generationError(err, result))
		}
		provider = g.fallback
		usedFallback = true
		result, err = g.call(ctx, provider, prompt)
	}

	success := err == nil && !result.Blocked && validOutput(result.Text)
	g.record(ctx, request, provider, result, started, usedFallback, primaryBlocked || result.Blocked, err)
	if !success {
		return Generation{}, fmt.Errorf("AI generation failed: %w", generationError(err, result))
	}
	return Generation{
		Text: strings.TrimSpace(result.Text), Emotion: strings.TrimSpace(result.Emotion),
		Provider: provider.Name(), Model: result.Model, Fallback: usedFallback,
	}, nil
}

func (g *Gateway) call(ctx context.Context, provider Provider, prompt Prompt) (ProviderResult, error) {
	callCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()
	return provider.Generate(callCtx, prompt)
}

func (g *Gateway) record(ctx context.Context, request GenerationRequest, provider Provider, result ProviderResult, started time.Time, fallback, blocked bool, generationErr error) {
	if g.usage == nil {
		return
	}
	errorCode := ""
	if generationErr != nil {
		switch {
		case errors.Is(generationErr, context.DeadlineExceeded):
			errorCode = "timeout"
		case errors.Is(generationErr, context.Canceled):
			errorCode = "canceled"
		default:
			errorCode = "provider_error"
		}
	}
	_ = g.usage.RecordAIUsage(ctx, UsageRecord{
		GenerationType: request.Type, CatID: request.Cat.ID, YardID: request.YardID,
		Provider: provider.Name(), Model: result.Model, TokensIn: result.TokensIn, TokensOut: result.TokensOut,
		Latency: g.now().Sub(started), Success: generationErr == nil && !result.Blocked && validOutput(result.Text),
		Blocked: blocked, Fallback: fallback, ErrorCode: errorCode, CreatedAt: g.now(),
	})
}

func validOutput(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && utf8.RuneCountInString(value) <= maxOutputRunes
}

func generationError(err error, result ProviderResult) error {
	if err != nil {
		return err
	}
	if result.Blocked {
		return errors.New("generation blocked")
	}
	return errors.New("invalid generation output")
}
