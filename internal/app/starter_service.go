package app

import (
	"context"
	"fmt"
	"strconv"

	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

type StarterService struct {
	cats   CatRepository
	clock  Clock
	rng    RNG
	events GameEventSink
}

func NewStarterService(cats CatRepository, clock Clock, rng RNG, events GameEventSink) *StarterService {
	return &StarterService{cats: cats, clock: clock, rng: rng, events: events}
}

func (s *StarterService) ChooseStarterCat(ctx context.Context, userID int64, breed domain.Breed) (*domain.Cat, error) {
	trait := s.randomTrait()
	hp, atk, def, spd := domain.BaseStatsByBreed(breed)
	name := s.defaultName()
	cat, err := s.cats.Create(ctx, userID, name, breed, trait, hp, atk, def, spd)
	if err != nil {
		return nil, err
	}
	if s.events != nil {
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: fmt.Sprintf("user:%d:cat:%d:created", userID, cat.ID),
			Kind:      GameEventCatCreated, UserID: userID, CatID: cat.ID, OccurredAt: s.clock.Now(),
			RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion,
			PayloadVersion: GameEventPayloadVersion, Notable: true,
			Payload: CatCreatedPayload{CatName: cat.Name, Breed: cat.Breed, Trait: cat.Trait, Level: cat.Level},
		})
	}
	return cat, nil
}

func (s *StarterService) randomTrait() domain.Trait {
	return domain.AllTraits[s.rng.Intn(len(domain.AllTraits))]
}

func (s *StarterService) defaultName() string {
	// "Мурчалкин123" (цифры рандомные)
	// 100..999, чтобы всегда были 3 цифры.
	n := 100 + s.rng.Intn(900)
	return domain.CatNameDefaultPrefix + strconv.Itoa(n)
}
