package app

import (
	"context"
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
	Opponents []domain.ArenaOpponent
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

func (s *ArenaService) View(ctx context.Context, userID int64, salt string) (ArenaView, error) {
	now := s.clock.Now()

	cat, err := s.cats.GetByUserID(ctx, userID)
	if err != nil {
		return ArenaView{}, err
	}
	power := domain.Power(cat)

	st, err := s.repo.GetState(ctx, userID, now)
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

	return ArenaView{State: st, Power: power, Opponents: ops}, nil
}

func (s *ArenaService) Fight(
	ctx context.Context,
	userID int64,
	telegramID int64,
	sourceChatID int64,
	sourceChatType string,
	opponentUserID int64,
) (domain.ArenaState, domain.ArenaFightResult, error) {
	now := s.clock.Now()
	seed := arenaSeed(now, userID, fmt.Sprintf("fight:%d", opponentUserID))
	st, res, xpGain, leveledUp, err := s.repo.Fight(ctx, userID, opponentUserID, now, seed)
	if err != nil {
		return st, res, err
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
				RageAfter:     st.Rage,
			})
		}
	}

	return st, res, nil
}

func arenaSeed(now time.Time, userID int64, salt string) string {
	// достаточно стабильный seed, чтобы “одинаковая кнопка” давала повторяемость в пределах секунды
	return fmt.Sprintf("%d:%d:%s", now.UTC().Unix(), userID, salt)
}
