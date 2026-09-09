package app

import (
	"context"
	"strconv"

	"catforge/internal/ai"
	"catforge/internal/gameengine"
)

type App struct {
	Payments *PaymentService
	Users    UserRepository
	Clock    Clock
	RNG      RNG
	Events   GameEventSink
	Updates  UpdateRepository

	Starter       *StarterService
	Profile       *ProfileService
	Personality   *PersonalityService
	CatReply      *CatReplyService
	CatFollowup   *CatFollowupService
	Training      *TrainingService
	Expedition    *ExpeditionService
	Collection    *CollectionService
	Bestiary      *BestiaryService
	Yard          *YardService
	Fight         *FightService
	CatBanter     *CatBanterService
	YardEvent     *YardEventService
	WeeklySummary *WeeklySummaryService
}

func New(users UserRepository, cats CatRepository, personalities PersonalityRepository, quota AIQuotaRepository, refs CatMessageReferenceRepository, yards YardRepository, yardEvents YardEventRepository, fights FightRepository, items ItemRepository, updates UpdateRepository, engine gameengine.Engine, voice ai.Generator, clock Clock, rng RNG, events GameEventSink) *App {
	a := &App{
		Users:   users,
		Clock:   clock,
		RNG:     rng,
		Events:  events,
		Updates: updates,
	}
	a.Starter = NewStarterService(cats, clock, rng, events)
	a.Profile = NewProfileService(cats, fights)
	a.Personality = NewPersonalityService(personalities, voice, events, clock)
	a.CatReply = NewCatReplyService(cats, personalities, quota, yards, voice, clock, events)
	a.CatFollowup = NewCatFollowupService(refs, yards, personalities, quota, voice, clock, events)
	a.Training = NewTrainingService(cats, users, personalities, quota, yards, engine, voice, clock, rng, events)
	a.Expedition = NewExpeditionService(cats, items, engine, clock, rng, events)
	a.Collection = NewCollectionService(items)
	a.Bestiary = NewBestiaryService(items)
	a.Yard = NewYardService(yards, cats, clock, events)
	a.Fight = NewFightService(cats, personalities, yards, fights, engine, voice, clock, rng, events)
	var idleYards IdleBanterYardRepository
	if repository, ok := yards.(IdleBanterYardRepository); ok {
		idleYards = repository
	}
	a.CatBanter = NewCatBanterService(yards, idleYards, personalities, voice, clock, events)
	a.YardEvent = NewYardEventService(yards, yardEvents, engine, voice, clock, rng, events)
	a.YardEvent.SetBanterService(a.CatBanter)
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
