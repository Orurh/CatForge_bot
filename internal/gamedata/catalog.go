package gamedata

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"catforge/internal/domain"
)

//go:embed items.json
var itemsJSON []byte

var (
	loadOnce sync.Once
	items    []domain.ItemDefinition
	byID     map[string]domain.ItemDefinition
	loadErr  error
)

func Items() ([]domain.ItemDefinition, error) {
	loadOnce.Do(load)
	return append([]domain.ItemDefinition(nil), items...), loadErr
}

func ItemByID(id string) (domain.ItemDefinition, bool) {
	loadOnce.Do(load)
	item, ok := byID[id]
	return item, ok
}

func LootCandidates(location domain.ExpeditionLocation, rarity domain.ItemRarity) ([]domain.ItemDefinition, error) {
	catalog, err := Items()
	if err != nil {
		return nil, err
	}
	result := make([]domain.ItemDefinition, 0)
	for _, item := range catalog {
		if item.Location == location && item.Rarity == rarity {
			result = append(result, item)
		}
	}
	return result, nil
}

func LootCounts(location domain.ExpeditionLocation) (common, rare, epic int, err error) {
	catalog, err := Items()
	if err != nil {
		return 0, 0, 0, err
	}
	for _, item := range catalog {
		if item.Location != location {
			continue
		}
		switch item.Rarity {
		case domain.ItemCommon:
			common++
		case domain.ItemRare:
			rare++
		case domain.ItemEpic:
			epic++
		}
	}
	return common, rare, epic, nil
}

func load() {
	if err := json.Unmarshal(itemsJSON, &items); err != nil {
		loadErr = fmt.Errorf("decode item catalog: %w", err)
		return
	}
	byID = make(map[string]domain.ItemDefinition, len(items))
	for index, item := range items {
		if err := validateItem(item); err != nil {
			loadErr = fmt.Errorf("item %d: %w", index, err)
			return
		}
		if _, exists := byID[item.ID]; exists {
			loadErr = fmt.Errorf("duplicate item id %q", item.ID)
			return
		}
		byID[item.ID] = item
	}
}

func validateItem(item domain.ItemDefinition) error {
	cap := map[domain.ItemRarity]int{domain.ItemCommon: 1, domain.ItemRare: 2, domain.ItemEpic: 3}[item.Rarity]
	if item.TrainingCritBonusPercent < 0 || item.TrainingCritBonusPercent > cap {
		return errors.New("invalid training crit bonus for rarity")
	}
	if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Name) == "" {
		return errors.New("id and name are required")
	}
	if item.Slot != domain.ItemSlotClaws && item.Slot != domain.ItemSlotCollar && item.Slot != domain.ItemSlotCharm {
		return fmt.Errorf("invalid slot %q", item.Slot)
	}
	if item.Rarity != domain.ItemCommon && item.Rarity != domain.ItemRare && item.Rarity != domain.ItemEpic {
		return fmt.Errorf("invalid rarity %q", item.Rarity)
	}
	if item.Location != domain.ExpeditionAlley && item.Location != domain.ExpeditionRooftop && item.Location != domain.ExpeditionPark {
		return fmt.Errorf("invalid location %q", item.Location)
	}
	stats := item.StatsAtLevel(1)
	if stats.HP < 0 || stats.ATK < 0 || stats.DEF < 0 || stats.SPD < 0 || stats == (domain.StatDelta{}) {
		return errors.New("base stats must contain a positive bonus")
	}
	n := 0
	for _, v := range []int{item.FelineBonus.ClawsTenthMM, item.FelineBonus.WeightGrams, item.FelineBonus.TailMM, item.FelineBonus.WhiskerSpanMM} {
		if v < 0 {
			return errors.New("negative physical bonus")
		}
		if v > 0 {
			n++
		}
	}
	if n > 1 {
		return errors.New("items allow at most one physical bonus")
	}
	if item.EffectID == "" || item.Ability == "" {
		return errors.New("item capability is required")
	}
	return nil
}
