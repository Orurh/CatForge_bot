package app

import (
	"context"
)

type App struct {
	Users    UserRepository
	Clock    Clock
	RNG      RNG
	EventLog EventLog

	Starter  *StarterService
	Profile  *ProfileService
	Training *TrainingService
}

func New(users UserRepository, cats CatRepository, clock Clock, rng RNG, ev EventLog) *App {
	a := &App{
		Users:    users,
		Clock:    clock,
		RNG:      rng,
		EventLog: ev,
	}
	a.Starter = NewStarterService(cats, rng)
	a.Profile = NewProfileService(cats)
	a.Training = NewTrainingService(cats, users, clock, ev)
	return a
}

func (a *App) EnsureUser(ctx context.Context, telegramID int64) (int64, error) {
	return a.Users.EnsureUser(ctx, telegramID)
}
