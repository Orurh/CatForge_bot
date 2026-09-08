package narrative

import (
	"strings"
	"testing"

	"catforge/internal/domain"
)

func TestHuntStoryIsDeterministicAndTraitSpecific(t *testing.T) {
	t.Parallel()
	lazy := HuntStory(domain.EncounterMicePack, "lazy", 123)
	if lazy != HuntStory(domain.EncounterMicePack, "lazy", 123) {
		t.Fatal("same hunt input produced different stories")
	}
	if philosopher := HuntStory(domain.EncounterMicePack, "philosopher", 123); lazy == philosopher {
		t.Fatalf("trait did not affect story: %q", lazy)
	}
	if !strings.Contains(lazy, "отдых") && !strings.Contains(lazy, "полежал") {
		t.Fatalf("lazy reaction is missing: %q", lazy)
	}
}

func TestHuntStoryHasRareAbsurdVariant(t *testing.T) {
	t.Parallel()
	story := HuntStory(domain.EncounterPigeon, "sleepy", 0)
	if !strings.Contains(story, "шаурм") && !strings.Contains(story, "психологически") && !strings.Contains(story, "сосиск") {
		t.Fatalf("absurd suffix is missing: %q", story)
	}
}

func TestExpeditionStoryRecognizesCloseVictoryAndLoot(t *testing.T) {
	t.Parallel()
	result := domain.ExpeditionResult{
		Outcome: domain.ExpeditionVictory, Location: domain.ExpeditionPark, Difficulty: domain.ExpeditionHard,
		Enemy: domain.Enemy{Kind: domain.EnemyWildLynx}, CatHPAfter: 9, Rounds: 4,
		Turns: []domain.BattleTurn{{Actor: domain.BattleActorEnemy, Damage: 91, DefenderHPAfter: 9}},
		Loot:  domain.LootRoll{Dropped: true, ItemID: "lynx_claw"},
	}
	story := ExpeditionStory(result, "neat")
	if story != ExpeditionStory(result, "neat") {
		t.Fatal("same expedition result produced different stories")
	}
	if !strings.Contains(story, "HP") && !strings.Contains(story, "лапах") && !strings.Contains(story, "один удар") {
		t.Fatalf("close-victory text is missing: %q", story)
	}
	if !strings.Contains(story, "трофей") && !strings.Contains(story, "наград") && !strings.Contains(story, "пустыми лапами") {
		t.Fatalf("loot suffix is missing: %q", story)
	}
}
