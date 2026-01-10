package app

import (
	"context"
	"time"
)

type App struct {
	users    UserRepository
	clock    Clock
	rng      RNG
	eventLog EventLog

	Starter  *StarterService
	Profile  *ProfileService
	Training *TrainingService
	Daily    *DailyService
}

// type DailyRepository interface {
// 	GetState(ctx context.Context, userID int64) (domain.DailyState, error)
// 	Claim(ctx context.Context, userID int64, now time.Time) (*domain.Cat, domain.DailyClaimResult, error)
// }



func New(users UserRepository, cats CatRepository, daily DailyRepository, clock Clock, rng RNG, ev EventLog) *App {
	a := &App{
		users:    users,
		clock:    clock,
		rng:      rng,
		eventLog: ev,
	}
	a.Starter = NewStarterService(cats, rng)
	a.Profile = NewProfileService(cats)
	a.Training = NewTrainingService(cats, users, clock, ev)
	a.Daily = NewDailyService(daily, clock)
	return a
}

func (a *App) Now() time.Time { return a.clock.Now() }

func (a *App) GetPendingAction(ctx context.Context, userID int64) (string, error) {
	return a.users.GetPendingAction(ctx, userID)
}

func (a *App) SetPendingAction(ctx context.Context, userID int64, action string) error {
	return a.users.SetPendingAction(ctx, userID, action)
}

func (a *App) GetHomeChat(ctx context.Context, userID int64) (chatID int64, chatType string, err error) {
	return a.users.GetHomeChat(ctx, userID)
}

func (a *App) SetHomeChat(ctx context.Context, userID int64, chatID int64, chatType string) error {
	return a.users.SetHomeChat(ctx, userID, chatID, chatType)
}


func (a *App) EnsureUser(ctx context.Context, telegramID int64) (int64, error) {
	return a.users.EnsureUser(ctx, telegramID)
}
