package views

import (
	"strings"
	"testing"

	"catforge/internal/domain"
)

func TestFormatBestiaryShowsProgressAndHidesUnknownNames(t *testing.T) {
	t.Parallel()
	text := FormatBestiary([]domain.BestiaryEntry{
		{EnemyKind: domain.EnemySewerRat, Encounters: 3, Victories: 2},
		{EnemyKind: domain.EnemyMoonLynx, Encounters: 1, Victories: 1},
	})
	for _, want := range []string{"открыто 2/6", "Канализационная крыса", "встреч 3", "Лунная рысь", "Неизвестный противник"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text %q does not contain %q", text, want)
		}
	}
	if strings.Contains(text, "Крысиный Бухгалтер") {
		t.Fatalf("undiscovered rare enemy leaked into bestiary: %q", text)
	}
}
