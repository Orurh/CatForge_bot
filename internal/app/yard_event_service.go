package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

const (
	yardEventDuration       = 6 * time.Hour
	yardEventActivityWindow = 7 * 24 * time.Hour
)

type YardEventService struct {
	yards      YardRepository
	eventsRepo YardEventRepository
	engine     gameengine.Engine
	voice      ai.Generator
	clock      Clock
	rng        RNG
	events     GameEventSink
	banter     *CatBanterService
}

func (s *YardEventService) SetBanterService(banter *CatBanterService) {
	s.banter = banter
}

type YardEventStatus struct {
	Yard    *domain.Yard
	Event   *domain.YardEvent
	Counts  map[domain.YardEventChoiceID]int
	Created bool
}

func NewYardEventService(yards YardRepository, eventsRepo YardEventRepository, engine gameengine.Engine, voice ai.Generator, clock Clock, rng RNG, events GameEventSink) *YardEventService {
	return &YardEventService{yards: yards, eventsRepo: eventsRepo, engine: engine, voice: voice, clock: clock, rng: rng, events: events}
}

func (s *YardEventService) StartOrGet(ctx context.Context, telegramChatID int64) (YardEventStatus, error) {
	yard, err := s.yards.GetByTelegramChatID(ctx, telegramChatID)
	if err != nil {
		return YardEventStatus{}, err
	}
	now := s.clock.Now()
	seed := int64(s.rng.Intn(1<<30))<<30 | int64(s.rng.Intn(1<<30))
	eventType, err := s.eventsRepo.NextEventType(ctx, yard.ID)
	if err != nil {
		return YardEventStatus{}, err
	}
	event, created, err := s.eventsRepo.StartOrGet(
		ctx, yard.ID, eventType, seed, now, now.Add(yardEventDuration), gameengine.CurrentContentVersion,
	)
	if err != nil {
		return YardEventStatus{}, err
	}
	if created && s.events != nil {
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: fmt.Sprintf("yard:%d:event:%d:started", yard.ID, event.ID), Kind: GameEventYardEventStarted,
			YardID: yard.ID, OccurredAt: now, ContentVersion: gameengine.CurrentContentVersion,
			PayloadVersion: GameEventPayloadVersion, Notable: true,
			Payload: YardEventStartedPayload{
				EventID: event.ID, EventType: string(event.Type), Seed: event.Seed,
				StartsAt: event.StartsAt, ResolvesAt: event.ResolvesAt,
			},
		})
	}
	counts, err := s.eventsRepo.ChoiceCounts(ctx, event.ID)
	if err != nil {
		return YardEventStatus{}, err
	}
	return YardEventStatus{Yard: yard, Event: event, Counts: counts, Created: created}, nil
}

func (s *YardEventService) GetActive(ctx context.Context, telegramChatID int64) (YardEventStatus, error) {
	yard, err := s.yards.GetByTelegramChatID(ctx, telegramChatID)
	if err != nil {
		return YardEventStatus{}, err
	}
	event, err := s.eventsRepo.GetCurrentActive(ctx, telegramChatID)
	if err != nil {
		return YardEventStatus{}, err
	}
	counts, err := s.eventsRepo.ChoiceCounts(ctx, event.ID)
	if err != nil {
		return YardEventStatus{}, err
	}
	return YardEventStatus{Yard: yard, Event: event, Counts: counts}, nil
}

func (s *YardEventService) StartDue(ctx context.Context, limit int) (int, error) {
	now := s.clock.Now()
	yards, err := s.eventsRepo.ListStartCandidates(ctx, now, now.Add(-yardEventActivityWindow), limit)
	if err != nil {
		return 0, err
	}
	started := 0
	for _, yard := range yards {
		seed := int64(s.rng.Intn(1<<30))<<30 | int64(s.rng.Intn(1<<30))
		eventType, typeErr := s.eventsRepo.NextEventType(ctx, yard.ID)
		if typeErr != nil {
			return started, typeErr
		}
		event, created, startErr := s.eventsRepo.StartOrGet(
			ctx, yard.ID, eventType, seed, now, now.Add(yardEventDuration), gameengine.CurrentContentVersion,
		)
		if startErr != nil {
			return started, startErr
		}
		if !created {
			continue
		}
		started++
		if s.events != nil {
			_ = s.events.Publish(ctx, GameEvent{
				DedupeKey: fmt.Sprintf("yard:%d:event:%d:started", yard.ID, event.ID), Kind: GameEventYardEventStarted,
				YardID: yard.ID, OccurredAt: now, ContentVersion: gameengine.CurrentContentVersion,
				PayloadVersion: GameEventPayloadVersion, Notable: true,
				Payload: YardEventStartedPayload{
					EventID: event.ID, TelegramChatID: yard.TelegramChatID, EventType: string(event.Type), Seed: event.Seed,
					StartsAt: event.StartsAt, ResolvesAt: event.ResolvesAt,
				},
			})
		}
	}
	return started, nil
}

func (s *YardEventService) ResolveDue(ctx context.Context, limit int) (int, error) {
	ids, err := s.eventsRepo.DueEventIDs(ctx, s.clock.Now(), limit)
	if err != nil {
		return 0, err
	}
	resolved := 0
	for _, eventID := range ids {
		saved, err := s.Resolve(ctx, eventID)
		if err != nil {
			return resolved, err
		}
		if saved {
			resolved++
		}
	}
	return resolved, nil
}

func (s *YardEventService) Resolve(ctx context.Context, eventID int64) (bool, error) {
	input, err := s.eventsRepo.GetResolutionInput(ctx, eventID)
	if err != nil {
		if errors.Is(err, domain.ErrYardEventUnavailable) {
			return false, nil
		}
		return false, err
	}
	result, err := s.engine.ResolveYardEvent(ctx, gameengine.YardEventInput{
		RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: input.Event.ContentVersion,
		Seed: uint64(input.Event.Seed), EventType: input.Event.Type, Participants: input.Participants,
	})
	if err != nil {
		return false, err
	}
	now := s.clock.Now()
	saved, err := s.eventsRepo.SaveResolution(ctx, eventID, result, gameengine.CurrentRulesVersion, input.Event.ContentVersion, now)
	if err != nil || !saved {
		return saved, err
	}
	relationships, _ := s.yards.ListRelationships(ctx, input.Event.YardID)
	narrativeRequest := eventNarrativeRequest(input, result, relationships)
	generation := ai.Generation{}
	if s.voice != nil {
		generation, _ = s.voice.GenerateEventNarrative(ctx, narrativeRequest)
	}
	if s.events != nil {
		participants := resolvedEventParticipants(input.Participants, result.Participants)
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: fmt.Sprintf("yard:%d:event:%d:resolved", input.Event.YardID, eventID),
			Kind:      GameEventYardEventResolved, YardID: input.Event.YardID, OccurredAt: now,
			RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: input.Event.ContentVersion,
			PayloadVersion: GameEventPayloadVersion, Notable: true,
			Payload: YardEventResolvedPayload{
				EventID: eventID, TelegramChatID: input.TelegramChatID, EventType: string(input.Event.Type), Result: result,
				Participants: participants,
				Narrative:    generation.Text, Provider: generation.Provider, Model: generation.Model, Fallback: generation.Fallback,
			},
		})
	}
	s.publishAutonomousReaction(ctx, input, result, relationships, now)
	if s.banter != nil {
		if yard, yardErr := s.yards.GetByID(ctx, input.Event.YardID); yardErr == nil {
			_, _ = s.banter.PublishYardEvent(ctx, yard, input, result, relationships, now)
		}
	}
	return true, nil
}

func resolvedEventParticipants(participants []domain.YardEventParticipant, results []domain.YardEventParticipantResult) []YardEventParticipantResultPayload {
	byID := make(map[int64]domain.YardEventParticipant, len(participants))
	for _, participant := range participants {
		byID[participant.CatID] = participant
	}
	resolved := make([]YardEventParticipantResultPayload, 0, len(results))
	for _, result := range results {
		cat := byID[result.CatID]
		resolved = append(resolved, YardEventParticipantResultPayload{
			CatID: result.CatID, CatName: cat.CatName, Breed: cat.Breed, Level: cat.Level,
			Choice: result.Choice, Contribution: result.Contribution, MVP: result.MVP,
		})
	}
	return resolved
}

func (s *YardEventService) publishAutonomousReaction(ctx context.Context, input domain.YardEventResolutionInput, result domain.YardEventResult, relationships []domain.CatRelationship, now time.Time) {
	if s.voice == nil || s.events == nil || len(result.Participants) == 0 {
		return
	}
	yard, err := s.yards.GetByID(ctx, input.Event.YardID)
	if err != nil || !yard.AutoMessagesEnabled || yard.MaxAutoMessagesDay <= 0 || yard.QuietUntil.After(now) {
		return
	}
	participants := make(map[int64]domain.YardEventParticipant, len(input.Participants))
	for _, participant := range input.Participants {
		participants[participant.CatID] = participant
	}
	var speaker domain.YardEventParticipant
	bestContribution := -1
	for _, outcome := range result.Participants {
		candidate := participants[outcome.CatID]
		if !candidate.AutoSpeakEnabled {
			continue
		}
		if outcome.MVP || outcome.Contribution > bestContribution {
			speaker = candidate
			bestContribution = outcome.Contribution
		}
		if outcome.MVP {
			break
		}
	}
	if speaker.CatID == 0 {
		return
	}
	allowed, err := s.yards.ClaimAutoMessageSlot(ctx, yard.ID, now, yard.MaxAutoMessagesDay, domain.AutoMessageSingle)
	if err != nil || !allowed {
		return
	}
	request := eventNarrativeRequest(input, result, relationships)
	request.Type = ai.GenerationAutonomousCat
	request.HumorMode = yard.HumorMode
	request.Cat = ai.CatContext{
		ID: speaker.CatID, Name: speaker.CatName, Breed: speaker.Breed,
		Trait: speaker.Trait, SpeechStyle: speaker.SpeechStyle,
	}
	generation, err := s.voice.Generate(ctx, request)
	if err != nil || generation.Text == "" {
		return
	}
	_ = s.events.Publish(ctx, GameEvent{
		DedupeKey: fmt.Sprintf("yard:%d:event:%d:auto-cat:%d", yard.ID, input.Event.ID, speaker.CatID),
		Kind:      GameEventAutonomousCatMessage, CatID: speaker.CatID, YardID: yard.ID, OccurredAt: now,
		ContentVersion: input.Event.ContentVersion, PayloadVersion: GameEventPayloadVersion, Notable: true,
		Payload: AutonomousCatMessagePayload{
			TelegramChatID: input.TelegramChatID, CatName: speaker.CatName, Breed: speaker.Breed,
			Level: speaker.Level, Text: generation.Text, Trigger: string(GameEventYardEventResolved),
			Provider: generation.Provider, Model: generation.Model, Fallback: generation.Fallback,
		},
	})
}

func eventNarrativeRequest(input domain.YardEventResolutionInput, result domain.YardEventResult, relationships []domain.CatRelationship) ai.GenerationRequest {
	byID := make(map[int64]domain.YardEventParticipant, len(input.Participants))
	participantIDs := make(map[int64]struct{}, len(input.Participants))
	for _, participant := range input.Participants {
		byID[participant.CatID] = participant
		participantIDs[participant.CatID] = struct{}{}
	}
	participants := make([]ai.EventParticipantContext, 0, len(result.Participants))
	facts := []string{
		"event_type=" + string(input.Event.Type), "outcome_tier=" + string(result.OutcomeTier),
		"team_score=" + strconv.Itoa(result.TeamScore), "target_score=" + strconv.Itoa(result.TargetScore),
		"yard_score=" + strconv.Itoa(result.YardScore), "xp_gain=" + strconv.FormatInt(result.XPGain, 10),
		"secret_found=" + strconv.FormatBool(result.SecretFound),
	}
	for _, outcome := range result.Participants {
		name := byID[outcome.CatID].CatName
		participants = append(participants, ai.EventParticipantContext{
			CatName: name, Choice: string(outcome.Choice), Contribution: outcome.Contribution,
			MVP: outcome.MVP,
		})
		facts = append(facts, fmt.Sprintf("cat=%q choice=%s contribution=%d mvp=%t", name, outcome.Choice, outcome.Contribution, outcome.MVP))
		facts = append(facts, outcome.ProgressionFacts...)
	}
	for _, effect := range result.RelationshipEffects {
		facts = append(facts, fmt.Sprintf(
			"relationship cat_a=%q cat_b=%q friendship_delta=%d rivalry_delta=%d respect_delta=%d",
			byID[effect.CatAID].CatName, byID[effect.CatBID].CatName,
			effect.FriendshipDelta, effect.RivalryDelta, effect.RespectDelta,
		))
	}
	return ai.GenerationRequest{
		YardID: input.Event.YardID, HumorMode: ai.HumorNormal, EventFacts: facts,
		Relationships: relationshipContextsAmong(participantIDs, relationships),
		Event: &ai.EventContext{
			EventType: string(input.Event.Type), OutcomeTier: string(result.OutcomeTier), TeamScore: result.TeamScore,
			TargetScore: result.TargetScore, YardScore: result.YardScore, XPGain: result.XPGain, SecretFound: result.SecretFound,
			StrategyBonus: result.StrategyBonus, Participants: participants,
		},
	}
}

func (s *YardEventService) Choose(ctx context.Context, telegramChatID, eventID, userID int64, choice domain.YardEventChoiceID) (YardEventStatus, error) {
	if !domain.IsValidYardEventChoice(choice) {
		return YardEventStatus{}, domain.ErrInvalidYardChoice
	}
	now := s.clock.Now()
	saved, first, err := s.eventsRepo.SubmitChoice(ctx, telegramChatID, eventID, userID, choice, now)
	if err != nil {
		return YardEventStatus{}, err
	}
	yard, err := s.yards.GetByTelegramChatID(ctx, telegramChatID)
	if err != nil {
		return YardEventStatus{}, err
	}
	if s.events != nil {
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: fmt.Sprintf("yard:%d:event:%d:cat:%d:choice:%s:at:%d", yard.ID, eventID, saved.CatID, choice, now.UnixNano()),
			Kind:      GameEventYardChoiceSubmitted, UserID: userID, CatID: saved.CatID, YardID: yard.ID, OccurredAt: now,
			ContentVersion: gameengine.CurrentContentVersion, PayloadVersion: GameEventPayloadVersion,
			Payload: YardChoiceSubmittedPayload{EventID: eventID, Choice: string(choice), First: first},
		})
	}
	counts, err := s.eventsRepo.ChoiceCounts(ctx, eventID)
	if err != nil {
		return YardEventStatus{}, err
	}
	event, err := s.eventsRepo.GetActive(ctx, telegramChatID, eventID)
	if err != nil {
		return YardEventStatus{}, err
	}
	return YardEventStatus{Yard: yard, Event: event, Counts: counts}, nil
}
