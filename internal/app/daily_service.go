package app

import (
	"context"
	"time"

	"catforge/internal/domain"
)

type DailyService struct {
	repo  DailyRepository
	clock Clock
	users UserRepository
    cats  CatRepository
    log   EventLog
    homeMinInterval time.Duration
}

func NewDailyService(repo DailyRepository, users UserRepository, cats CatRepository, clock Clock, log EventLog) *DailyService {
    return &DailyService{
        repo: repo, users: users, cats: cats, clock: clock, log: log,
        homeMinInterval: 3 * time.Second,
    }
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

    if s.log != nil && res.Outcome == domain.DailyClaimOK {
        targetChatID, err := pickPublicChat(ctx, s.users, userID, now, s.homeMinInterval, 0, "private")
        if err != nil {
            targetChatID = 0
        }

        if targetChatID != 0 {
            name := ""
            breed := domain.Breed("")
            if cat != nil {
                name = cat.Name
                breed = cat.Breed
            }
            _ = s.log.DailyClaim(ctx, DailyClaimEvent{
                ChatID: targetChatID,
                CatName: name,
                Breed: breed,
                Now: now,
                Streak: res.Streak,
                XPGain: res.XPGain,
                EnergyGain: res.EnergyGain,
            })
        }
    }
	return cat, res, nil
}
