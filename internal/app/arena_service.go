package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"catforge/internal/domain"
)

type ArenaService struct {
	repo            ArenaRepository
	cats            CatRepository
	users           UserRepository
	clock           Clock
	log             EventLog
	homeMinInterval time.Duration
}

type ArenaView struct {
	State     domain.ArenaState
	Power     int
	Opponents []ArenaOpponentView

	CanFreeReroll bool
	RerollWait    time.Duration
	CanPayReroll  bool
	RerollCostE   int
	EnergyNow     int
}

type ArenaOpponentView struct {
	UserID     int64
	Name       string
	Breed      domain.Breed
	Level      int
	Power      int
	Kind       domain.ArenaOpponentKind
	WinProbPct int
}

var (
	ErrArenaRerollCooldown  = errors.New("arena: reroll cooldown")
	ErrArenaNotEnoughEnergy = errors.New("arena: not enough energy")
)

type ArenaFightMeta struct {
	XPGain      int64
	LeveledUp   int
	RiskMulPct  int
	SeasonDelta int
	RageBefore  int
	RageAfter   int
}

func NewArenaService(repo ArenaRepository, cats CatRepository, users UserRepository, clock Clock, log EventLog) *ArenaService {
	return &ArenaService{
		repo:            repo,
		cats:            cats,
		users:           users,
		clock:           clock,
		log:             log,
		homeMinInterval: 3 * time.Second,
	}
}


func (s *ArenaService) Reroll(ctx context.Context, userID int64, pay bool) (domain.ArenaState, int, error) {
	now := s.clock.Now()
	cost := 0
	if pay {
		cost = domain.ArenaRerollEnergyCost
	}
	st, energyNow, err := s.repo.Reroll(ctx, userID, now, pay, cost)
	return st, energyNow, err
}


func (s *ArenaService) Fight(
	ctx context.Context,
	userID int64,
	telegramID int64,
	sourceChatID int64,
	sourceChatType string,
	opponentUserID int64,
) (domain.ArenaState, domain.ArenaFightResult, ArenaFightMeta, error) {
	now := s.clock.Now()
	seed := arenaSeed(now, userID, fmt.Sprintf("fight:%d", opponentUserID))
	st, res, xpGain, leveledUp, riskMulPct, seasonDelta, rageBefore, rageAfter, err :=
		s.repo.Fight(ctx, userID, opponentUserID, now, seed)
	if err != nil {
		return st, res, ArenaFightMeta{}, err
	}

	if s.log != nil {
		targetChatID, err := pickPublicChat(ctx, s.users, userID, now, s.homeMinInterval, sourceChatID, sourceChatType)
		if err != nil {
			targetChatID = 0
		}

		if targetChatID != 0 {
			attCat, _ := s.cats.GetByUserID(ctx, userID)
			defCat, _ := s.cats.GetByUserID(ctx, opponentUserID)

			attName := ""
			attBreed := domain.Breed("")
			attLevel := 1
			if attCat != nil {
				attName = attCat.Name
				attBreed = attCat.Breed
				attLevel = attCat.Level
			}

			defName := ""
			defBreed := domain.Breed("")
			defLevel := 1
			if defCat != nil {
				defName = defCat.Name
				defBreed = defCat.Breed
				defLevel = defCat.Level
			}

			winProbPct := int(res.WinProb*100 + 0.5)

			_ = s.log.ArenaFight(ctx, ArenaFightEvent{
				ChatID:        targetChatID,
				TelegramID:    telegramID,
				CatName:       attName,
				Breed:         attBreed,
				Level:         attLevel,
				Now:           now,
				OpponentName:  defName,
				OpponentBreed: defBreed,
				OpponentLevel: defLevel,
				OpponentPower: res.DefenderPower,
				AttackerPower: res.AttackerPower,
				WinProb:       winProbPct,
				Won:           res.AttackerWon,
				RatingDelta:   res.RatingDelta,
				NewRating:     st.Rating,
				XPGain:        xpGain,
				LeveledUp:     leveledUp,
				RageBefore:    rageBefore,
				RageAfter:     rageAfter,
				RiskMulPct:    riskMulPct,
				SeasonDelta:   seasonDelta,
			})
		}
	}

	return st, res, ArenaFightMeta{
		XPGain:      xpGain,
		LeveledUp:   leveledUp,
		RiskMulPct:  riskMulPct,
		SeasonDelta: seasonDelta,
		RageBefore:  rageBefore,
		RageAfter:   rageAfter,
	}, nil

}

func arenaSeed(now time.Time, userID int64, salt string) string {
	// достаточно стабильный seed, чтобы “одинаковая кнопка” давала повторяемость в пределах секунды
	return fmt.Sprintf("%d:%d:%s", now.UTC().Unix(), userID, salt)
}
