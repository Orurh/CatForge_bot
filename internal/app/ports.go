package app

import (
	"context"
	"time"

	"catforge/internal/domain"
	"catforge/internal/pkg/randx"
)

// Clock provides current time (for deterministic tests).
type Clock interface {
	Now() time.Time
}

type RNG = randx.RNG

// EventLog publishes public events (groups/channels) from usecases.
// Transport implements it (telegram adapter); domain/app layer only depends on this interface.
type EventLog interface {
	Training(ctx context.Context, e TrainingEvent) error
	DailyClaim(ctx context.Context, e DailyClaimEvent) error
    ArenaFight(ctx context.Context, e ArenaFightEvent) error
}

type UserRepository interface {
	EnsureUser(ctx context.Context, telegramID int64) (int64, error)
	GetPendingAction(ctx context.Context, userID int64) (string, error)
	SetPendingAction(ctx context.Context, userID int64, action string) error
	GetHomeChat(ctx context.Context, userID int64) (chatID int64, chatType string, err error)
	SetHomeChat(ctx context.Context, userID int64, chatID int64, chatType string) error
	TryTouchHomeChatLog(ctx context.Context, userID int64, now time.Time, minInterval time.Duration) (bool, error)
}

type CatRepository interface {
	GetByUserID(ctx context.Context, userID int64) (*domain.Cat, error)
	Create(ctx context.Context, userID int64, name string, breed domain.Breed, trait domain.Trait, hp, atk, def, spd int) (*domain.Cat, error)
	DeleteByUserID(ctx context.Context, userID int64) error
	Train(ctx context.Context, userID int64, now time.Time) (*domain.Cat, domain.TrainResult, error)
	SetName(ctx context.Context, userID int64, name string) (*domain.Cat, error)
}

type DailyRepository interface {
	GetState(ctx context.Context, userID int64) (domain.DailyState, error)
	Claim(ctx context.Context, userID int64, now time.Time) (*domain.Cat, domain.DailyClaimResult, error)
}

type ArenaRepository interface {
	GetState(ctx context.Context, userID int64, now time.Time) (domain.ArenaState, error)
	SaveState(ctx context.Context, userID int64, st domain.ArenaState) error

	// 3 цели на выбор
	FindOpponents(ctx context.Context, userID int64, attackerPower int, scopeChatID int64, scopeChatType string, seed string) ([]domain.ArenaOpponent, error)

	// атомарный бой: списать билет, пересчитать рейтинг, записать матч
	Fight(ctx context.Context, userID int64, opponentUserID int64, now time.Time, seed string) (domain.ArenaState, domain.ArenaFightResult, int64, int, int, int, int, int, error)
	Reroll(ctx context.Context, userID int64, now time.Time, pay bool, energyCost int) (domain.ArenaState, int, error)

}
