package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDirectReplyOutageUsesProceduralFallbackAndIsNotRemembered(t *testing.T) {
	for _, kind := range []GenerationType{GenerationCatReply, GenerationHumanReplyToCat} {
		for _, primary := range []Provider{
			nil,
			&stubProvider{name: "offline", err: errors.New("offline")},
			&stubProvider{name: "timeout", err: context.DeadlineExceeded},
			&stubProvider{name: "invalid", result: ProviderResult{Text: "one\ntwo"}},
			&stubProvider{name: "blocked", result: ProviderResult{Blocked: true}},
		} {
			memory := &fakeSpeechMemory{byScope: make(map[string][]SpeechMemory)}
			g := NewGateway(primary, NewFallbackProvider(), nil, time.Second)
			g.SetConversationMemory(memory)
			got, err := g.Generate(context.Background(), GenerationRequest{
				Type: kind, ChatID: 1, Cat: CatContext{ID: 7},
				UserMessage: "стоит ли не делать этого?", HumorMode: HumorBold,
			})
			if err != nil || !got.Fallback || got.Provider != "procedural" || got.Model != "procedural-v1" || got.Text == "" || strings.Contains(got.Text, "ИИ недоступен") {
				t.Fatalf("outage reply = %+v, %v", got, err)
			}
			if len(memory.byScope) != 0 {
				t.Fatal("outage notice polluted conversation memory")
			}
		}
	}
}

func TestQuestionPromptKeepsMeaningBeforeJoke(t *testing.T) {
	prompt := BuildPrompt(GenerationRequest{Type: GenerationCatReply, UserMessage: "стоит ли не делать этого?"})
	for _, fragment := range []string{"Сначала ответь на конкретный смысл", "двойные отрицания", "Шутка не должна заменять ответ"} {
		if !strings.Contains(prompt.System, fragment) {
			t.Fatalf("missing reply rule %q", fragment)
		}
	}
}
