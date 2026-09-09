package domain

import "testing"

func TestGrantXPAppliesLevelThresholdsAndStats(t *testing.T) {
	t.Parallel()
	cat := Cat{Breed: BreedBengal, Level: 2, XP: 190, HPBase: 30, ATKBase: 20, DEFBase: 10, SPDBase: 20}
	progress := GrantXP(&cat, 220)
	if progress.XPGain != 220 || progress.LevelsGained != 1 || cat.Level != 3 || cat.XP != 210 {
		t.Fatalf("progress/cat = %+v/%+v", progress, cat)
	}
	if progress.StatsGained != (StatDelta{HP: 3, ATK: 2, DEF: 1, SPD: 2}) {
		t.Fatalf("stats gained = %+v", progress.StatsGained)
	}
}
