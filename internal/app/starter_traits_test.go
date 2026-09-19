package app

import (
	"context"
	"testing"

	"catforge/internal/domain"
)

type starterTraitCats struct {
	stubCats
	trait domain.Trait
}

func (s *starterTraitCats) Create(_ context.Context, uid int64, name string, breed domain.Breed, trait domain.Trait, hp, atk, def, spd int) (*domain.Cat, error) {
	s.trait = trait
	return &domain.Cat{ID: 1, UserID: uid, Name: name, Breed: breed, Trait: trait, HPBase: hp, ATKBase: atk, DEFBase: def, SPDBase: spd}, nil
}

func TestStarterCanCreateEachNewTrait(t *testing.T) {
	for index, want := range []domain.Trait{domain.TraitLazy, domain.TraitBully, domain.TraitVeteran, domain.TraitCunning, domain.TraitArrogant, domain.TraitAngry} {
		cats := &starterTraitCats{}
		service := NewStarterService(cats, fakeClock{}, &sequenceRNG{values: []int{index, 0}}, nil)
		cat, err := service.ChooseStarterCat(context.Background(), 7, domain.BreedBritish)
		if err != nil || cat.Trait != want || cats.trait != want {
			t.Fatalf("starter trait %s: %+v %v", want, cat, err)
		}
	}
}
