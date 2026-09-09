package app

import (
	"context"

	"catforge/internal/domain"
)

type ProfileService struct {
	cats   CatRepository
	fights FightRepository
}

func NewProfileService(cats CatRepository, fights ...FightRepository) *ProfileService {
	service := &ProfileService{cats: cats}
	if len(fights) > 0 {
		service.fights = fights[0]
	}
	return service
}

func (s *ProfileService) ArenaStats(ctx context.Context, catID int64) (domain.ArenaStats, error) {
	if s.fights == nil {
		return domain.ArenaStats{}, nil
	}
	return s.fights.CatStats(ctx, catID)
}

func (s *ProfileService) GetCat(ctx context.Context, userID int64) (*domain.Cat, error) {
	return s.cats.GetByUserID(ctx, userID)
}

func (s *ProfileService) ResetCat(ctx context.Context, userID int64) error {
	return s.cats.DeleteByUserID(ctx, userID)
}

func (s *ProfileService) RenameCat(ctx context.Context, userID int64, name string) (*domain.Cat, error) {
	return s.cats.SetName(ctx, userID, name)
}
