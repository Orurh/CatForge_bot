package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"catforge/internal/domain"
	"catforge/internal/gamecontent"
)

func TestAllTraitsHaveDistinctFoodRepliesAndSpeechStyles(t *testing.T) {
	seen := map[string]bool{}
	for _, trait := range domain.AllTraits {
		for _, mode := range []HumorMode{HumorNormal, HumorBold} {
			got, err := NewGateway(nil, NewFallbackProvider(), nil, time.Second).Generate(context.Background(), GenerationRequest{
				Type: GenerationCatReply, Cat: CatContext{Name: "Кот", Trait: trait},
				HumorMode: mode, UserMessage: "Я тебе корм купил",
			})
			if err != nil || !got.Fallback || got.Text != gamecontent.Render("fallback.food."+string(trait), 0, nil) {
				t.Fatalf("trait=%s mode=%s: %+v %v", trait, mode, got, err)
			}
			if mode == HumorNormal {
				if seen[got.Text] {
					t.Fatalf("traits share food reply: %q", got.Text)
				}
				seen[got.Text] = true
			}
		}
		prompt := BuildPrompt(GenerationRequest{Type: GenerationCatReply, Cat: CatContext{Trait: trait}})
		if prompt.Request.Cat.SpeechStyle != domain.DefaultSpeechStyle(trait) || !strings.Contains(prompt.System, "Не смешивай борзого, наглого и злого") {
			t.Fatalf("missing trait guidance: %s", trait)
		}
	}
	if len(seen) != 6 {
		t.Fatalf("want six traits, got %d", len(seen))
	}
	for _, retired := range []domain.Trait{"philosopher", "neat", "sleepy"} {
		if domain.IsValidTrait(retired) {
			t.Fatalf("retired trait accepted: %s", retired)
		}
	}
}

func TestNewTraitContentCoversSpeechTrainingAndYardEvents(t *testing.T) {
	for _, trait := range domain.AllTraits {
		for _, prefix := range []string{
			"fallback.first_line.", "fallback.reply.normal.", "fallback.reply.bold.",
			"fallback.followup.normal.", "fallback.followup.bold.",
			"fallback.bedtime.normal.", "fallback.bedtime.bold.",
			"fallback.relationship_reply.rivalry.", "fallback.relationship_reply.friendship.", "fallback.relationship_reply.respect.",
			"yard_event.fish_truck.autonomous.success.", "yard_event.big_dog.autonomous.success.", "yard_event.big_box.autonomous.success.",
		} {
			key := prefix + string(trait)
			if !gamecontent.Has(key) || gamecontent.Render(key, 0, map[string]string{"OtherCatName": "Друг"}) == "" {
				t.Errorf("missing content: %s", key)
			}
		}
		if !gamecontent.Has("trait." + string(trait) + ".reaction") {
			t.Errorf("missing training trait %s", trait)
		}
	}
}
