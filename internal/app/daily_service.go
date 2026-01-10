package app

import (
	"context"

	"catforge/internal/domain"
)

type DailyService struct {
	repo  DailyRepository
	clock Clock
}

func NewDailyService(repo DailyRepository, clock Clock) *DailyService {
	return &DailyService{repo: repo, clock: clock}
}

func (s *DailyService) State(ctx context.Context, userID int64) (domain.DailyView, error) {
	now := s.clock.Now()
	st, err := s.repo.GetState(ctx, userID)
	if err != nil {
		return domain.DailyView{}, err
	}
	return domain.MakeDailyView(st, now), nil
}

func (s *DailyService) Claim(ctx context.Context, userID int64) (*domain.Cat, domain.DailyClaimResult, error) {
	now := s.clock.Now()
	cat, res, err := s.repo.Claim(ctx, userID, now)
	if err != nil {
		return nil, domain.DailyClaimResult{}, err
	}
	if res.NextAt.IsZero() {
		res.NextAt = domain.NextDailyAt(now)
	}
	return cat, res, nil
}
