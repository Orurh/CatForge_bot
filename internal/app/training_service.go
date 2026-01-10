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
		targetChatID := int64(0)

		// 1) Если действие было в группе/канале — логируем туда же.
		if sourceChatType != "" && sourceChatType != "private" {
			targetChatID = sourceChatID
		} else if s.users != nil {
			// 2) Если действие было в личке — пробуем логировать в "домашний чат" пользователя.
			homeID, homeType, err := s.users.GetHomeChat(ctx, userID)
			if err == nil && homeID != 0 && homeType != "" && homeType != "private" {
				ok, _ := s.users.TryTouchHomeChatLog(ctx, userID, now, s.homeMinInterval)
				if ok {
					targetChatID = homeID
				}
			}
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
