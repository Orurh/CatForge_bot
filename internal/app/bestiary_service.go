package app

import (
	"context"

	"catforge/internal/domain"
)

type BestiaryService struct{ items ItemRepository }

func NewBestiaryService(items ItemRepository) *BestiaryService { return &BestiaryService{items: items} }

func (s *BestiaryService) List(ctx context.Context, userID int64) ([]domain.BestiaryEntry, error) {
	return s.items.ListBestiary(ctx, userID)
}
