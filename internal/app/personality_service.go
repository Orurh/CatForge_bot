package app

import (
	"context"
	"fmt"
	"strconv"

	"catforge/internal/ai"
	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

type PersonalityService struct {
	repo   PersonalityRepository
	voice  ai.Generator
	events GameEventSink
	clock  Clock
}

func NewPersonalityService(repo PersonalityRepository, voice ai.Generator, events GameEventSink, clock Clock) *PersonalityService {
	return &PersonalityService{repo: repo, voice: voice, events: events, clock: clock}
}

func (s *PersonalityService) Get(ctx context.Context, catID int64) (*domain.CatPersonality, error) {
	if catID <= 0 {
		return nil, fmt.Errorf("cat id must be positive")
	}
	return s.repo.GetByCatID(ctx, catID)
}

func (s *PersonalityService) FirstLine(ctx context.Context, cat *domain.Cat) (ai.Generation, error) {
	if cat == nil || cat.ID <= 0 {
		return ai.Generation{}, fmt.Errorf("cat is required")
	}
	personality, err := s.Get(ctx, cat.ID)
	if err != nil {
		return ai.Generation{}, err
	}
	generation, err := s.voice.Generate(ctx, ai.GenerationRequest{
		Type: ai.GenerationFirstPersonalityLine,
		Cat: ai.CatContext{
			ID: cat.ID, Name: cat.Name, Breed: cat.Breed, Trait: personality.Trait,
			SpeechStyle: personality.SpeechStyle,
		},
		HumorMode: ai.HumorNormal,
	})
	if err != nil {
		return ai.Generation{}, err
	}
	if s.events != nil {
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: "cat:" + strconv.FormatInt(cat.ID, 10) + ":first-personality-line",
			Kind:      GameEventFirstPersonalityLine, UserID: cat.UserID, CatID: cat.ID,
			OccurredAt: s.clock.Now(), ContentVersion: gameengine.CurrentContentVersion,
			PayloadVersion: GameEventPayloadVersion, Notable: true,
			Payload: FirstPersonalityLinePayload{
				Provider: generation.Provider, Model: generation.Model,
				Emotion: generation.Emotion, Fallback: generation.Fallback,
			},
		})
	}
	return generation, nil
}

func (s *PersonalityService) SetAutoSpeak(ctx context.Context, catID int64, enabled bool) error {
	if catID <= 0 {
		return fmt.Errorf("cat id must be positive")
	}
	return s.repo.SetAutoSpeak(ctx, catID, enabled)
}

func (s *PersonalityService) SetHumorMode(ctx context.Context, catID int64, mode domain.HumorMode) error {
	if catID <= 0 {
		return fmt.Errorf("cat id must be positive")
	}
	if !domain.IsValidHumorMode(mode) {
		return fmt.Errorf("invalid humor mode %q", mode)
	}
	return s.repo.SetHumorMode(ctx, catID, mode)
}
