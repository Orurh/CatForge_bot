package gamedata

import (
	"testing"

	"catforge/internal/domain"
)

func TestItemCatalogShape(t *testing.T) {
	items, err := Items()
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	if len(items) != 12 {
		t.Fatalf("item count = %d, want 12", len(items))
	}
	slots := map[domain.ItemSlot]int{}
	rarities := map[domain.ItemRarity]int{}
	locations := map[domain.ExpeditionLocation]int{}
	for _, item := range items {
		slots[item.Slot]++
		rarities[item.Rarity]++
		locations[item.Location]++
		if item.StatsAtLevel(5) == item.StatsAtLevel(1) {
			t.Fatalf("item %q does not grow with level", item.ID)
		}
	}
	if len(slots) != 3 || len(rarities) != 3 || len(locations) != 3 {
		t.Fatalf("catalog dimensions: slots=%v rarities=%v locations=%v", slots, rarities, locations)
	}
}

func TestUpgradeCosts(t *testing.T) {
	for level := 1; level < domain.ItemMaxLevel; level++ {
		fragments, coins, ok := domain.UpgradeCost(level)
		if !ok || fragments <= 0 || coins <= 0 {
			t.Fatalf("invalid upgrade cost for level %d: %d %d %v", level, fragments, coins, ok)
		}
	}
	if _, _, ok := domain.UpgradeCost(domain.ItemMaxLevel); ok {
		t.Fatal("max-level item must not be upgradeable")
	}
}

func TestLootCountsMatchCatalog(t *testing.T) {
	t.Parallel()
	common, rare, epic, err := LootCounts(domain.ExpeditionAlley)
	if err != nil {
		t.Fatal(err)
	}
	if common != 1 || rare != 3 || epic != 0 {
		t.Fatalf("alley loot counts = %d/%d/%d, want 1/3/0", common, rare, epic)
	}
	candidates, err := LootCandidates(domain.ExpeditionAlley, domain.ItemCommon)
	if err != nil || len(candidates) != common || candidates[0].ID != "string_collar" {
		t.Fatalf("unexpected candidates: %+v, err=%v", candidates, err)
	}
}
