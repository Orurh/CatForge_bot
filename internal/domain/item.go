package domain

type ItemSlot string

const (
	ItemSlotClaws  ItemSlot = "claws"
	ItemSlotCollar ItemSlot = "collar"
	ItemSlotCharm  ItemSlot = "charm"
)

type ItemRarity string

const (
	ItemCommon ItemRarity = "common"
	ItemRare   ItemRarity = "rare"
	ItemEpic   ItemRarity = "epic"
)

const ItemMaxLevel = 5

type ItemDefinition struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Slot     ItemSlot           `json:"slot"`
	Rarity   ItemRarity         `json:"rarity"`
	Location ExpeditionLocation `json:"location"`
	Base     StatDelta          `json:"base_stats"`
	PerLevel StatDelta          `json:"per_level_stats"`
	EffectID string             `json:"effect_id,omitempty"`
}

func (definition ItemDefinition) StatsAtLevel(level int) StatDelta {
	level = max(1, min(level, ItemMaxLevel))
	steps := level - 1
	return StatDelta{
		HP:  definition.Base.HP + definition.PerLevel.HP*steps,
		ATK: definition.Base.ATK + definition.PerLevel.ATK*steps,
		DEF: definition.Base.DEF + definition.PerLevel.DEF*steps,
		SPD: definition.Base.SPD + definition.PerLevel.SPD*steps,
	}
}

type OwnedItem struct {
	ItemID    string
	Level     int
	Fragments int
	Equipped  bool
}

type EquippedItem struct {
	ItemID string
	Level  int
	Slot   ItemSlot
}

type Loadout struct {
	Claws  *EquippedItem
	Collar *EquippedItem
	Charm  *EquippedItem
}

func UpgradeCost(level int) (fragments int, coins int64, ok bool) {
	switch level {
	case 1:
		return 1, 50, true
	case 2:
		return 2, 120, true
	case 3:
		return 4, 250, true
	case 4:
		return 7, 500, true
	default:
		return 0, 0, false
	}
}
