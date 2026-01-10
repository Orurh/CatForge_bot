package app

import (
	"context"

	"catforge/internal/domain"
)

type ProfileService struct {
	cats CatRepository
}

func NewProfileService(cats CatRepository) *ProfileService {
	return &ProfileService{cats: cats}
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
