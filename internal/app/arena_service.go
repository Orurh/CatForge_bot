package app

import (
	"context"
	"fmt"
	"time"

	"catforge/internal/domain"
)

type ArenaService struct {
	repo  ArenaRepository
	cats  CatRepository
	clock Clock
}

type ArenaView struct {
	State     domain.ArenaState
	Power     int
	Opponents []domain.ArenaOpponent
}

func NewArenaService(repo ArenaRepository, cats CatRepository, clock Clock) *ArenaService {
	return &ArenaService{repo: repo, cats: cats, clock: clock}
}

func (s *ArenaService) View(ctx context.Context, userID int64, salt string) (ArenaView, error) {
	now := s.clock.Now()

	cat, err := s.cats.GetByUserID(ctx, userID)
	if err != nil {
		return ArenaView{}, err
	}
	power := domain.Power(cat)

	st, err := s.repo.GetState(ctx, userID)
	if err != nil {
		return ArenaView{}, err
	}
	st = domain.RegenArenaTickets(st, now)
	if err := s.repo.SaveState(ctx, userID, st); err != nil {
		return ArenaView{}, err
	}

	if salt == "" {
		salt = "view"
	}
	seed := arenaSeed(now, userID, "view:"+salt)
	ops, err := s.repo.FindOpponents(ctx, userID, power, seed)
	if err != nil {
		return ArenaView{}, err
	}

	return ArenaView{State: st, Power: power, Opponents: ops}, nil
}

func (s *ArenaService) Fight(ctx context.Context, userID, opponentUserID int64) (domain.ArenaState, domain.ArenaFightResult, error) {
	now := s.clock.Now()
	seed := arenaSeed(now, userID, fmt.Sprintf("fight:%d", opponentUserID))
	return s.repo.Fight(ctx, userID, opponentUserID, seed)
}

func arenaSeed(now time.Time, userID int64, salt string) string {
	// достаточно стабильный seed, чтобы “одинаковая кнопка” давала повторяемость в пределах секунды
	return fmt.Sprintf("%d:%d:%s", now.UTC().Unix(), userID, salt)
}
