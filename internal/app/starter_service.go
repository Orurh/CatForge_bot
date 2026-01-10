package app

import (
	"context"
	"strconv"

	"catforge/internal/domain"
)

type StarterService struct {
	cats CatRepository
	rng  RNG
}

func NewStarterService(cats CatRepository, rng RNG) *StarterService {
	return &StarterService{cats: cats, rng: rng}
}

func (s *StarterService) ChooseStarterCat(ctx context.Context, userID int64, breed domain.Breed) (*domain.Cat, error) {
	trait := s.randomTrait()
	hp, atk, def, spd := baseByBreed(breed)
	name := s.defaultName()
	return s.cats.Create(ctx, userID, name, breed, trait, hp, atk, def, spd)
}

func (s *StarterService) randomTrait() domain.Trait {
	traits := []domain.Trait{"lazy", "bully", "philosopher", "neat", "sleepy"}
	return traits[s.rng.Intn(len(traits))]
}

func (s *StarterService) defaultName() string {
	// "Мурчалкин123" (цифры рандомные)
	// 100..999, чтобы всегда были 3 цифры.
	n := 100 + s.rng.Intn(900)
	return domain.CatNameDefaultPrefix + strconv.Itoa(n)
}

func baseByBreed(b domain.Breed) (hp, atk, def, spd int) {
	switch b {
	case domain.BreedMaineCoon:
		return 50, 18, 18, 14
	case domain.BreedSiamese:
		return 38, 18, 18, 26
	case domain.BreedBritish:
		return 42, 16, 26, 16
	case domain.BreedBengal:
		return 38, 26, 16, 20
	default:
		return 40, 20, 20, 20
	}
}
