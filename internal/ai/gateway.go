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

const primaryAttempts = 2

type Gateway struct {
	memory   ConversationMemory
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
	scope := speechScope(request)
	if history := g.recentSpeech(ctx, scope, request); len(history) > 0 {
		request.RecentSpeech = history
	}
	prompt := BuildPrompt(request)
	provider := g.primary
	usingFallback := provider == nil
	usedFallback := usingFallback
	if provider == nil {
		provider = g.fallback
	}

	var result ProviderResult
	var err error
	primaryBlocked := false
	if usingFallback {
		result, err = g.callFallback(ctx, prompt)
	} else {
		attemptTimeout := g.timeout / primaryAttempts
		if attemptTimeout <= 0 {
			attemptTimeout = g.timeout
		}
		for attempt := 0; attempt < primaryAttempts; attempt++ {
			result, err = g.callWithTimeout(ctx, provider, prompt, attemptTimeout)
			primaryBlocked = primaryBlocked || result.Blocked
			if err == nil && !result.Blocked && validOutputForRequest(request, result.Text) {
				break
			}
			if attempt+1 < primaryAttempts {
				g.record(ctx, request, provider, result, started, false, result.Blocked, generationError(err, result))
			}
			if result.Blocked || attempt+1 == primaryAttempts {
				break
			}
			select {
			case <-time.After(250 * time.Millisecond):
			case <-ctx.Done():
				err = ctx.Err()
			}
		}
	}
	if err != nil || result.Blocked || !validOutputForRequest(request, result.Text) {
		if usingFallback {
			g.record(ctx, request, provider, result, started, true, result.Blocked, err)
			return Generation{}, fmt.Errorf("AI fallback generation failed: %w", generationError(err, result))
		}
		// Keep the failed primary attempt: a successful fallback must not erase
		// provider outages from usage/diagnostics.
		g.record(ctx, request, provider, result, started, false, result.Blocked, generationError(err, result))
		provider = g.fallback
		usedFallback = true
		result, err = g.callFallback(ctx, prompt)
	}

	success := err == nil && !result.Blocked && validOutputForRequest(request, result.Text)
	g.record(ctx, request, provider, result, started, usedFallback, primaryBlocked || result.Blocked, err)
	if !success {
		return Generation{}, fmt.Errorf("AI generation failed: %w", generationError(err, result))
	}
	if !usedFallback || !isDirectReply(request.Type) {
		g.rememberSpeech(ctx, scope, request, strings.TrimSpace(result.Text))
	}
	return Generation{
		Text: strings.TrimSpace(result.Text), Emotion: strings.TrimSpace(result.Emotion),
		Provider: provider.Name(), Model: result.Model, Fallback: usedFallback,
	}, nil
}

func isDirectReply(kind GenerationType) bool {
	return kind == GenerationCatReply || kind == GenerationHumanReplyToCat
}

func (g *Gateway) callFallback(ctx context.Context, prompt Prompt) (ProviderResult, error) {
	return g.call(ctx, g.fallback, prompt)
}

func (g *Gateway) call(ctx context.Context, provider Provider, prompt Prompt) (ProviderResult, error) {
	return g.callWithTimeout(ctx, provider, prompt, g.timeout)
}

func (g *Gateway) callWithTimeout(ctx context.Context, provider Provider, prompt Prompt, timeout time.Duration) (ProviderResult, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
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
			if utf8.RuneCountInString(lines[index]) > 220 {
				return false
			}
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
	switch request.Type {
	case GenerationCatReply, GenerationHumanReplyToCat, GenerationFirstPersonalityLine, GenerationAutonomousCat, GenerationBedtimeFarewell:
		if strings.ContainsAny(strings.TrimSpace(value), "\r\n") || utf8.RuneCountInString(strings.TrimSpace(value)) > 300 {
			return false
		}
		if request.Type == GenerationBedtimeFarewell && request.Cat.Name != "" &&
			strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), strings.ToLower(strings.TrimSpace(request.Cat.Name))+":") {
			return false
		}
	case GenerationTrainingNarrative:
		if strings.ContainsAny(strings.TrimSpace(value), "\r\n") || utf8.RuneCountInString(strings.TrimSpace(value)) > 280 {
			return false
		}
	case GenerationWeeklySummary, GenerationEventNarrative:
		if utf8.RuneCountInString(strings.TrimSpace(value)) > 450 {
			return false
		}
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
