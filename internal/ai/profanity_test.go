package ai

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCensorProfanity(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"Блядь, охуеть! Заебали, нахуй, пиздец.", "Б***, о***! З***, н***, п***."},
		{"бля сука ёбаный хуйню", "б*** с*** ё*** х***"},
		{"Страхуй корабля, употреблять хлеб, учебник, рубля, мебель.", "Страхуй корабля, употреблять хлеб, учебник, рубля, мебель."},
		{"Бл***, на х***!\nП***ец.", "Бл***, на х***!\nП***ец."},
	} {
		if got := censorProfanity(tc.input); got != tc.want {
			t.Errorf("censor(%q)=%q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestGatewayCensorsBothProviders(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		primary := &stubProvider{name: "primary", result: ProviderResult{Text: "Блядь, опять."}}
		backup := &stubProvider{name: "backup", result: ProviderResult{Text: "Блядь, опять."}}
		primary.result.Blocked = fallback
		got, err := NewGateway(primary, backup, nil, time.Second).GenerateCatReply(context.Background(), GenerationRequest{HumorMode: HumorBold})
		if err != nil || got.Text != "Б***, опять." || got.Fallback != fallback {
			t.Fatalf("%+v %v", got, err)
		}
	}
}

func TestEveryPromptDefinesHumorMode(t *testing.T) {
	for _, kind := range []GenerationType{GenerationCatReply, GenerationHumanReplyToCat, GenerationArenaBanter, GenerationYardBanter, GenerationTrainingNarrative, GenerationEventNarrative, GenerationWeeklySummary, GenerationAutonomousCat, GenerationFirstPersonalityLine} {
		bold := BuildPrompt(GenerationRequest{Type: kind, HumorMode: HumorBold})
		normal := BuildPrompt(GenerationRequest{Type: kind, HumorMode: HumorNormal})
		if !strings.Contains(bold.System, "Разрешён запиканный мат") || !strings.Contains(bold.System, "Никогда не пиши мат целиком") {
			t.Errorf("bold instructions absent for %s", kind)
		}
		if strings.Contains(normal.System, "Разрешён запиканный мат") || !strings.Contains(normal.System, "Normal — ирония без мата") {
			t.Errorf("normal instructions absent for %s", kind)
		}
	}
}
