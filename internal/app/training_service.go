package app

import (
	"context"
	"time"

	"catforge/internal/domain"
)

type TrainingService struct {
	cats            CatRepository
	users           UserRepository
	clock           Clock
	log             EventLog
	homeMinInterval time.Duration
}

func NewTrainingService(cats CatRepository, users UserRepository, clock Clock, log EventLog) *TrainingService {
	return &TrainingService{
		cats:            cats,
		users:           users,
		clock:           clock,
		log:             log,
		homeMinInterval: 3 * time.Second,
	}
}

func (s *TrainingService) Train(
	ctx context.Context,
	userID int64,
	telegramID int64,
	sourceChatID int64,
	sourceChatType string,
) (*domain.Cat, domain.TrainResult, error) {

	now := s.clock.Now()

	cat, res, err := s.cats.Train(ctx, userID, now)
	if err != nil {
		return nil, domain.TrainResult{}, err
	}

	// Публичные чаты: публикуем событие.
	if s.log != nil {
		targetChatID, err := pickPublicChat(ctx, s.users, userID, now, s.homeMinInterval, sourceChatID, sourceChatType)
		if err != nil {
			targetChatID = 0
		}

		if targetChatID != 0 {
			energyNow := domain.RegenEnergy(cat.Energy, cat.EnergyUpdatedAt, now)

			_ = s.log.Training(ctx, TrainingEvent{
				ChatID:     targetChatID,
				TelegramID: telegramID,
				CatName:    cat.Name,
				Breed:      cat.Breed,
				Now:        now,
				Energy:     energyNow,
				Level:      cat.Level,
				Result:     res,
			})
		}
	}

	return cat, res, nil
}
