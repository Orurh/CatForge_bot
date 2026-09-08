package views

import (
	"strings"
	"testing"
	"time"

	"catforge/internal/domain"
)

func TestFormatCatProfileUsesShortLabeledStatRows(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_000, 0)
	cat := &domain.Cat{
		Name: "Барсик", Breed: domain.BreedBengal, Trait: domain.TraitBully,
		Level: 4, XP: 123, Energy: 80, EnergyUpdatedAt: now, Coins: 42,
		HPBase: 50, ATKBase: 41, DEFBase: 27, SPDBase: 31,
	}
	text := FormatCatProfileWithArena(cat, now, domain.ArenaStats{
		Wins: 8, Losses: 6, CurrentStreak: 3, StreakWins: true,
	})
	for _, want := range []string{
		"бенгал · уровень 4",
		"⭐ XP: 123/400\n⚡ 80/100 · можно тренироваться",
		"🥊 АРЕНА\nПобеды: 8 · Поражения: 6\nСерия: 3 побед",
		"🩸 Когти: 24.5 мм\n🐈 Вес: 5.5 кг\n🐾 Хвост: 46.0 см\n〰️ Усы: 31.0 см",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("FormatCatProfileWithArena() = %q, missing %q", text, want)
		}
	}
}

func TestNewCatShowsPhysicalStatsAndOnlyNextUnlock(t *testing.T) {
	c := &domain.Cat{Name: "Кот", Breed: domain.BreedBritish, Level: 1, Feline: domain.BaseFelineStats(domain.BreedBritish)}
	text := FormatCatProfile(c, time.Now())
	for _, forbidden := range []string{"HP", "ATK", "DEF", "SPD", "Монеты", "АРЕНА", "Lv3", "Lv5", "Lv8", "специализация"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("unexpected %q in %q", forbidden, text)
		}
	}
	if !strings.Contains(text, "первый предмет — Lv2") {
		t.Fatal("missing next unlock")
	}
}
