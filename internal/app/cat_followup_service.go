package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

const CatMessageReferenceTTL = 48 * time.Hour

type CatFollowupResult struct {
	Cat        *domain.Cat
	Generation ai.Generation
}

type CatFollowupService struct {
	refs          CatMessageReferenceRepository
	yards         YardRepository
	personalities PersonalityRepository
	quota         AIQuotaRepository
	voice         ai.Generator
	clock         Clock
	events        GameEventSink
}

func NewCatFollowupService(refs CatMessageReferenceRepository, yards YardRepository, personalities PersonalityRepository, quota AIQuotaRepository, voice ai.Generator, clock Clock, events GameEventSink) *CatFollowupService {
	return &CatFollowupService{
		refs: refs, yards: yards, personalities: personalities, quota: quota,
		voice: voice, clock: clock, events: events,
	}
}

func (s *CatFollowupService) Remember(ctx context.Context, chatID int64, messageID int, catID int64) error {
	if s == nil || s.refs == nil || chatID == 0 || messageID <= 0 || catID <= 0 {
		return nil
	}
	return s.refs.Remember(ctx, chatID, messageID, catID, s.clock.Now().Add(CatMessageReferenceTTL))
}

// Reply claims a known cat message once. matched is true when the incoming
// Telegram reply really targeted a remembered bot message, even if current
// Yard settings suppress the cat's answer.
func (s *CatFollowupService) Reply(ctx context.Context, userID, chatID int64, sourceMessageID, humanReplyMessageID int, previousMessage, humanReply string) (result *CatFollowupResult, matched bool, err error) {
	if s == nil || s.refs == nil || strings.TrimSpace(humanReply) == "" {
		return nil, false, nil
	}
	now := s.clock.Now()
	ref, claimed, err := s.refs.Claim(ctx, chatID, sourceMessageID, now)
	if err != nil || !claimed {
		return nil, false, err
	}

	yard, err := s.yards.GetByTelegramChatID(ctx, chatID)
	if err != nil {
		if errors.Is(err, domain.ErrNoYard) {
			return nil, true, nil
		}
		return nil, true, err
	}
	s.publishHumanReply(ctx, userID, ref.CatID, yard.ID, chatID, sourceMessageID, humanReplyMessageID, now)
	if !yard.AutoMessagesEnabled || yard.MaxAutoMessagesDay < 1 || yard.QuietUntil.After(now) {
		return nil, true, nil
	}

	members, err := s.yards.ListMembers(ctx, yard.ID)
	if err != nil {
		return nil, true, err
	}
	var speaker *domain.YardMember
	for index := range members {
		if members[index].CatID == ref.CatID {
			speaker = &members[index]
			break
		}
	}
	if speaker == nil {
		return nil, true, nil
	}
	personality, err := s.personalities.GetByCatID(ctx, ref.CatID)
	if err != nil {
		return nil, true, err
	}
	if !personality.AutoSpeakEnabled {
		return nil, true, nil
	}
	allowed, err := s.quota.AllowAIRequest(ctx, userID, chatID, now, requestedAIUserHourlyLimit, requestedAIChatHourlyLimit)
	if err != nil {
		return nil, true, err
	}
	if !allowed {
		return nil, true, domain.ErrAIRateLimited
	}
	relationships, _ := s.yards.ListRelationships(ctx, yard.ID)

	generation, err := s.voice.Generate(ctx, ai.GenerationRequest{
		Type: ai.GenerationHumanReplyToCat,
		Cat: ai.CatContext{
			ID: ref.CatID, Name: speaker.CatName, Breed: speaker.Breed, Trait: personality.Trait,
			SpeechStyle: personality.SpeechStyle,
		},
		YardID: yard.ID, HumorMode: yard.HumorMode,
		PreviousMessage: previousMessage, UserMessage: humanReply,
		Relationships: relationshipContextsForCat(ref.CatID, relationships, maxAIRelationships),
	})
	if err != nil {
		return nil, true, err
	}
	cat := &domain.Cat{
		ID: ref.CatID, UserID: speaker.UserID, Name: speaker.CatName,
		Breed: speaker.Breed, Trait: personality.Trait, Level: speaker.Level,
	}
	s.publishFollowup(ctx, userID, cat.ID, yard.ID, chatID, sourceMessageID, humanReplyMessageID, generation, now)
	return &CatFollowupResult{Cat: cat, Generation: generation}, true, nil
}

func (s *CatFollowupService) publishHumanReply(ctx context.Context, userID, catID, yardID, chatID int64, sourceMessageID, humanReplyMessageID int, now time.Time) {
	if s.events == nil {
		return
	}
	_ = s.events.Publish(ctx, GameEvent{
		DedupeKey: fmt.Sprintf("chat:%d:message:%d:human-reply-to-cat", chatID, humanReplyMessageID),
		Kind:      GameEventHumanRepliedToCat, UserID: userID, CatID: catID, YardID: yardID,
		OccurredAt: now, ContentVersion: gameengine.CurrentContentVersion, PayloadVersion: GameEventPayloadVersion,
		Payload: HumanRepliedToCatPayload{
			TelegramChatID: chatID, SourceMessageID: sourceMessageID, HumanReplyMessageID: humanReplyMessageID,
		},
	})
}

func (s *CatFollowupService) publishFollowup(ctx context.Context, userID, catID, yardID, chatID int64, sourceMessageID, humanReplyMessageID int, generation ai.Generation, now time.Time) {
	if s.events == nil {
		return
	}
	_ = s.events.Publish(ctx, GameEvent{
		DedupeKey: fmt.Sprintf("chat:%d:message:%d:cat-followup", chatID, humanReplyMessageID),
		Kind:      GameEventCatFollowupGenerated, UserID: userID, CatID: catID, YardID: yardID,
		OccurredAt: now, ContentVersion: gameengine.CurrentContentVersion, PayloadVersion: GameEventPayloadVersion,
		Payload: CatFollowupGeneratedPayload{
			TelegramChatID: chatID, SourceMessageID: sourceMessageID, HumanReplyMessageID: humanReplyMessageID,
			Provider: generation.Provider, Model: generation.Model, Emotion: generation.Emotion, Fallback: generation.Fallback,
		},
	})
}
