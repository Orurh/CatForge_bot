package main

import (
	"reflect"
	"testing"

	"catforge/internal/domain"
)

func TestCatAtLevelUsesEngineProgression(t *testing.T) {
	t.Parallel()
	cat := catAtLevel(1, domain.BreedBengal, 4)
	if cat.Level != 4 || cat.HPBase != 49 || cat.ATKBase != 28 || cat.DEFBase != 19 || cat.SPDBase != 26 {
		t.Fatalf("catAtLevel() = %+v", cat)
	}
}

func TestParseDiffs(t *testing.T) {
	t.Parallel()
	diffs, err := parseDiffs("0, 3,5")
	if err != nil || !reflect.DeepEqual(diffs, []int{0, 3, 5}) {
		t.Fatalf("parseDiffs() = %v, %v", diffs, err)
	}
	if _, err := parseDiffs("-1"); err == nil {
		t.Fatal("parseDiffs(-1) error = nil")
	}
}
