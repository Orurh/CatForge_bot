package app

import (
	"context"
	"testing"
	"time"

	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

type fightCats map[int64]*domain.Cat

func (f fightCats) GetByUserID(_ context.Context, userID int64) (*domain.Cat, error) {
	cat, ok := f[userID]
	if !ok {
		return nil, domain.ErrNoCat
	}
	return cat, nil
}
func (fightCats) Create(context.Context, int64, string, domain.Breed, domain.Trait, int, int, int, int) (*domain.Cat, error) {
	return nil, nil
}
func (fightCats) DeleteByUserID(context.Context, int64) error { return nil }
func (fightCats) SaveProgress(context.Context, int64, int64, domain.Cat) (bool, error) {
	return true, nil
}
func (fightCats) SetName(context.Context, int64, string) (*domain.Cat, error) { return nil, nil }

type fakeFightRepo struct {
	toggle      domain.FightQueueToggle
	toggleNow   time.Time
	toggleUntil time.Time
	saved       bool
	record      domain.FightRecord
	result      domain.FightResult
	catAName    string
	catBName    string
}

func (f *fakeFightRepo) ToggleQueue(_ context.Context, _, _, _ int64, now, expiresAt time.Time) (domain.FightQueueToggle, error) {
	f.toggleNow, f.toggleUntil = now, expiresAt
	return f.toggle, nil
}
func (f *fakeFightRepo) SaveFight(_ context.Context, record domain.FightRecord, result domain.FightResult, catAName, catBName string) (int64, int, error) {
	f.saved, f.record, f.result, f.catAName, f.catBName = true, record, result, catAName, catBName
	return 55, 9, nil
}

type fightEngine struct {
	input  gameengine.FightInput
	result domain.FightResult
	calls  int
}

func (e *fightEngine) Train(_ context.Context, input gameengine.TrainingInput) (gameengine.TrainingOutput, error) {
	return gameengine.TrainingOutput{Cat: input.Cat}, nil
}
func (e *fightEngine) Expedition(_ context.Context, input gameengine.ExpeditionInput) (gameengine.ExpeditionOutput, error) {
	return gameengine.ExpeditionOutput{Cat: input.Cat}, nil
}
func (e *fightEngine) ResolveYardEvent(context.Context, gameengine.YardEventInput) (domain.YardEventResult, error) {
	return domain.YardEventResult{}, nil
}
func (e *fightEngine) Fight(_ context.Context, input gameengine.FightInput) (domain.FightResult, error) {
	e.calls++
	e.input = input
	return e.result, nil
}

type sequenceRNG struct {
	values []int
	index  int
}

func (s *sequenceRNG) Intn(int) int {
	value := s.values[s.index]
	s.index++
	return value
}

func TestFightServiceWaitsAndCancelsWithoutRunningEngine(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	cat := &domain.Cat{ID: 10, UserID: 1, Name: "Барсик"}
	for _, status := range []domain.FightQueueStatus{domain.FightQueueWaiting, domain.FightQueueCanceled} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()
			repo := &fakeFightRepo{toggle: domain.FightQueueToggle{Status: status}}
			engine := &fightEngine{}
			svc := NewFightService(fightCats{1: cat}, stubYards{yard: &domain.Yard{ID: 7}}, repo, engine, fakeClock{t: now}, zeroRNG{}, nil)

			outcome, err := svc.Toggle(context.Background(), -100, 1)
			if err != nil {
				t.Fatalf("Toggle() error = %v", err)
			}
			if outcome.Status != status || outcome.Cat != cat || engine.calls != 0 || repo.saved {
				t.Fatalf("unexpected outcome: %+v, engine calls=%d saved=%v", outcome, engine.calls, repo.saved)
			}
			if !repo.toggleNow.Equal(now) || !repo.toggleUntil.Equal(now.Add(FightQueueTTL)) {
				t.Fatalf("queue interval = %v..%v", repo.toggleNow, repo.toggleUntil)
			}
		})
	}
}

func TestFightServiceMatchesWaitingOpponentAndPersistsResult(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	waiter := &domain.Cat{ID: 10, UserID: 1, Name: "Барсик", Level: 3, HPBase: 50, ATKBase: 20, DEFBase: 15, SPDBase: 18}
	newcomer := &domain.Cat{ID: 20, UserID: 2, Name: "Батон", Level: 2, HPBase: 40, ATKBase: 24, DEFBase: 12, SPDBase: 22}
	result := domain.FightResult{
		WinnerCatID: 20, LoserCatID: 10, Rounds: 4, FinalHPA: 0, FinalHPB: 4,
		Turns: []domain.FightTurn{{Round: 4, AttackerCatID: 20, DefenderCatID: 10, Damage: 8}},
	}
	repo := &fakeFightRepo{toggle: domain.FightQueueToggle{
		Status: domain.FightQueueMatched, Opponent: domain.FightQueueEntry{UserID: 1, CatID: 10},
	}}
	engine := &fightEngine{result: result}
	events := &fakeEvents{}
	rng := &sequenceRNG{values: []int{3, 4}}
	svc := NewFightService(fightCats{1: waiter, 2: newcomer}, stubYards{yard: &domain.Yard{ID: 7}}, repo, engine, fakeClock{t: now}, rng, events)

	outcome, err := svc.Toggle(context.Background(), -100, 2)
	if err != nil {
		t.Fatalf("Toggle() error = %v", err)
	}
	wantSeed := uint64(3)<<30 | 4
	if engine.calls != 1 || engine.input.CatA.ID != waiter.ID || engine.input.CatB.ID != newcomer.ID || engine.input.Seed != wantSeed {
		t.Fatalf("unexpected engine input: %+v", engine.input)
	}
	if !repo.saved || repo.record.RivalryDelta != 2 || repo.catAName != waiter.Name || repo.catBName != newcomer.Name {
		t.Fatalf("fight was not persisted correctly: %+v", repo)
	}
	if outcome.FightID != 55 || outcome.Rivalry != 9 || outcome.Opponent != waiter || outcome.Seed != wantSeed {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
	if len(events.events) != 1 || events.events[0].Kind != GameEventFightFinished || events.events[0].YardID != 7 {
		t.Fatalf("unexpected events: %+v", events.events)
	}
}
