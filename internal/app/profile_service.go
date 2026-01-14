package app

import (
	"context"

	"catforge/internal/domain"
)

type ProfileService struct {
	cats CatRepository
    arena ArenaRepository
    clock Clock
}

func NewProfileService(cats CatRepository, arena ArenaRepository, clock Clock) *ProfileService {
    return &ProfileService{
        cats:  cats,
        arena: arena,
        clock: clock,
    }
}

func (s *ProfileService) GetCat(ctx context.Context, userID int64) (*domain.Cat, error) {
	return s.cats.GetByUserID(ctx, userID)
}

func (s *ProfileService) ResetCat(ctx context.Context, userID int64) error {
    if err := s.cats.DeleteByUserID(ctx, userID); err != nil {
        return err
    }

    // Arena state belongs to the user, not the cat. Reset it on "new cat".
    if s.arena != nil && s.clock != nil {
        now := s.clock.Now()
        if err := s.arena.ResetState(ctx, userID, now); err != nil {
            return err
        }
    }
    return nil
}

func (s *ProfileService) RenameCat(ctx context.Context, userID int64, name string) (*domain.Cat, error) {
	return s.cats.SetName(ctx, userID, name)
}
