package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

type fakeClock struct{ t time.Time }

func (f fakeClock) Now() time.Time { return f.t }

type fakeEvents struct {
	events []GameEvent
}

func (l *fakeEvents) Publish(_ context.Context, event GameEvent) error {
	l.events = append(l.events, event)
	return nil
}

type stubCats struct {
	cat *domain.Cat
	err error
}

func (s stubCats) GetByUserID(context.Context, int64) (*domain.Cat, error) { return s.cat, s.err }
func (s stubCats) Create(context.Context, int64, string, domain.Breed, domain.Trait, int, int, int, int) (*domain.Cat, error) {
	return s.cat, s.err
}
func (s stubCats) DeleteByUserID(context.Context, int64) error                 { return nil }
func (s stubCats) SetName(context.Context, int64, string) (*domain.Cat, error) { return nil, nil }
func (s stubCats) SaveProgress(context.Context, int64, int64, domain.Cat) (bool, error) {
	return true, s.err
}

type stubEngine struct{ result domain.TrainResult }

func (s stubEngine) Train(_ context.Context, input gameengine.TrainingInput) (gameengine.TrainingOutput, error) {
	return gameengine.TrainingOutput{Cat: input.Cat, Result: s.result}, nil
}

func (stubEngine) Expedition(_ context.Context, input gameengine.ExpeditionInput) (gameengine.ExpeditionOutput, error) {
	return gameengine.ExpeditionOutput{Cat: input.Cat}, nil
}

func (stubEngine) ResolveYardEvent(context.Context, gameengine.YardEventInput) (domain.YardEventResult, error) {
	return domain.YardEventResult{}, nil
}

func (stubEngine) Fight(context.Context, gameengine.FightInput) (domain.FightResult, error) {
	return domain.FightResult{}, nil
}

type zeroRNG struct{}

func (zeroRNG) Intn(int) int { return 0 }

type stubUsers struct {
}

func (s stubUsers) EnsureUser(context.Context, int64) (int64, error)        { return 1, nil }
func (s stubUsers) GetPendingAction(context.Context, int64) (string, error) { return "", nil }
func (s stubUsers) SetPendingAction(context.Context, int64, string) error   { return nil }
func (s stubUsers) GetPendingInput(context.Context, int64, int64) (*PendingInput, error) {
	return nil, nil
}
func (s stubUsers) SavePendingInput(context.Context, PendingInput) error  { return nil }
func (s stubUsers) ClearPendingInput(context.Context, int64, int64) error { return nil }
func (s stubUsers) ClaimDailyCommand(context.Context, int64, int64, string, time.Time) (bool, error) {
	return true, nil
}

func TestTrainingServicePublishesLogInSourceGroup(t *testing.T) {
	t.Parallel()

	now := time.Unix(123, 0)
	events := &fakeEvents{}
	cats := stubCats{cat: &domain.Cat{Name: "Тест", Breed: domain.BreedBengal, Level: 2, Energy: 80, EnergyUpdatedAt: now}}
	engine := stubEngine{result: domain.TrainResult{Outcome: domain.TrainingOK, XPGain: 10, EnergyCost: 20, EffPercent: 100}}
	yards := stubYards{yard: &domain.Yard{ID: 9, TelegramChatID: -100}}
	svc := NewTrainingService(cats, stubUsers{}, nil, nil, yards, engine, nil, fakeClock{t: now}, zeroRNG{}, events)

	_, _, _, err := svc.Train(context.Background(), 1, 777, -100, "group")
	if err != nil {
		t.Fatalf("Train() error = %v", err)
	}
	if len(events.events) != 1 {
		t.Fatalf("events = %d, want 1", len(events.events))
	}
	payload, ok := events.events[0].Payload.(CatTrainedPayload)
	if !ok || payload.TargetChatID != -100 || payload.CatName != "Тест" {
		t.Fatalf("unexpected event: %+v", events.events[0])
	}
	if events.events[0].YardID != 9 {
		t.Fatalf("yard id = %d, want 9", events.events[0].YardID)
	}
}

func TestTrainingServiceTracksCompletedHunt(t *testing.T) {
	t.Parallel()

	now := time.Unix(123, 0)
	events := &fakeEvents{}
	cats := stubCats{cat: &domain.Cat{Name: "Тест", Level: 1, Energy: 100, EnergyUpdatedAt: now}}
	svc := NewTrainingService(cats, stubUsers{}, nil, nil, nil, stubEngine{}, nil, fakeClock{t: now}, zeroRNG{}, events)

	if _, _, _, err := svc.Train(context.Background(), 9, 777, 10, "private"); err != nil {
		t.Fatalf("Train() error = %v", err)
	}
	if len(events.events) != 1 || events.events[0].Kind != GameEventCatTrained || events.events[0].UserID != 9 {
		t.Fatalf("unexpected game events: %+v", events.events)
	}
}

func TestTrainingServiceKeepsPrivateActionPrivateEvenWithLegacyHomeChat(t *testing.T) {
	t.Parallel()

	now := time.Unix(123, 0)
	events := &fakeEvents{}
	cats := stubCats{cat: &domain.Cat{Name: "Тест", Level: 1, Energy: 50, EnergyUpdatedAt: now}}
	engine := stubEngine{result: domain.TrainResult{Outcome: domain.TrainingOK}}
	svc := NewTrainingService(cats, stubUsers{}, nil, nil, nil, engine, nil, fakeClock{t: now}, zeroRNG{}, events)

	_, _, _, err := svc.Train(context.Background(), 1, 777, 10, "private")
	if err != nil {
		t.Fatalf("Train() error = %v", err)
	}
	if len(events.events) != 1 {
		t.Fatalf("unexpected event count: %d", len(events.events))
	}
	payload := events.events[0].Payload.(CatTrainedPayload)
	if payload.TargetChatID != 0 {
		t.Fatalf("private training leaked into legacy home chat: %+v", events.events)
	}
}

func TestTrainingServiceSkipsPrivateLogWithoutHomeChat(t *testing.T) {
	t.Parallel()

	now := time.Unix(123, 0)
	events := &fakeEvents{}
	cats := stubCats{cat: &domain.Cat{Name: "Тест", Level: 1, Energy: 50, EnergyUpdatedAt: now}}
	engine := stubEngine{result: domain.TrainResult{Outcome: domain.TrainingOK}}
	svc := NewTrainingService(cats, stubUsers{}, nil, nil, nil, engine, nil, fakeClock{t: now}, zeroRNG{}, events)

	_, _, _, err := svc.Train(context.Background(), 1, 777, 10, "private")
	if err != nil {
		t.Fatalf("Train() error = %v", err)
	}
	if len(events.events) != 1 {
		t.Fatalf("events = %d, want 1 durable event", len(events.events))
	}
	if payload := events.events[0].Payload.(CatTrainedPayload); payload.TargetChatID != 0 {
		t.Fatalf("target chat = %d, want 0", payload.TargetChatID)
	}
	if events.events[0].YardID != 0 {
		t.Fatalf("private training yard id = %d, want 0", events.events[0].YardID)
	}
}

type retryCats struct {
	stubCats
	versions   []int64
	saveResult []bool
	getCalls   int
	saveCalls  int
}

func (r *retryCats) GetByUserID(context.Context, int64) (*domain.Cat, error) {
	cat := *r.cat
	cat.StateVersion = r.versions[r.getCalls]
	r.getCalls++
	return &cat, nil
}

func (r *retryCats) SaveProgress(context.Context, int64, int64, domain.Cat) (bool, error) {
	result := r.saveResult[r.saveCalls]
	r.saveCalls++
	return result, nil
}

func TestTrainingServiceRetriesOptimisticConflict(t *testing.T) {
	t.Parallel()

	now := time.Unix(123, 0)
	cats := &retryCats{
		stubCats:   stubCats{cat: &domain.Cat{Name: "Тест", Level: 1, Energy: 100, EnergyUpdatedAt: now}},
		versions:   []int64{7, 8},
		saveResult: []bool{false, true},
	}
	svc := NewTrainingService(cats, stubUsers{}, nil, nil, nil, stubEngine{}, nil, fakeClock{t: now}, zeroRNG{}, &fakeEvents{})

	cat, _, _, err := svc.Train(context.Background(), 1, 777, 10, "private")
	if err != nil {
		t.Fatalf("Train() error = %v", err)
	}
	if cats.getCalls != 2 || cats.saveCalls != 2 {
		t.Fatalf("calls: get=%d save=%d, want 2/2", cats.getCalls, cats.saveCalls)
	}
	if cat.StateVersion != 9 {
		t.Fatalf("state version = %d, want 9", cat.StateVersion)
	}
}

func TestTrainingServiceStopsAfterRepeatedOptimisticConflicts(t *testing.T) {
	t.Parallel()

	now := time.Unix(123, 0)
	cats := &retryCats{
		stubCats:   stubCats{cat: &domain.Cat{Name: "Тест", Level: 1, Energy: 100, EnergyUpdatedAt: now}},
		versions:   []int64{7, 8, 9},
		saveResult: []bool{false, false, false},
	}
	svc := NewTrainingService(cats, stubUsers{}, nil, nil, nil, stubEngine{}, nil, fakeClock{t: now}, zeroRNG{}, &fakeEvents{})

	_, _, _, err := svc.Train(context.Background(), 1, 777, 10, "private")
	if !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("Train() error = %v, want ErrConcurrentUpdate", err)
	}
}

func TestTrainingServiceGeneratesRivalAwareNarrative(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	cat := &domain.Cat{
		ID: 42, UserID: 7, Name: "Барсик", Breed: domain.BreedBengal,
		Trait: domain.TraitBully, Level: 3, Energy: 100, EnergyUpdatedAt: now,
	}
	result := domain.TrainResult{
		Outcome: domain.TrainingOK, Encounter: domain.EncounterPigeon,
		XPGain: 140, CoinsGain: 9, EnergyCost: 90, EffPercent: 100,
	}
	personalities := stubPersonalities{personality: &domain.CatPersonality{
		CatID: 42, Trait: domain.TraitBully, SpeechStyle: domain.DefaultSpeechStyle(domain.TraitBully), HumorMode: domain.HumorBold,
	}}
	quota := &stubAIQuota{allowed: true}
	yards := stubYards{
		yard: &domain.Yard{ID: 9, TelegramChatID: -100},
		relationships: []domain.CatRelationship{
			{CatAID: 42, CatBID: 50, CatAName: "Барсик", CatBName: "Батон", Rivalry: 17},
			{CatAID: 42, CatBID: 49, CatAName: "Барсик", CatBName: "Сметана", Rivalry: 4},
		},
	}
	events := &fakeEvents{}
	service := NewTrainingService(
		stubCats{cat: cat}, stubUsers{}, personalities, quota, yards,
		stubEngine{result: result}, ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second),
		fakeClock{t: now}, zeroRNG{}, events,
	)

	_, _, generation, err := service.Train(context.Background(), 7, 777, -100, "group")
	if err != nil {
		t.Fatalf("Train() error = %v", err)
	}
	if !strings.Contains(generation.Text, "Батон") || quota.calls != 1 {
		t.Fatalf("generation/quota = %+v/%d", generation, quota.calls)
	}
	payload := events.events[0].Payload.(CatTrainedPayload)
	if payload.RivalName != "Батон" || payload.Rivalry != 17 || payload.Narrative != generation.Text || !payload.Fallback {
		t.Fatalf("training payload = %+v", payload)
	}
}

func TestTrainingServiceKeepsRoutineTrainingProcedural(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	cat := &domain.Cat{
		ID: 42, UserID: 7, Name: "Барсик", Breed: domain.BreedBengal,
		Trait: domain.TraitBully, Level: 3, Energy: 100, EnergyUpdatedAt: now,
	}
	result := domain.TrainResult{
		Outcome: domain.TrainingOK, Encounter: domain.EncounterPigeon,
		XPGain: 100, EnergyCost: 90, EffPercent: 100,
	}
	quota := &stubAIQuota{allowed: true}
	events := &fakeEvents{}
	service := NewTrainingService(
		stubCats{cat: cat}, stubUsers{}, stubPersonalities{}, quota, nil,
		stubEngine{result: result}, ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second),
		fakeClock{t: now}, zeroRNG{}, events,
	)

	_, _, generation, err := service.Train(context.Background(), 7, 777, 10, "private")
	if err != nil {
		t.Fatalf("Train() error = %v", err)
	}
	if generation.Text != "" || quota.calls != 0 {
		t.Fatalf("routine training used AI: generation=%+v quota_calls=%d", generation, quota.calls)
	}
	payload := events.events[0].Payload.(CatTrainedPayload)
	if payload.Narrative != "" || payload.Provider != "" {
		t.Fatalf("routine training stored AI narrative: %+v", payload)
	}
}
