package telegram

import (
	"strings"
	"testing"

	"catforge/internal/app"
	"catforge/internal/domain"
)

func TestFormatFightOutcomeUsesExternalQueueText(t *testing.T) {
	t.Parallel()
	cat := &domain.Cat{ID: 10, Name: "Барсик"}
	waiting := formatFightOutcome(app.FightOutcome{Status: domain.FightQueueWaiting, Cat: cat})
	canceled := formatFightOutcome(app.FightOutcome{Status: domain.FightQueueCanceled, Cat: cat})
	if !strings.Contains(waiting, "Барсик") || !strings.Contains(waiting, "/fight") {
		t.Fatalf("unexpected waiting text: %q", waiting)
	}
	if !strings.Contains(canceled, "Барсик") {
		t.Fatalf("unexpected canceled text: %q", canceled)
	}
}

func TestFormatFinishedFightIncludesWinnerAndRivalry(t *testing.T) {
	t.Parallel()
	waiter := &domain.Cat{ID: 10, Name: "Барсик", Level: 4}
	newcomer := &domain.Cat{ID: 20, Name: "Батон", Level: 2}
	text := formatFightOutcome(app.FightOutcome{
		Status: domain.FightQueueMatched, Cat: newcomer, Opponent: waiter, Seed: 42, Rivalry: 7,
		Result: domain.FightResult{WinnerCatID: 20, LoserCatID: 10, Rounds: 3, FinalHPA: 0, FinalHPB: 4},
	})
	for _, fragment := range []string{"Барсик", "Батон", "4 HP", "Раундов: 3", "Соперничество: 7"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("fight text %q does not contain %q", text, fragment)
		}
	}
}
