package ai

import (
	"context"
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
			t.Errorf("censor(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestGatewayCensorsBothProviders(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		primary := &stubProvider{name: "primary", result: ProviderResult{Text: "Блядь, опять."}}
		backup := &stubProvider{name: "backup", result: ProviderResult{Text: "Блядь, опять."}}
		if fallback {
			primary.result.Blocked = true
		}
		got, err := NewGateway(primary, backup, nil, time.Second).GenerateCatReply(context.Background(), GenerationRequest{HumorMode: HumorBold})
		if err != nil || got.Text != "Б***, опять." || got.Fallback != fallback {
			t.Fatalf("fallback=%v: %+v, %v", fallback, got, err)
		}
	}
}
