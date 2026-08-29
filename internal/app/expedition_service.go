package app

import (
	"context"
	"fmt"

	"catforge/internal/domain"
	"catforge/internal/gamedata"
	"catforge/internal/gameengine"
)

type ExpeditionService struct {
	cats   CatRepository
	items  ItemRepository
	engine gameengine.Engine
	clock  Clock
	rng    RNG
	events GameEventSink
}

func NewExpeditionService(cats CatRepository, items ItemRepository, engine gameengine.Engine, clock Clock, rng RNG, events GameEventSink) *ExpeditionService {
	return &ExpeditionService{cats: cats, items: items, engine: engine, clock: clock, rng: rng, events: events}
}

func (s *ExpeditionService) Explore(ctx context.Context, userID int64, location domain.ExpeditionLocation, difficulty domain.ExpeditionDifficulty) (*domain.Cat, domain.ExpeditionResult, error) {
	now := s.clock.Now()
	seed := uint64(s.rng.Intn(1<<30))<<30 | uint64(s.rng.Intn(1<<30))
	common, rare, epic, err := gamedata.LootCounts(location)
	if err != nil {
		return nil, domain.ExpeditionResult{}, err
	}
	startPublished := false

	for attempt := 0; attempt < 3; attempt++ {
		current, err := s.cats.GetByUserID(ctx, userID)
		if err != nil {
			return nil, domain.ExpeditionResult{}, err
		}
		if s.events != nil && !startPublished {
			_ = s.events.Publish(ctx, GameEvent{
				DedupeKey: fmt.Sprintf("user:%d:expedition:%d:started", userID, seed),
				Kind:      GameEventExpeditionStarted, UserID: userID, CatID: current.ID, OccurredAt: now,
				RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion,
				PayloadVersion: GameEventPayloadVersion,
				Payload:        ExpeditionStartedPayload{Location: location, Difficulty: difficulty, Seed: seed},
			})
			startPublished = true
		}
		owned, err := s.items.ListOwned(ctx, userID)
		if err != nil {
			return nil, domain.ExpeditionResult{}, err
		}
		bonus := effectiveStatsFromOwned(owned)
		out, err := s.engine.Expedition(ctx, gameengine.ExpeditionInput{
			RulesVersion:   gameengine.CurrentRulesVersion,
			ContentVersion: gameengine.CurrentContentVersion,
			Cat:            *current,
			Now:            now,
			Seed:           seed,
			Location:       location,
			Difficulty:     difficulty,
			EquipmentBonus: bonus,
			LootCounts:     gameengine.LootCounts{Common: common, Rare: rare, Epic: epic},
			ForceLoot:      len(owned) == 0,
		})
		if err != nil {
			return nil, domain.ExpeditionResult{}, err
		}
		dropItemID := ""
		if out.Result.Loot.Dropped {
			candidates, catalogErr := gamedata.LootCandidates(location, out.Result.Loot.Rarity)
			if catalogErr != nil {
				return nil, domain.ExpeditionResult{}, catalogErr
			}
			if out.Result.Loot.ItemIndex < 0 || out.Result.Loot.ItemIndex >= len(candidates) {
				return nil, domain.ExpeditionResult{}, fmt.Errorf("loot index %d outside %s/%s catalog", out.Result.Loot.ItemIndex, location, out.Result.Loot.Rarity)
			}
			dropItemID = candidates[out.Result.Loot.ItemIndex].ID
		}
		save, err := s.items.SaveExpedition(ctx, userID, current.StateVersion, out.Cat, dropItemID, out.Result.Enemy.Kind, out.Result.Outcome == domain.ExpeditionVictory)
		if err != nil {
			return nil, domain.ExpeditionResult{}, err
		}
		if !save.Saved {
			continue
		}
		if dropItemID != "" {
			out.Result.Loot.ItemID = dropItemID
			out.Result.Loot.New = save.IsNew
			out.Result.Loot.Fragments = save.Item.Fragments
		}
		out.Cat.StateVersion = current.StateVersion + 1
		if s.events != nil {
			baseKey := fmt.Sprintf("user:%d:cat:%d:state:%d", userID, out.Cat.ID, out.Cat.StateVersion)
			_ = s.events.Publish(ctx, GameEvent{
				DedupeKey: baseKey + ":expedition-finished", Kind: GameEventExpeditionFinished,
				UserID: userID, CatID: out.Cat.ID, OccurredAt: now,
				RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion,
				PayloadVersion: GameEventPayloadVersion,
				Notable:        domain.IsRareEnemy(out.Result.Enemy.Kind) || out.Result.LeveledUp > 0,
				Payload: ExpeditionFinishedPayload{
					CatName: out.Cat.Name, Breed: out.Cat.Breed, Trait: out.Cat.Trait,
					Energy: out.Cat.Energy, Level: out.Cat.Level, Result: out.Result,
				},
			})
			if out.Result.LeveledUp > 0 {
				_ = s.events.Publish(ctx, GameEvent{
					DedupeKey: baseKey + ":leveled-up", Kind: GameEventCatLeveledUp,
					UserID: userID, CatID: out.Cat.ID, OccurredAt: now,
					RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion,
					PayloadVersion: GameEventPayloadVersion, Notable: true,
					Payload: CatLeveledUpPayload{CatName: out.Cat.Name, Level: out.Cat.Level, LevelsGained: out.Result.LeveledUp, StatsGained: out.Result.StatsGained},
				})
			}
			if out.Result.Loot.Dropped {
				_ = s.events.Publish(ctx, GameEvent{
					DedupeKey: baseKey + ":item-found", Kind: GameEventItemFound,
					UserID: userID, CatID: out.Cat.ID, OccurredAt: now,
					RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion,
					PayloadVersion: GameEventPayloadVersion,
					Notable:        out.Result.Loot.New || out.Result.Loot.Rarity != domain.ItemCommon,
					Payload:        ItemFoundPayload{ItemID: out.Result.Loot.ItemID, Rarity: out.Result.Loot.Rarity, IsNew: out.Result.Loot.New, Fragments: out.Result.Loot.Fragments},
				})
			}
		}
		return &out.Cat, out.Result, nil
	}

	return nil, domain.ExpeditionResult{}, domain.ErrConcurrentUpdate
}

func effectiveStatsFromOwned(owned []domain.OwnedItem) domain.StatDelta {
	var total domain.StatDelta
	for _, item := range owned {
		if !item.Equipped {
			continue
		}
		definition, ok := gamedata.ItemByID(item.ItemID)
		if ok {
			total.Add(definition.StatsAtLevel(item.Level))
		}
	}
	return total
}
