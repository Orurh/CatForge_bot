package domain

// FelineStats are physical measurements, stored exclusively as integers.
// Legacy RPG fields remain only for the disabled expedition protocol.
type FelineStats struct {
	ClawsTenthMM  int `json:"claws_tenth_mm"`
	WeightGrams   int `json:"weight_grams"`
	TailMM        int `json:"tail_mm"`
	WhiskerSpanMM int `json:"whisker_span_mm"`
}

func (s FelineStats) Valid() bool {
	return s.ClawsTenthMM > 0 && s.WeightGrams > 0 && s.TailMM > 0 && s.WhiskerSpanMM > 0
}
func (s *FelineStats) Add(d FelineStats) {
	s.ClawsTenthMM += d.ClawsTenthMM
	s.WeightGrams += d.WeightGrams
	s.TailMM += d.TailMM
	s.WhiskerSpanMM += d.WhiskerSpanMM
}
func BaseFelineStats(b Breed) FelineStats {
	switch b {
	case BreedMaineCoon:
		return FelineStats{100, 6000, 320, 180}
	case BreedSiamese:
		return FelineStats{100, 4000, 400, 300}
	case BreedBritish:
		return FelineStats{100, 5500, 250, 300}
	case BreedBengal:
		return FelineStats{150, 4500, 350, 200}
	default:
		return FelineStats{100, 5000, 300, 300}
	}
}

// Used by simulations and migration previews; live progress is calculated by C++.
func FelineGrowth(b Breed) FelineStats {
	switch b {
	case BreedMaineCoon:
		return FelineStats{5, 300, 20, 20}
	case BreedSiamese:
		return FelineStats{5, 100, 30, 30}
	case BreedBritish:
		return FelineStats{5, 300, 10, 30}
	case BreedBengal:
		return FelineStats{15, 100, 30, 10}
	default:
		return FelineStats{10, 200, 20, 20}
	}
}
func (c Cat) PhysicalStats() FelineStats {
	if c.Feline.Valid() {
		return c.Feline
	}
	s := BaseFelineStats(c.Breed)
	hp, atk, def, spd := BaseStatsByBreed(c.Breed)
	s.Add(FelineStats{max(0, c.ATKBase-atk) * 5, max(0, c.HPBase-hp) * 100, max(0, c.SPDBase-spd) * 10, max(0, c.DEFBase-def) * 10})
	return s
}

type Unlock struct {
	Level int
	ID    string
	Label string
}

var ProgressionUnlocks = [...]Unlock{
	{2, "first_item", "первый предмет"}, {3, "arena_unlocked", "Арена"},
	{5, "equipment_slot_2", "второй слот снаряжения"}, {8, "equipment_slot_3", "третий слот снаряжения"},
}

func NextUnlock(level int) (Unlock, bool) {
	for _, u := range ProgressionUnlocks {
		if u.Level > level {
			return u, true
		}
	}
	return Unlock{}, false
}
func CrossedUnlocks(before, after int) []string {
	var result []string
	for _, u := range ProgressionUnlocks {
		if before < u.Level && after >= u.Level {
			result = append(result, u.ID)
		}
	}
	return result
}
func EquipmentSlots(level int) int {
	switch {
	case level >= 8:
		return 3
	case level >= 5:
		return 2
	case level >= 2:
		return 1
	default:
		return 0
	}
}
func SlotUnlocked(slot ItemSlot, level int) bool {
	switch slot {
	case ItemSlotClaws:
		return level >= 2
	case ItemSlotCollar:
		return level >= 5
	case ItemSlotCharm:
		return level >= 8
	}
	return false
}

// Reserved for the next epic; no unavailable perk is advertised in the UI.
const SpecializationUnlockLevel = 10

func EquipmentSlotLevel(slot ItemSlot) int {
	switch slot {
	case ItemSlotClaws:
		return 2
	case ItemSlotCollar:
		return 5
	case ItemSlotCharm:
		return 8
	}
	return 0
}
