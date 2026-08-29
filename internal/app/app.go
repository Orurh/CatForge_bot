package app

import (
	"context"
	"strconv"

	"catforge/internal/ai"
	"catforge/internal/gameengine"
)

type App struct {
	Users   UserRepository
	Clock   Clock
	RNG     RNG
	Events  GameEventSink
	Updates UpdateRepository

	Starter       *StarterService
	Profile       *ProfileService
	Personality   *PersonalityService
	CatReply      *CatReplyService
	Training      *TrainingService
	Expedition    *ExpeditionService
	Collection    *CollectionService
	Bestiary      *BestiaryService
	Yard          *YardService
	Fight         *FightService
	YardEvent     *YardEventService
	WeeklySummary *WeeklySummaryService
}

func New(users UserRepository, cats CatRepository, personalities PersonalityRepository, quota AIQuotaRepository, yards YardRepository, yardEvents YardEventRepository, fights FightRepository, items ItemRepository, updates UpdateRepository, engine gameengine.Engine, voice ai.Generator, clock Clock, rng RNG, events GameEventSink) *App {
	a := &App{
		Users:   users,
		Clock:   clock,
		RNG:     rng,
		Events:  events,
		Updates: updates,
	}
	a.Starter = NewStarterService(cats, clock, rng, events)
	a.Profile = NewProfileService(cats)
	a.Personality = NewPersonalityService(personalities, voice, events, clock)
	a.CatReply = NewCatReplyService(cats, personalities, quota, voice, clock, events)
	a.Training = NewTrainingService(cats, users, personalities, quota, yards, engine, voice, clock, rng, events)
	a.Expedition = NewExpeditionService(cats, items, engine, clock, rng, events)
	a.Collection = NewCollectionService(items)
	a.Bestiary = NewBestiaryService(items)
	a.Yard = NewYardService(yards, cats, clock, events)
	a.Fight = NewFightService(cats, yards, fights, engine, clock, rng, events)
	a.YardEvent = NewYardEventService(yards, yardEvents, engine, voice, clock, rng, events)
	a.WeeklySummary = NewWeeklySummaryService(yards, yardEvents, quota, voice, clock, events)
	return a
}

func (a *App) EnsureUser(ctx context.Context, telegramID int64) (int64, error) {
	return a.Users.EnsureUser(ctx, telegramID)
}

func (a *App) RecordUserStarted(ctx context.Context, userID, telegramID int64, chatType string) {
	if a.Events == nil {
		return
	}
	_ = a.Events.Publish(ctx, GameEvent{
		DedupeKey:      "user:" + strconv.FormatInt(userID, 10) + ":started",
		Kind:           GameEventUserStarted,
		UserID:         userID,
		OccurredAt:     a.Clock.Now(),
		PayloadVersion: GameEventPayloadVersion,
		Payload:        UserStartedPayload{TelegramID: telegramID, ChatType: chatType},
	})
}
