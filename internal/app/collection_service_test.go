package app

import (
	"context"
	"testing"

	"catforge/internal/domain"
)

type fakeItems struct {
	owned        []domain.OwnedItem
	equippedID   string
	equippedSlot domain.ItemSlot
	upgraded     domain.OwnedItem
	saveResults  []bool
	saveCalls    int
	savedItemID  string
	savedBonus   domain.StatDelta
	saveItem     domain.OwnedItem
	saveIsNew    bool
}

func (f *fakeItems) ListBestiary(context.Context, int64) ([]domain.BestiaryEntry, error) {
	return nil, nil
}

func (f *fakeItems) ListOwned(context.Context, int64) ([]domain.OwnedItem, error) {
	return append([]domain.OwnedItem(nil), f.owned...), nil
}

func (f *fakeItems) Equip(_ context.Context, _ int64, slot domain.ItemSlot, itemID string) error {
	f.equippedID, f.equippedSlot = itemID, slot
	return nil
}

func (f *fakeItems) Upgrade(_ context.Context, _ int64, itemID string, expectedLevel, fragments int, coins int64) (domain.OwnedItem, error) {
	f.upgraded = domain.OwnedItem{ItemID: itemID, Level: expectedLevel + 1, Fragments: 0}
	if fragments != 1 || coins != 50 {
		return domain.OwnedItem{}, domain.ErrNotEnoughFragments
	}
	return f.upgraded, nil
}

func (f *fakeItems) SaveExpedition(_ context.Context, _ int64, _ int64, _ domain.Cat, itemID string, _ domain.EnemyKind, _ bool) (ExpeditionSaveResult, error) {
	saved := true
	if f.saveCalls < len(f.saveResults) {
		saved = f.saveResults[f.saveCalls]
	}
	f.saveCalls++
	f.savedItemID = itemID
	return ExpeditionSaveResult{Saved: saved, Item: f.saveItem, IsNew: f.saveIsNew}, nil
}

func TestCollectionServiceEquipUsesCatalogSlot(t *testing.T) {
	t.Parallel()
	repo := &fakeItems{owned: []domain.OwnedItem{{ItemID: "rat_tooth", Level: 1}}}
	service := NewCollectionService(repo)
	if err := service.Equip(context.Background(), 1, "rat_tooth"); err != nil {
		t.Fatalf("Equip() error = %v", err)
	}
	if repo.equippedID != "rat_tooth" || repo.equippedSlot != domain.ItemSlotClaws {
		t.Fatalf("unexpected equip: id=%s slot=%s", repo.equippedID, repo.equippedSlot)
	}
}

func TestCollectionServiceUpgradeAndEffectiveStats(t *testing.T) {
	t.Parallel()
	repo := &fakeItems{owned: []domain.OwnedItem{{ItemID: "rat_tooth", Level: 1, Fragments: 1, Equipped: true}}}
	service := NewCollectionService(repo)
	item, err := service.Upgrade(context.Background(), 1, "rat_tooth")
	if err != nil || item.Level != 2 {
		t.Fatalf("Upgrade() = %+v, %v", item, err)
	}
	entries, err := service.List(context.Background(), 1)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if stats := EffectiveItemStats(entries); stats.ATK != 3 {
		t.Fatalf("effective item stats = %+v, want ATK 3", stats)
	}
}
