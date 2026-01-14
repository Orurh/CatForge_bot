package app

import (
	"catforge/internal/domain"
	"context"
	"math"
	"time"
)

func (s *ArenaService) View(ctx context.Context, userID int64, salt string) (ArenaView, error) {
	now := s.clock.Now()

	cat, err := s.cats.GetByUserID(ctx, userID)
	if err != nil {
		return ArenaView{}, err
	}
	power := domain.Power(cat)
	energyNow := domain.RegenEnergy(cat.Energy, cat.EnergyUpdatedAt, now)

	st, err := s.repo.GetState(ctx, userID, now)
	if err != nil {
		return ArenaView{}, err
	}
	st = domain.RegenArenaTickets(st, now)
	if err := s.repo.SaveState(ctx, userID, st); err != nil {
		return ArenaView{}, err
	}

	canFree := (st.RerollsToday == 0) || now.After(st.RerollReadyAt) || now.Equal(st.RerollReadyAt)
	wait := time.Duration(0)
	if st.RerollsToday > 0 && now.Before(st.RerollReadyAt) {
		wait = st.RerollReadyAt.Sub(now)
		canFree = false
	}
	cost := domain.ArenaRerollEnergyCost
	canPay := energyNow >= cost

	if salt == "" {
		salt = "view"
	}
	seed := arenaSeed(now, userID, "view:"+salt)
	var scopeChatID int64
	var scopeChatType string
	if s.users != nil {
		homeID, homeType, err := s.users.GetHomeChat(ctx, userID)
		if err == nil && homeID != 0 && homeType != "" && homeType != "private" {
			scopeChatID = homeID
			scopeChatType = homeType
		}
	}

	ops, err := s.repo.FindOpponents(ctx, userID, power, scopeChatID, scopeChatType, seed)

	if err != nil {
		return ArenaView{}, err
	}

	out := make([]ArenaOpponentView, 0, len(ops))
	for _, op := range ops {
		p := domain.ArenaWinProbabilityWithRage(power, op.Power, st.Rage)
		wp := int(math.Round(p * 100))
		if wp < 0 {
			wp = 0
		}
		if wp > 100 {
			wp = 100
		}
		out = append(out, ArenaOpponentView{
			UserID:     op.UserID,
			Name:       op.Name,
			Breed:      op.Breed,
			Level:      op.Level,
			Power:      op.Power,
			Kind:       op.Kind,
			WinProbPct: wp,
		})
	}
	return ArenaView{
	State:         st,
	Power:         power,
	Opponents:     out,
	CanFreeReroll: canFree,
	RerollWait:    wait,
	CanPayReroll:  canPay,
	RerollCostE:   cost,
	EnergyNow:     energyNow,
}, nil
}
