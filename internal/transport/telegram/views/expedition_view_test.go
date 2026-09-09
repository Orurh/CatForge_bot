package views

import (
	"strings"
	"testing"
	"time"

	"catforge/internal/domain"
)

func TestFormatExpeditionResultIncludesRewardAndCompactLog(t *testing.T) {
	t.Parallel()

	turns := make([]domain.BattleTurn, 8)
	for index := range turns {
		turns[index] = domain.BattleTurn{Round: index/2 + 1, Actor: domain.BattleActor(index % 2), Damage: index + 2, DefenderHPAfter: 30 - index}
	}
	text := FormatExpeditionResultText(&domain.Cat{Name: "Барсик", Trait: "bully"}, domain.ExpeditionResult{
		Outcome: domain.ExpeditionVictory, Location: domain.ExpeditionRooftop, Difficulty: domain.ExpeditionHard,
		Enemy: domain.Enemy{Kind: domain.EnemyStrayDog, Level: 3}, EnergyCost: 35,
		XPGain: 85, CoinsGain: 37, Rounds: 4, CatHPAfter: 12, Turns: turns,
	})
	for _, want := range []string{"Крыши", "сложно", "монеты: +37", "Ключевые ходы", "…"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text %q does not contain %q", text, want)
		}
	}
}

func TestFormatExpeditionScreenUsesRegeneratedEnergy(t *testing.T) {
	t.Parallel()
	now := time.Unix(100, 0)
	text, canExplore := FormatExpeditionScreen(&domain.Cat{
		Name: "Мур", Level: 1, Energy: 10, EnergyUpdatedAt: now.Add(-10 * domain.EnergyRegenInterval),
	}, now)
	if !canExplore || !strings.Contains(text, "Энергия: 20/100") {
		t.Fatalf("unexpected screen: canExplore=%v text=%q", canExplore, text)
	}
}

func TestFormatExpeditionResultShowsNewLoot(t *testing.T) {
	t.Parallel()
	text := FormatExpeditionResultText(&domain.Cat{Name: "Мур", Trait: "lazy"}, domain.ExpeditionResult{
		Outcome: domain.ExpeditionVictory, Location: domain.ExpeditionAlley, Difficulty: domain.ExpeditionEasy,
		Loot: domain.LootRoll{Dropped: true, Rarity: domain.ItemCommon, ItemID: "rat_tooth", New: true},
	})
	for _, want := range []string{"Новый предмет", "Крысиный зуб", "ATK +3"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text %q does not contain %q", text, want)
		}
	}
}

func TestFormatExpeditionResultHighlightsRareEnemy(t *testing.T) {
	t.Parallel()
	text := FormatExpeditionResultText(&domain.Cat{Name: "Мур"}, domain.ExpeditionResult{
		Outcome: domain.ExpeditionVictory, Location: domain.ExpeditionAlley, Difficulty: domain.ExpeditionNormal,
		Enemy: domain.Enemy{Kind: domain.EnemyRatAccountant, Level: 2}, XPGain: 50, CoinsGain: 20,
	})
	for _, want := range []string{"Редкая встреча", "Награда ×1.5", "Крысиный Бухгалтер"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text %q does not contain %q", text, want)
		}
	}
}
