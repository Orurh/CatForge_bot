package telegram

import (
	"strings"
	"testing"
	"unicode/utf8"

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

func TestFormatFightBanterUsesOneCompactTwoCatMessage(t *testing.T) {
	t.Parallel()
	text := formatFightBanter(&app.FightBanter{
		FirstCat:  &domain.Cat{Name: "Барсик", Breed: domain.BreedBengal, Level: 3},
		SecondCat: &domain.Cat{Name: "Батон", Breed: domain.BreedBritish, Level: 4},
		FirstLine: "Это была разведка.", SecondLine: "Тогда лежи и изучай местность.",
	})
	for _, want := range []string{"Барсик: Это была разведка.", "Батон: Тогда лежи и изучай местность.", "\n\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("fight banter %q misses %q", text, want)
		}
	}
}

func TestFightResultDoesNotOfferImpossibleRevenge(t *testing.T) {
	t.Parallel()
	catA := &domain.Cat{ID: 10, Name: "Барсик", Level: 2}
	catB := &domain.Cat{ID: 20, Name: "Батон", Level: 2}
	text := formatFinishedFight(app.FightOutcome{
		Status: domain.FightQueueMatched, Cat: catB, Opponent: catA, Seed: 1,
		Result: domain.FightResult{
			WinnerCatID: catA.ID, LoserCatID: catB.ID, Rounds: 1, FinalHPA: 5,
			Turns: []domain.FightTurn{{Round: 1, AttackerCatID: catA.ID, DefenderCatID: catB.ID, Damage: 5}},
		},
	})
	if strings.Contains(strings.ToLower(text), "реванш") {
		t.Fatalf("fight result still offers revenge: %q", text)
	}
	if !strings.Contains(contentText("fight.limit.daily"), "одна драка") {
		t.Fatalf("daily limit text is stale: %q", contentText("fight.limit.daily"))
	}
}

func TestFormatFinishedFightIncludesWinnerAndRivalry(t *testing.T) {
	t.Parallel()
	waiter := &domain.Cat{ID: 10, Name: "Барсик", Level: 4}
	newcomer := &domain.Cat{ID: 20, Name: "Батон", Level: 2}
	text := formatFightOutcome(app.FightOutcome{
		Status: domain.FightQueueMatched, Cat: newcomer, Opponent: waiter, Seed: 42, Rivalry: 7,
		Stats: domain.FightStats{CatAWins: 4, CatALosses: 2, CatBWins: 3, CatBLosses: 1, PairCatAWins: 1, PairCatBWins: 2},
		Result: domain.FightResult{
			WinnerCatID: 20, LoserCatID: 10, Rounds: 3, FinalHPA: 0, FinalHPB: 4,
			Turns: []domain.FightTurn{
				{Round: 1, AttackerCatID: 20, DefenderCatID: 10, Damage: 5, DefenderHPAfter: 15},
				{Round: 1, AttackerCatID: 10, DefenderCatID: 20, Damage: 2, DefenderHPAfter: 8},
				{Round: 3, AttackerCatID: 20, DefenderCatID: 10, Damage: 15, Crit: true, DefenderHPAfter: 0},
			},
		},
	})
	for _, fragment := range []string{
		"Барсик", "Батон", "4 ед. сил", "−5 ед. сил", "КРИТ",
		"📊 ИТОГ БОЯ\n├ Раунды: 3",
		"├ Барсик\n│  ├ Победы: 4\n│  └ Поражения: 2",
		"😾 ЛИЧНЫЕ ВСТРЕЧИ\n├ Барсик: 1\n├ Батон: 2",
		"└ Соперничество: 7",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("fight text %q does not contain %q", text, fragment)
		}
	}
}

func TestSelectFightTurnIndicesKeepsOpeningCritsAndFinish(t *testing.T) {
	t.Parallel()
	turns := make([]domain.FightTurn, 20)
	for index := range turns {
		turns[index] = domain.FightTurn{Round: index/2 + 1}
	}
	turns[11].Crit = true
	indices := selectFightTurnIndices(turns, 8)
	if len(indices) != 8 || indices[0] != 0 || indices[1] != 1 || indices[len(indices)-1] != 19 {
		t.Fatalf("selected indices = %v", indices)
	}
	foundCrit := false
	for _, index := range indices {
		foundCrit = foundCrit || index == 11
	}
	if !foundCrit {
		t.Fatalf("selected indices lost the critical turn: %v", indices)
	}
}

func TestLongFightNarrativeFitsTelegramMessage(t *testing.T) {
	t.Parallel()
	catA := &domain.Cat{ID: 10, Name: strings.Repeat("А", 24), Level: 8, SPDBase: 10}
	catB := &domain.Cat{ID: 20, Name: strings.Repeat("Б", 24), Level: 8, SPDBase: 10}
	turns := make([]domain.FightTurn, 20)
	for index := range turns {
		attackerID, defenderID := catA.ID, catB.ID
		if index%2 == 1 {
			attackerID, defenderID = catB.ID, catA.ID
		}
		turns[index] = domain.FightTurn{
			Round: index/2 + 1, AttackerCatID: attackerID, DefenderCatID: defenderID,
			Damage: 4, DefenderHPAfter: 50 - index*2,
		}
	}
	turns[11].Crit = true
	turns[len(turns)-1].Crit = true
	turns[len(turns)-1].DefenderHPAfter = 0
	text := formatFinishedFight(app.FightOutcome{
		Status: domain.FightQueueMatched, Cat: catB, Opponent: catA, Seed: 99, Rivalry: 12,
		Result: domain.FightResult{
			WinnerCatID: catB.ID, LoserCatID: catA.ID, Rounds: 10,
			FinalHPA: 0, FinalHPB: 4, Turns: turns,
		},
	})
	if utf8.RuneCountInString(text) > 4096 {
		t.Fatalf("fight narrative has %d runes, Telegram limit is 4096", utf8.RuneCountInString(text))
	}
	if !strings.Contains(text, "…") || !strings.Contains(text, "ФИНАЛЬНЫЙ КРИТ") {
		t.Fatalf("long fight lost its collapsed middle or finishing turn: %q", text)
	}
}
