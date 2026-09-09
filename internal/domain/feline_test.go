package domain

import "testing"

func TestProgressionUnlocksOnlyExposeNextMilestone(t *testing.T) {
	for level, want := range map[int]int{1: 2, 2: 3, 3: 5, 4: 5, 5: 8, 7: 8, 8: 0, 10: 0} {
		next, ok := NextUnlock(level)
		if (want != 0) != ok || next.Level != want {
			t.Errorf("Lv%d next=%+v %v", level, next, ok)
		}
	}
	for level, want := range map[int]int{1: 0, 2: 1, 3: 1, 5: 2, 8: 3, 20: 3} {
		if EquipmentSlots(level) != want {
			t.Errorf("Lv%d slots", level)
		}
	}
	if SlotUnlocked(ItemSlotCharm, 7) || SlotUnlocked(ItemSlotCollar, 4) || SlotUnlocked(ItemSlotClaws, 1) {
		t.Fatal("locked equipment slot available")
	}
	if len(CrossedUnlocks(3, 3)) != 0 || len(CrossedUnlocks(2, 8)) != 3 {
		t.Fatal("unlock transition")
	}
}
func TestSpecialActionsRequireTheirActualCapability(t *testing.T) {
	stats := BaseFelineStats(BreedBritish)
	for _, action := range SpecialActions {
		if action.RequiredEffect == "" {
			continue
		}
		if action.Available(stats, nil) {
			t.Fatalf("%s without item", action.ID)
		}
		if !action.Available(stats, []string{action.RequiredEffect}) {
			t.Fatalf("%s with item", action.ID)
		}
	}
	ledge, _ := SpecialActionByID("ledge")
	stats.TailMM = 339
	if ledge.Available(stats, nil) {
		t.Fatal("short tail passes")
	}
	stats.TailMM = 340
	if !ledge.Available(stats, nil) {
		t.Fatal("threshold blocked")
	}
}
