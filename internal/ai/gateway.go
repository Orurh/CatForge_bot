package ai

import (
	"catforge/internal/observability"
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

func (g *Gateway) GenerateYardBanter(ctx context.Context, request GenerationRequest) (Generation, error) {
	request.Type = GenerationYardBanter
	return g.Generate(ctx, request)
}

func (g *Gateway) GenerateWeeklySummary(ctx context.Context, request GenerationRequest) (Generation, error) {
	request.Type = GenerationWeeklySummary
	return g.Generate(ctx, request)
}

func (g *Gateway) Generate(ctx context.Context, request GenerationRequest) (generation Generation, generationErr error) {
	observedStart := time.Now()
	defer func() {
		outcome := "primary"
		if generationErr != nil {
			outcome = "error"
		} else if g.primary == nil {
			outcome = "procedural"
		} else if generation.Fallback {
			outcome = "fallback"
		}
		observability.Operations.WithLabelValues("ai", "generate", outcome).Inc()
		observability.Duration.WithLabelValues("ai", "generate").Observe(time.Since(observedStart).Seconds())
	}()
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
	if err != nil || result.Blocked || !validOutputForRequest(request, result.Text) {
		if usingFallback {
			g.record(ctx, request, provider, result, started, true, result.Blocked, err)
			return Generation{}, fmt.Errorf("AI fallback generation failed: %w", generationError(err, result))
		}
		provider = g.fallback
		usedFallback = true
		result, err = g.call(ctx, provider, prompt)
	}

	success := err == nil && !result.Blocked && validOutputForRequest(request, result.Text)
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
	result, err := provider.Generate(callCtx, prompt)
	result.Text = censorProfanity(result.Text)
	return result, err
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
		Latency: g.now().Sub(started), Success: generationErr == nil && !result.Blocked && validOutputForRequest(request, result.Text),
		Blocked: blocked, Fallback: fallback, ErrorCode: errorCode, CreatedAt: g.now(),
	})
}

func validOutput(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && utf8.RuneCountInString(value) <= maxOutputRunes
}

func validOutputForRequest(request GenerationRequest, value string) bool {
	if !validOutput(value) {
		return false
	}
	if isBanterGeneration(request.Type) {
		lines, ok := ParseArenaBanterLines(value)
		if !ok {
			return false
		}
		for index, cat := range []CatContext{request.Cat, request.OtherCat} {
			line := strings.ToLower(strings.TrimSpace(lines[index]))
			namePrefix := strings.ToLower(strings.TrimSpace(cat.Name)) + ":"
			marker := "cat_a:"
			if index == 1 {
				marker = "cat_b:"
			}
			if strings.HasPrefix(line, namePrefix) || strings.HasPrefix(line, marker) || strings.HasPrefix(line, "- ") {
				return false
			}
		}
		return true
	}
	if request.Type == GenerationHumanReplyToCat && strings.ContainsAny(strings.TrimSpace(value), "\r\n") {
		return false
	}
	return true
}

func isBanterGeneration(kind GenerationType) bool {
	return kind == GenerationArenaBanter || kind == GenerationYardBanter
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
