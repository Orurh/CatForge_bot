package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"catforge/internal/domain"
)

func TestBedtimeUsesPrimaryAndConversationMemory(t *testing.T) {
	p := &stubProvider{name: "ai", result: ProviderResult{Text: "Я спать. Ночной цирк сегодня без меня."}}
	memory := &fakeSpeechMemory{byScope: map[string][]SpeechMemory{"yard:9": {{CatID: 42, CatName: "Барсик", Text: "Опять устроили цирк."}}}}
	g := NewGateway(p, nil, nil, time.Second)
	g.SetConversationMemory(memory)
	got, err := g.Generate(context.Background(), GenerationRequest{Type: GenerationBedtimeFarewell, YardID: 9,
		Cat: CatContext{ID: 42, Name: "Барсик", Trait: domain.TraitBully}, HumorMode: HumorBold})
	if err != nil || got.Fallback || got.Text != p.result.Text {
		t.Fatalf("primary: %+v %v", got, err)
	}
	if !strings.Contains(p.prompt.System, "прощается со всем чатом") || len(p.prompt.Request.RecentSpeech) != 1 || len(memory.byScope["yard:9"]) != 2 {
		t.Fatalf("bedtime prompt or memory missing: %+v", p.prompt)
	}
}

func TestBedtimeFallbackIsAboutSleepAndHonorsHumorMode(t *testing.T) {
	for _, trait := range []domain.Trait{domain.TraitLazy, domain.TraitBully, domain.TraitNeat, domain.TraitPhilosopher, domain.TraitSleepy, ""} {
		for _, mode := range []HumorMode{HumorNormal, HumorBold} {
			p := &stubProvider{name: "unavailable", err: context.DeadlineExceeded}
			got, err := NewGateway(p, nil, nil, time.Second).Generate(context.Background(), GenerationRequest{
				Type: GenerationBedtimeFarewell, Cat: CatContext{Name: "Барсик", Trait: trait}, HumorMode: mode})
			if err != nil || !got.Fallback || !strings.Contains(got.Text, "спать") || strings.Contains(got.Text, "Барсик:") {
				t.Fatalf("trait=%s mode=%s: %+v %v", trait, mode, got, err)
			}
			if mode == HumorNormal && strings.Contains(got.Text, "***") {
				t.Fatalf("normal profanity: %q", got.Text)
			}
		}
	}
	p := &stubProvider{name: "prefixed", result: ProviderResult{Text: "Барсик: я спать."}}
	got, err := NewGateway(p, nil, nil, time.Second).Generate(context.Background(), GenerationRequest{Type: GenerationBedtimeFarewell, Cat: CatContext{Name: "Барсик"}})
	if err != nil || !got.Fallback || strings.Contains(got.Text, "Барсик:") {
		t.Fatalf("duplicate prefix: %+v %v", got, err)
	}
}
