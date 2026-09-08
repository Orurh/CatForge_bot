package telegram

import (
	"strings"
	"testing"

	"catforge/internal/app"
	"catforge/internal/domain"
)

func TestFormatYardSnapshotUsesVerticalStatBlocks(t *testing.T) {
	t.Parallel()
	text := formatYardSnapshot(app.YardSnapshot{
		Yard: &domain.Yard{Name: "Друзья"}, Created: true, Joined: true,
		Members: []domain.YardMember{
			{CatName: "Барсик", Breed: domain.BreedBengal, Trait: domain.TraitBully, Level: 4},
			{CatName: "Батон", Breed: domain.BreedBritish, Trait: domain.TraitLazy, Level: 3},
		},
		Relationships: []domain.CatRelationship{{
			CatAName: "Барсик", CatBName: "Батон", Friendship: 2, Rivalry: 7, Respect: 3,
		}},
	})
	for _, want := range []string{
		"🏘 ДВОР\n«Друзья»",
		"🐈 КОТЫ · 2",
		"Барсик\n└ Характер: царапыч",
		"Батон\n└ Характер: прокрастимятор",
		"Барсик ↔ Батон\n├ 🤝 Дружба: 2\n├ ⚔️ Соперничество: 7\n└ 🏅 Уважение: 3",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatYardSnapshot() = %q, missing %q", text, want)
		}
	}
}

func TestFormatYardSnapshotHandlesNilYard(t *testing.T) {
	t.Parallel()
	if got := formatYardSnapshot(app.YardSnapshot{}); got != "" {
		t.Fatalf("formatYardSnapshot() = %q, want empty", got)
	}
}
