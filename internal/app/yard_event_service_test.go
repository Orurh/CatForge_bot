package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

type stubYardEvents struct {
	event         *domain.YardEvent
	created       bool
	choice        domain.YardEventChoice
	firstChoice   bool
	counts        map[domain.YardEventChoiceID]int
	err           error
	resolution    domain.YardEventResolutionInput
	weekly        domain.YardWeeklySummary
	candidates    []domain.Yard
	saved         bool
	nextType      domain.YardEventType
	wantStartType domain.YardEventType
}

func (s stubYardEvents) NextEventType(context.Context, int64) (domain.YardEventType, error) {
	if s.nextType == "" {
		return domain.YardEventFishTruck, s.err
	}
	return s.nextType, s.err
}
func (s stubYardEvents) StartOrGet(_ context.Context, _ int64, eventType domain.YardEventType, _ int64, _ time.Time, _ time.Time, _ uint32) (*domain.YardEvent, bool, error) {
	if s.wantStartType != "" && eventType != s.wantStartType {
		return nil, false, errors.New("unexpected yard event type")
	}
	return s.event, s.created, s.err
}
func (s stubYardEvents) GetCurrentActive(context.Context, int64) (*domain.YardEvent, error) {
	return s.event, s.err
}
func (s stubYardEvents) GetActive(context.Context, int64, int64) (*domain.YardEvent, error) {
	return s.event, s.err
}
func (s stubYardEvents) ListStartCandidates(context.Context, time.Time, time.Time, int) ([]domain.Yard, error) {
	return s.candidates, s.err
}
func (s stubYardEvents) SubmitChoice(context.Context, int64, int64, int64, domain.YardEventChoiceID, time.Time) (domain.YardEventChoice, bool, error) {
	return s.choice, s.firstChoice, s.err
}
func (s stubYardEvents) ChoiceCounts(context.Context, int64) (map[domain.YardEventChoiceID]int, error) {
	return s.counts, s.err
}
func (s stubYardEvents) DueEventIDs(context.Context, time.Time, int) ([]int64, error) {
	return nil, s.err
}
func (s stubYardEvents) GetResolutionInput(context.Context, int64) (domain.YardEventResolutionInput, error) {
	return s.resolution, s.err
}
func (s stubYardEvents) SaveResolution(context.Context, int64, domain.YardEventResult, uint32, uint32, time.Time) (bool, error) {
	return s.saved, s.err
}
func (s stubYardEvents) WeeklySummary(context.Context, int64, time.Time, time.Time) (domain.YardWeeklySummary, error) {
	return s.weekly, s.err
}

type resolvingEngine struct{ result domain.YardEventResult }

func (e resolvingEngine) Train(_ context.Context, input gameengine.TrainingInput) (gameengine.TrainingOutput, error) {
	return gameengine.TrainingOutput{Cat: input.Cat}, nil
}
func (e resolvingEngine) Expedition(_ context.Context, input gameengine.ExpeditionInput) (gameengine.ExpeditionOutput, error) {
	return gameengine.ExpeditionOutput{Cat: input.Cat}, nil
}
func (e resolvingEngine) ResolveYardEvent(context.Context, gameengine.YardEventInput) (domain.YardEventResult, error) {
	return e.result, nil
}
func (e resolvingEngine) Fight(context.Context, gameengine.FightInput) (domain.FightResult, error) {
	return domain.FightResult{}, nil
}

func TestYardEventServiceStartsSingleStructuredEvent(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	yard := &domain.Yard{ID: 9, TelegramChatID: -100, Name: "Друзья"}
	event := &domain.YardEvent{ID: 11, YardID: 9, Type: domain.YardEventFishTruck, State: domain.YardEventActive, Seed: 77, ResolvesAt: now.Add(yardEventDuration)}
	counts := map[domain.YardEventChoiceID]int{}
	gameEvents := &fakeEvents{}
	svc := NewYardEventService(stubYards{yard: yard}, stubYardEvents{event: event, created: true, counts: counts}, stubEngine{}, nil, fakeClock{t: now}, zeroRNG{}, gameEvents)

	status, err := svc.StartOrGet(context.Background(), -100)
	if err != nil {
		t.Fatalf("StartOrGet() error = %v", err)
	}
	if !status.Created || status.Event.ID != 11 || len(gameEvents.events) != 1 {
		t.Fatalf("unexpected status/events: %+v/%+v", status, gameEvents.events)
	}
	if gameEvents.events[0].Kind != GameEventYardEventStarted || gameEvents.events[0].YardID != 9 {
		t.Fatalf("unexpected game event: %+v", gameEvents.events[0])
	}
}

func TestYardEventServiceStartsRepositoryRotationType(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	yard := &domain.Yard{ID: 9, TelegramChatID: -100, Name: "Друзья"}
	event := &domain.YardEvent{ID: 12, YardID: 9, Type: domain.YardEventBigDog, State: domain.YardEventActive, Seed: 78, ResolvesAt: now.Add(yardEventDuration)}
	repo := stubYardEvents{
		event: event, created: true, counts: map[domain.YardEventChoiceID]int{},
		nextType: domain.YardEventBigDog, wantStartType: domain.YardEventBigDog,
	}
	svc := NewYardEventService(stubYards{yard: yard}, repo, stubEngine{}, nil, fakeClock{t: now}, zeroRNG{}, nil)

	status, err := svc.StartOrGet(context.Background(), -100)
	if err != nil || status.Event.Type != domain.YardEventBigDog {
		t.Fatalf("StartOrGet() = %+v, %v", status, err)
	}
}

func TestYardEventServiceStoresChoice(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	yard := &domain.Yard{ID: 9, TelegramChatID: -100}
	choice := domain.YardEventChoice{EventID: 11, CatID: 42, ChoiceID: domain.YardChoiceScout, SubmittedAt: now}
	counts := map[domain.YardEventChoiceID]int{domain.YardChoiceScout: 1}
	gameEvents := &fakeEvents{}
	event := &domain.YardEvent{ID: 11, YardID: 9, Type: domain.YardEventFishTruck, State: domain.YardEventActive, ResolvesAt: now.Add(time.Hour)}
	svc := NewYardEventService(stubYards{yard: yard}, stubYardEvents{event: event, choice: choice, firstChoice: true, counts: counts}, stubEngine{}, nil, fakeClock{t: now}, zeroRNG{}, gameEvents)

	status, err := svc.Choose(context.Background(), -100, 11, 7, domain.YardChoiceScout)
	if err != nil {
		t.Fatalf("Choose() error = %v", err)
	}
	if status.Counts[domain.YardChoiceScout] != 1 || len(gameEvents.events) != 1 {
		t.Fatalf("unexpected status/events: %+v/%+v", status, gameEvents.events)
	}
	if status.Event.ResolvesAt != event.ResolvesAt {
		t.Fatalf("choice status lost active event deadline: %+v", status.Event)
	}
	if event := gameEvents.events[0]; event.Kind != GameEventYardChoiceSubmitted || event.CatID != 42 || event.UserID != 7 {
		t.Fatalf("unexpected choice event: %+v", event)
	}
}

func TestYardEventServiceSchedulerStartsAndAnnouncesDueYard(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	yard := domain.Yard{ID: 9, TelegramChatID: -100, Name: "Друзья"}
	event := &domain.YardEvent{ID: 11, YardID: 9, Type: domain.YardEventFishTruck, State: domain.YardEventActive, Seed: 77, StartsAt: now, ResolvesAt: now.Add(yardEventDuration)}
	gameEvents := &fakeEvents{}
	svc := NewYardEventService(
		stubYards{}, stubYardEvents{event: event, created: true, candidates: []domain.Yard{yard}},
		stubEngine{}, nil, fakeClock{t: now}, zeroRNG{}, gameEvents,
	)

	started, err := svc.StartDue(context.Background(), 20)
	if err != nil || started != 1 || len(gameEvents.events) != 1 {
		t.Fatalf("StartDue() = %d, %v; events = %+v", started, err, gameEvents.events)
	}
	payload, ok := gameEvents.events[0].Payload.(YardEventStartedPayload)
	if !ok || payload.TelegramChatID != yard.TelegramChatID || payload.ResolvesAt != event.ResolvesAt {
		t.Fatalf("scheduled event announcement payload = %+v", gameEvents.events[0].Payload)
	}
}

func TestYardEventServiceRejectsUnknownChoice(t *testing.T) {
	t.Parallel()
	svc := NewYardEventService(nil, nil, stubEngine{}, nil, fakeClock{}, zeroRNG{}, nil)
	_, err := svc.Choose(context.Background(), -100, 11, 7, "sleep")
	if !errors.Is(err, domain.ErrInvalidYardChoice) {
		t.Fatalf("Choose() error = %v, want ErrInvalidYardChoice", err)
	}
}

func TestYardEventResolutionPublishesOptedInAutonomousCat(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	yard := &domain.Yard{ID: 9, TelegramChatID: -100, Name: "Друзья", HumorMode: domain.HumorNormal, AutoMessagesEnabled: true, MaxAutoMessagesDay: 1}
	input := domain.YardEventResolutionInput{
		Event:          domain.YardEvent{ID: 11, YardID: 9, Type: domain.YardEventFishTruck, Seed: 77, ContentVersion: gameengine.CurrentContentVersion},
		TelegramChatID: -100,
		Participants:   []domain.YardEventParticipant{{CatID: 42, CatName: "Барсик", Breed: domain.BreedBengal, Trait: domain.TraitBully, Level: 3, AutoSpeakEnabled: true}},
	}
	result := domain.YardEventResult{OutcomeTier: domain.YardOutcomeSuccess, YardScore: 12, XPGain: 30, TargetScore: 10, TeamScore: 15, Participants: []domain.YardEventParticipantResult{{CatID: 42, Contribution: 15, MVP: true}}}
	events := &fakeEvents{}
	svc := NewYardEventService(
		stubYards{yard: yard}, stubYardEvents{resolution: input, saved: true}, resolvingEngine{result: result},
		ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second), fakeClock{t: now}, zeroRNG{}, events,
	)

	saved, err := svc.Resolve(context.Background(), 11)
	if err != nil || !saved {
		t.Fatalf("Resolve() = %v, %v", saved, err)
	}
	if len(events.events) != 2 || events.events[1].Kind != GameEventAutonomousCatMessage {
		t.Fatalf("events = %+v", events.events)
	}
	resolvedPayload, ok := events.events[0].Payload.(YardEventResolvedPayload)
	if !ok || len(resolvedPayload.Participants) != 1 || resolvedPayload.Participants[0].CatName != "Барсик" || resolvedPayload.Participants[0].Level != 3 {
		t.Fatalf("resolved payload lost participant presentation data: %+v", events.events[0].Payload)
	}
	payload, ok := events.events[1].Payload.(AutonomousCatMessagePayload)
	if !ok || payload.CatName != "Барсик" || payload.Text == "" || !payload.Fallback {
		t.Fatalf("autonomous payload = %+v", events.events[1].Payload)
	}
}

func TestYardEventResolutionCanPublishSingleReactionAndBanter(t *testing.T) {
	t.Parallel()
	now := time.Unix(456, 0)
	claimed := []domain.AutoMessageKind{}
	yard := &domain.Yard{
		ID: 9, TelegramChatID: -100, HumorMode: domain.HumorNormal,
		AutoMessagesEnabled: true, MaxAutoMessagesDay: 2, CatToCatBanter: true,
	}
	input := domain.YardEventResolutionInput{
		Event:          domain.YardEvent{ID: 12, YardID: 9, Type: domain.YardEventBigDog, Seed: 78, ContentVersion: gameengine.CurrentContentVersion},
		TelegramChatID: -100,
		Participants: []domain.YardEventParticipant{
			{CatID: 42, CatName: "Барсик", Breed: domain.BreedBengal, Trait: domain.TraitBully, Level: 3, SpeechStyle: "задиристо", AutoSpeakEnabled: true, Choice: domain.YardChoiceSteal},
			{CatID: 50, CatName: "Батон", Breed: domain.BreedBritish, Trait: domain.TraitLazy, Level: 4, SpeechStyle: "лениво", AutoSpeakEnabled: true, Choice: domain.YardChoiceScout},
		},
	}
	result := domain.YardEventResult{
		OutcomeTier: domain.YardOutcomeExceptional, YardScore: 20, XPGain: 40, TargetScore: 10, TeamScore: 22,
		Participants: []domain.YardEventParticipantResult{
			{CatID: 42, Choice: domain.YardChoiceSteal, Contribution: 9},
			{CatID: 50, Choice: domain.YardChoiceScout, Contribution: 13, MVP: true},
		},
	}
	events := &fakeEvents{}
	yards := stubYards{yard: yard, claimedKinds: &claimed}
	voice := ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second)
	svc := NewYardEventService(yards, stubYardEvents{resolution: input, saved: true}, resolvingEngine{result: result}, voice, fakeClock{t: now}, zeroRNG{}, events)
	svc.SetBanterService(NewCatBanterService(yards, nil, nil, voice, fakeClock{t: now}, events))

	saved, err := svc.Resolve(context.Background(), 12)
	if err != nil || !saved {
		t.Fatalf("Resolve() = %v, %v", saved, err)
	}
	if len(events.events) != 3 || events.events[0].Kind != GameEventYardEventResolved || events.events[1].Kind != GameEventAutonomousCatMessage || events.events[2].Kind != GameEventCatBanter {
		t.Fatalf("events = %+v", events.events)
	}
	if len(claimed) != 2 || claimed[0] != domain.AutoMessageSingle || claimed[1] != domain.AutoMessageBanter {
		t.Fatalf("claimed kinds = %v", claimed)
	}
}
