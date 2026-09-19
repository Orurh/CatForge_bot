package views

import (
	"catforge/internal/domain"
	"testing"
)

func TestTraitRussianNames(t *testing.T) {
	for trait, want := range map[domain.Trait]string{
		domain.TraitLazy: "лентяй", domain.TraitBully: "борзый", domain.TraitVeteran: "бывалый",
		domain.TraitCunning: "хитрый", domain.TraitArrogant: "наглый", domain.TraitAngry: "злой",
	} {
		if got := TraitRU(trait); got != want {
			t.Errorf("%s: got %q, want %q", trait, got, want)
		}
	}
}
