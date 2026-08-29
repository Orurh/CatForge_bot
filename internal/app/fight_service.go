package app

import (
	"context"
	"fmt"
	"time"

	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

const FightQueueTTL = 15 * time.Minute

type FightOutcome struct {
	Status   domain.FightQueueStatus
	Yard     *domain.Yard
	Cat      *domain.Cat
	Opponent *domain.Cat
	Result   domain.FightResult
	FightID  int64
	Rivalry  int
	Seed     uint64
}

type FightService struct {
	cats   CatRepository
	yards  YardRepository
	fights FightRepository
	engine gameengine.Engine
	clock  Clock
	rng    RNG
	events GameEventSink
}

func NewFightService(cats CatRepository, yards YardRepository, fights FightRepository, engine gameengine.Engine, clock Clock, rng RNG, events GameEventSink) *FightService {
	return &FightService{cats: cats, yards: yards, fights: fights, engine: engine, clock: clock, rng: rng, events: events}
}

// Toggle enters or leaves the caller's cat in the yard arena. Matching and
// removing a waiting opponent is performed atomically by FightRepository.
func (s *FightService) Toggle(ctx context.Context, telegramChatID, userID int64) (FightOutcome, error) {
	yard, err := s.yards.GetByTelegramChatID(ctx, telegramChatID)
	if err != nil {
		return FightOutcome{}, err
	}
	cat, err := s.cats.GetByUserID(ctx, userID)
	if err != nil {
		return FightOutcome{}, err
	}

	now := s.clock.Now()
	toggle, err := s.fights.ToggleQueue(ctx, yard.ID, userID, cat.ID, now, now.Add(FightQueueTTL))
	if err != nil {
		return FightOutcome{}, err
	}
	outcome := FightOutcome{Status: toggle.Status, Yard: yard, Cat: cat}
	if toggle.Status != domain.FightQueueMatched {
		return outcome, nil
	}

	opponent, err := s.cats.GetByUserID(ctx, toggle.Opponent.UserID)
	if err != nil {
		return FightOutcome{}, fmt.Errorf("load arena opponent: %w", err)
	}
	seed := s.fightSeed()
	result, err := s.engine.Fight(ctx, gameengine.FightInput{
		RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion,
		Seed: seed, CatA: *opponent, CatB: *cat,
	})
	if err != nil {
		return FightOutcome{}, err
	}
	if err := validateFightResult(result, opponent.ID, cat.ID); err != nil {
		return FightOutcome{}, err
	}

	rivalryDelta := 1
	if winnerHP(result, opponent.ID, cat.ID) <= 5 {
		rivalryDelta++
	}
	record := domain.FightRecord{
		YardID: yard.ID, CatAID: opponent.ID, CatBID: cat.ID,
		WinnerCatID: result.WinnerCatID, LoserCatID: result.LoserCatID,
		Seed: int64(seed), Rounds: result.Rounds, FinalHPA: result.FinalHPA, FinalHPB: result.FinalHPB,
		RivalryDelta: rivalryDelta, RulesVersion: gameengine.CurrentRulesVersion,
		ContentVersion: gameengine.CurrentContentVersion, CreatedAt: now,
	}
	fightID, rivalry, err := s.fights.SaveFight(ctx, record, result, opponent.Name, cat.Name)
	if err != nil {
		return FightOutcome{}, err
	}
	outcome.Opponent = opponent
	outcome.Result = result
	outcome.FightID = fightID
	outcome.Rivalry = rivalry
	outcome.Seed = seed

	if s.events != nil {
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: fmt.Sprintf("yard:%d:fight:%d:finished", yard.ID, fightID), Kind: GameEventFightFinished,
			UserID: userID, CatID: result.WinnerCatID, YardID: yard.ID, OccurredAt: now,
			RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion,
			PayloadVersion: GameEventPayloadVersion, Notable: rivalryDelta > 1,
			Payload: FightFinishedPayload{
				FightID: fightID, TelegramChatID: telegramChatID, CatAName: opponent.Name, CatBName: cat.Name,
				Result: result, Rivalry: rivalry, Seed: seed,
			},
		})
	}
	return outcome, nil
}

func (s *FightService) fightSeed() uint64 {
	return uint64(s.rng.Intn(1<<30))<<30 | uint64(s.rng.Intn(1<<30))
}

func winnerHP(result domain.FightResult, catAID, catBID int64) int {
	if result.WinnerCatID == catAID {
		return result.FinalHPA
	}
	if result.WinnerCatID == catBID {
		return result.FinalHPB
	}
	return 0
}

func validateFightResult(result domain.FightResult, catAID, catBID int64) error {
	validPair := (result.WinnerCatID == catAID && result.LoserCatID == catBID) ||
		(result.WinnerCatID == catBID && result.LoserCatID == catAID)
	if !validPair || result.Rounds < 1 || result.FinalHPA < 0 || result.FinalHPB < 0 || len(result.Turns) == 0 {
		return fmt.Errorf("game engine returned an invalid fight result")
	}
	return nil
}
