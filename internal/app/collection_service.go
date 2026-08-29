package app

import (
	"context"
	"fmt"

	"catforge/internal/domain"
	"catforge/internal/gamedata"
)

type CollectionEntry struct {
	Definition domain.ItemDefinition
	Owned      domain.OwnedItem
}

type CollectionService struct {
	items ItemRepository
}

func NewCollectionService(items ItemRepository) *CollectionService {
	return &CollectionService{items: items}
}

func (s *CollectionService) List(ctx context.Context, userID int64) ([]CollectionEntry, error) {
	owned, err := s.items.ListOwned(ctx, userID)
	if err != nil {
		return nil, err
	}
	entries := make([]CollectionEntry, 0, len(owned))
	for _, item := range owned {
		definition, ok := gamedata.ItemByID(item.ItemID)
		if !ok {
			return nil, fmt.Errorf("unknown owned item %q", item.ItemID)
		}
		entries = append(entries, CollectionEntry{Definition: definition, Owned: item})
	}
	return entries, nil
}

func (s *CollectionService) Equip(ctx context.Context, userID int64, itemID string) error {
	definition, ok := gamedata.ItemByID(itemID)
	if !ok {
		return domain.ErrItemNotOwned
	}
	return s.items.Equip(ctx, userID, definition.Slot, itemID)
}

func (s *CollectionService) Upgrade(ctx context.Context, userID int64, itemID string) (domain.OwnedItem, error) {
	entries, err := s.List(ctx, userID)
	if err != nil {
		return domain.OwnedItem{}, err
	}
	for _, entry := range entries {
		if entry.Owned.ItemID != itemID {
			continue
		}
		fragments, coins, ok := domain.UpgradeCost(entry.Owned.Level)
		if !ok {
			return domain.OwnedItem{}, domain.ErrItemMaxLevel
		}
		return s.items.Upgrade(ctx, userID, itemID, entry.Owned.Level, fragments, coins)
	}
	return domain.OwnedItem{}, domain.ErrItemNotOwned
}

func EffectiveItemStats(entries []CollectionEntry) domain.StatDelta {
	var total domain.StatDelta
	for _, entry := range entries {
		if entry.Owned.Equipped {
			total.Add(entry.Definition.StatsAtLevel(entry.Owned.Level))
		}
	}
	return total
}
