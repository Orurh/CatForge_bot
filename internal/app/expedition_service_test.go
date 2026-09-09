package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

type expeditionEngine struct {
	calls int
	loot  domain.LootRoll
	input gameengine.ExpeditionInput
}

func (e *expeditionEngine) Train(_ context.Context, input gameengine.TrainingInput) (gameengine.TrainingOutput, error) {
	return gameengine.TrainingOutput{Cat: input.Cat}, nil
}

func (e *expeditionEngine) Expedition(_ context.Context, input gameengine.ExpeditionInput) (gameengine.ExpeditionOutput, error) {
	e.calls++
	e.input = input
	cat := input.Cat
	cat.Energy -= 20
	return gameengine.ExpeditionOutput{Cat: cat, Result: domain.ExpeditionResult{Outcome: domain.ExpeditionVictory, EnergyCost: 20, Loot: e.loot}}, nil
}

func (e *expeditionEngine) ResolveYardEvent(context.Context, gameengine.YardEventInput) (domain.YardEventResult, error) {
	return domain.YardEventResult{}, nil
}

func (e *expeditionEngine) Fight(context.Context, gameengine.FightInput) (domain.FightResult, error) {
	return domain.FightResult{}, nil
}

func TestExpeditionServiceResolvesAndPersistsLoot(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	cats := &retryCats{stubCats: stubCats{cat: &domain.Cat{Level: 1, Energy: 100, EnergyUpdatedAt: now}}, versions: []int64{4}}
	engine := &expeditionEngine{loot: domain.LootRoll{Dropped: true, Rarity: domain.ItemCommon, ItemIndex: 0}}
	items := &fakeItems{saveItem: domain.OwnedItem{ItemID: "string_collar", Level: 1}, saveIsNew: true}
	svc := NewExpeditionService(cats, items, engine, fakeClock{t: now}, zeroRNG{}, nil)

	_, result, err := svc.Explore(context.Background(), 7, domain.ExpeditionAlley, domain.ExpeditionEasy)
	if err != nil {
		t.Fatalf("Explore() error = %v", err)
	}
	if items.savedItemID != "string_collar" || result.Loot.ItemID != "string_collar" || !result.Loot.New {
		t.Fatalf("loot was not resolved/persisted: saved=%q result=%+v", items.savedItemID, result.Loot)
	}
	if !engine.input.ForceLoot || engine.input.LootCounts.Common == 0 {
		t.Fatalf("first expedition must force catalog-backed loot: %+v", engine.input)
	}
}

func TestExpeditionServiceRetriesWithSameAction(t *testing.T) {
	t.Parallel()

	now := time.Unix(123, 0)
	cats := &retryCats{
		stubCats:   stubCats{cat: &domain.Cat{Level: 1, Energy: 100, EnergyUpdatedAt: now}},
		versions:   []int64{4, 5},
		saveResult: []bool{false, true},
	}
	engine := &expeditionEngine{}
	events := &fakeEvents{}
	items := &fakeItems{saveResults: []bool{false, true}}
	svc := NewExpeditionService(cats, items, engine, fakeClock{t: now}, zeroRNG{}, events)

	cat, result, err := svc.Explore(context.Background(), 7, domain.ExpeditionAlley, domain.ExpeditionEasy)
	if err != nil {
		t.Fatalf("Explore() error = %v", err)
	}
	if engine.calls != 2 || cats.getCalls != 2 || items.saveCalls != 2 {
		t.Fatalf("calls: engine=%d get=%d save=%d, want 2/2/2", engine.calls, cats.getCalls, items.saveCalls)
	}
	if cat.StateVersion != 6 || cat.Energy != 80 || result.Outcome != domain.ExpeditionVictory {
		t.Fatalf("unexpected result: cat=%+v result=%+v", cat, result)
	}
	if len(events.events) != 2 || events.events[0].Kind != GameEventExpeditionStarted || events.events[1].Kind != GameEventExpeditionFinished {
		t.Fatalf("unexpected game events: %+v", events.events)
	}
}

func TestExpeditionServiceStopsAfterConflicts(t *testing.T) {
	t.Parallel()

	now := time.Unix(123, 0)
	cats := &retryCats{
		stubCats:   stubCats{cat: &domain.Cat{Level: 1, Energy: 100, EnergyUpdatedAt: now}},
		versions:   []int64{4, 5, 6},
		saveResult: []bool{false, false, false},
	}
	items := &fakeItems{saveResults: []bool{false, false, false}}
	svc := NewExpeditionService(cats, items, &expeditionEngine{}, fakeClock{t: now}, zeroRNG{}, nil)

	_, _, err := svc.Explore(context.Background(), 7, domain.ExpeditionAlley, domain.ExpeditionEasy)
	if !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("Explore() error = %v, want ErrConcurrentUpdate", err)
	}
}
