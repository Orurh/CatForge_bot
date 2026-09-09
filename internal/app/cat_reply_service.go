package app

import (
	"context"
	"fmt"

	"catforge/internal/ai"
	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

const (
	requestedAIUserHourlyLimit = 12
	requestedAIChatHourlyLimit = 40
)

type CatReplyService struct {
	cats          CatRepository
	personalities PersonalityRepository
	quota         AIQuotaRepository
	yards         YardRepository
	voice         ai.Generator
	clock         Clock
	events        GameEventSink
}

func NewCatReplyService(cats CatRepository, personalities PersonalityRepository, quota AIQuotaRepository, yards YardRepository, voice ai.Generator, clock Clock, events GameEventSink) *CatReplyService {
	return &CatReplyService{cats: cats, personalities: personalities, quota: quota, yards: yards, voice: voice, clock: clock, events: events}
}

func (s *CatReplyService) Reply(ctx context.Context, userID, chatID int64, requestKey, message string) (*domain.Cat, ai.Generation, error) {
	now := s.clock.Now()
	cat, err := s.cats.GetByUserID(ctx, userID)
	if err != nil {
		return nil, ai.Generation{}, err
	}
	allowed, err := s.quota.AllowAIRequest(ctx, userID, chatID, now, requestedAIUserHourlyLimit, requestedAIChatHourlyLimit)
	if err != nil {
		return nil, ai.Generation{}, err
	}
	if !allowed {
		return nil, ai.Generation{}, domain.ErrAIRateLimited
	}
	personality, err := s.personalities.GetByCatID(ctx, cat.ID)
	if err != nil {
		return nil, ai.Generation{}, err
	}
	yardID := int64(0)
	var relationships []ai.RelationshipContext
	if s.yards != nil {
		if yard, yardErr := s.yards.GetByTelegramChatID(ctx, chatID); yardErr == nil && yard != nil {
			yardID = yard.ID
			if stored, relationshipErr := s.yards.ListRelationships(ctx, yard.ID); relationshipErr == nil {
				relationships = relationshipContextsForCat(cat.ID, stored, maxAIRelationships)
			}
		}
	}
	var progressionFacts []string
	if history, ok := s.cats.(interface {
		RecentProgressionFacts(context.Context, int64) ([]string, error)
	}); ok {
		progressionFacts, _ = history.RecentProgressionFacts(ctx, cat.ID)
	}
	generation, err := s.voice.GenerateCatReply(ctx, ai.GenerationRequest{
		Cat: ai.CatContext{
			ID: cat.ID, Name: cat.Name, Breed: cat.Breed, Trait: personality.Trait,
			SpeechStyle: personality.SpeechStyle,
		},
		EventFacts: progressionFacts, YardID: yardID, HumorMode: personality.HumorMode, UserMessage: message, Relationships: relationships,
	})
	if err != nil {
		return nil, ai.Generation{}, err
	}
	if s.events != nil {
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: requestKey, Kind: GameEventCatReplyGenerated,
			UserID: userID, CatID: cat.ID, YardID: yardID, OccurredAt: now,
			ContentVersion: gameengine.CurrentContentVersion, PayloadVersion: GameEventPayloadVersion,
			Payload: CatReplyGeneratedPayload{
				TargetChatID: chatID, Provider: generation.Provider, Model: generation.Model,
				Emotion: generation.Emotion, Fallback: generation.Fallback, Requested: true,
			},
		})
	}
	return cat, generation, nil
}

func CatReplyRequestKey(chatID int64, messageID int) string {
	return fmt.Sprintf("chat:%d:message:%d:cat-reply", chatID, messageID)
}
