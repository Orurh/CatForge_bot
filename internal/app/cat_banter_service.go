package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

const IdleBanterActivityWindow = 7 * 24 * time.Hour

const (
	BanterTriggerIdle      = "idle"
	BanterTriggerYardEvent = "yard_event"
)

type CatBanterService struct {
	yards         YardRepository
	idleYards     IdleBanterYardRepository
	personalities PersonalityRepository
	voice         ai.Generator
	clock         Clock
	events        GameEventSink
}

type banterSpeaker struct {
	ID           int64
	Name         string
	Breed        domain.Breed
	Trait        domain.Trait
	Level        int
	SpeechStyle  string
	HumorMode    domain.HumorMode
	Choice       domain.YardEventChoiceID
	Contribution int
	MVP          bool
}

func NewCatBanterService(yards YardRepository, idleYards IdleBanterYardRepository, personalities PersonalityRepository, voice ai.Generator, clock Clock, events GameEventSink) *CatBanterService {
	return &CatBanterService{yards: yards, idleYards: idleYards, personalities: personalities, voice: voice, clock: clock, events: events}
}

func (s *CatBanterService) RunDue(ctx context.Context, limit int) (int, error) {
	if s == nil || s.idleYards == nil || s.yards == nil || s.personalities == nil || s.voice == nil || s.events == nil {
		return 0, nil
	}
	now := s.clock.Now()
	yards, err := s.idleYards.ListIdleBanterCandidates(ctx, now, now.Add(-IdleBanterActivityWindow), limit)
	if err != nil {
		return 0, err
	}
	published := 0
	for index := range yards {
		ok, publishErr := s.publishIdle(ctx, &yards[index], now)
		if publishErr != nil {
			return published, publishErr
		}
		if ok {
			published++
		}
	}
	return published, nil
}

func (s *CatBanterService) publishIdle(ctx context.Context, yard *domain.Yard, now time.Time) (bool, error) {
	if yard == nil || !banterAllowed(yard, now) {
		return false, nil
	}
	members, err := s.yards.ListMembers(ctx, yard.ID)
	if err != nil {
		return false, err
	}
	speakers := make([]banterSpeaker, 0, len(members))
	for _, member := range members {
		if member.LastActiveAt.Before(now.Add(-IdleBanterActivityWindow)) {
			continue
		}
		personality, personalityErr := s.personalities.GetByCatID(ctx, member.CatID)
		if personalityErr != nil {
			return false, personalityErr
		}
		if personality == nil || !personality.AutoSpeakEnabled {
			continue
		}
		speakers = append(speakers, banterSpeaker{
			ID: member.CatID, Name: member.CatName, Breed: member.Breed, Trait: personality.Trait,
			Level: member.Level, SpeechStyle: personality.SpeechStyle, HumorMode: personality.HumorMode,
		})
	}
	if len(speakers) < 2 {
		return false, nil
	}
	relationships, err := s.yards.ListRelationships(ctx, yard.ID)
	if err != nil {
		return false, err
	}
	first, second, relationship := chooseBanterPair(speakers, relationships)
	context := &ai.YardBanterContext{Trigger: BanterTriggerIdle}
	if relationship != nil {
		context.Friendship = relationship.Friendship
		context.Rivalry = relationship.Rivalry
		context.Respect = relationship.Respect
	}
	request := yardBanterRequest(yard, first, second, context, relationship)
	return s.generateAndPublish(ctx, yard, first, second, request, BanterTriggerIdle, 0,
		fmt.Sprintf("yard:%d:idle-banter:%s", yard.ID, now.UTC().Format("2006-01-02")), now)
}

func (s *CatBanterService) PublishYardEvent(ctx context.Context, yard *domain.Yard, input domain.YardEventResolutionInput, result domain.YardEventResult, relationships []domain.CatRelationship, now time.Time) (bool, error) {
	if s == nil || s.yards == nil || s.voice == nil || s.events == nil || yard == nil || !banterAllowed(yard, now) {
		return false, nil
	}
	outcomes := make(map[int64]domain.YardEventParticipantResult, len(result.Participants))
	for _, outcome := range result.Participants {
		outcomes[outcome.CatID] = outcome
	}
	speakers := make([]banterSpeaker, 0, len(input.Participants))
	for _, participant := range input.Participants {
		outcome, ok := outcomes[participant.CatID]
		if !ok || !participant.AutoSpeakEnabled {
			continue
		}
		speakers = append(speakers, banterSpeaker{
			ID: participant.CatID, Name: participant.CatName, Breed: participant.Breed, Trait: participant.Trait,
			Level: participant.Level, SpeechStyle: participant.SpeechStyle, HumorMode: participant.HumorMode,
			Choice: participant.Choice, Contribution: outcome.Contribution, MVP: outcome.MVP,
		})
	}
	if len(speakers) < 2 || !notableEventBanter(result, speakers, relationships) {
		return false, nil
	}
	first, second, relationship := chooseBanterPair(speakers, relationships)
	context := &ai.YardBanterContext{
		Trigger: BanterTriggerYardEvent, EventType: string(input.Event.Type), OutcomeTier: string(result.OutcomeTier),
		CatAChoice: string(first.Choice), CatBChoice: string(second.Choice),
		CatAContribution: first.Contribution, CatBContribution: second.Contribution, CatAMVP: first.MVP, CatBMVP: second.MVP,
	}
	if relationship != nil {
		context.Friendship = relationship.Friendship
		context.Rivalry = relationship.Rivalry
		context.Respect = relationship.Respect
	}
	request := yardBanterRequest(yard, first, second, context, relationship)
	request.EventFacts = append(request.EventFacts,
		"event_type="+strconv.Quote(string(input.Event.Type)), "outcome_tier="+strconv.Quote(string(result.OutcomeTier)),
		"cat_a_choice="+strconv.Quote(string(first.Choice)), "cat_b_choice="+strconv.Quote(string(second.Choice)),
		"cat_a_contribution="+strconv.Itoa(first.Contribution), "cat_b_contribution="+strconv.Itoa(second.Contribution),
		"cat_a_mvp="+strconv.FormatBool(first.MVP), "cat_b_mvp="+strconv.FormatBool(second.MVP),
	)
	return s.generateAndPublish(ctx, yard, first, second, request, BanterTriggerYardEvent, input.Event.ID,
		fmt.Sprintf("yard:%d:event:%d:banter", yard.ID, input.Event.ID), now)
}

func (s *CatBanterService) generateAndPublish(ctx context.Context, yard *domain.Yard, first, second banterSpeaker, request ai.GenerationRequest, trigger string, sourceEventID int64, dedupeKey string, now time.Time) (bool, error) {
	allowed, err := s.yards.ClaimAutoMessageSlot(ctx, yard.ID, now, yard.MaxAutoMessagesDay, domain.AutoMessageBanter)
	if err != nil || !allowed {
		return false, err
	}
	generation, err := s.voice.GenerateYardBanter(ctx, request)
	if err != nil {
		return false, nil
	}
	lines, ok := ai.ParseArenaBanterLines(generation.Text)
	if !ok {
		return false, nil
	}
	err = s.events.Publish(ctx, GameEvent{
		DedupeKey: dedupeKey, Kind: GameEventCatBanter, CatID: first.ID, YardID: yard.ID, OccurredAt: now,
		ContentVersion: gameengine.CurrentContentVersion, PayloadVersion: GameEventPayloadVersion, Notable: true,
		Payload: CatBanterPayload{
			TelegramChatID: yard.TelegramChatID,
			FirstCatID:     first.ID, FirstCatName: first.Name, FirstCatBreed: first.Breed, FirstCatLevel: first.Level, FirstLine: lines[0],
			SecondCatID: second.ID, SecondCatName: second.Name, SecondCatBreed: second.Breed, SecondCatLevel: second.Level, SecondLine: lines[1],
			Trigger: trigger, SourceEventID: sourceEventID,
			Provider: generation.Provider, Model: generation.Model, Fallback: generation.Fallback,
		},
	})
	return err == nil, err
}

func banterAllowed(yard *domain.Yard, now time.Time) bool {
	return yard != nil && yard.AutoMessagesEnabled && yard.CatToCatBanter && yard.MaxAutoMessagesDay > 0 && !yard.QuietUntil.After(now)
}

func yardBanterRequest(yard *domain.Yard, first, second banterSpeaker, context *ai.YardBanterContext, relationship *domain.CatRelationship) ai.GenerationRequest {
	humor := domain.HumorNormal
	if yard.HumorMode == domain.HumorBold && first.HumorMode == domain.HumorBold && second.HumorMode == domain.HumorBold {
		humor = domain.HumorBold
	}
	request := ai.GenerationRequest{
		YardID: yard.ID, HumorMode: humor,
		Cat:        ai.CatContext{ID: first.ID, Name: first.Name, Breed: first.Breed, Trait: first.Trait, SpeechStyle: first.SpeechStyle},
		OtherCat:   ai.CatContext{ID: second.ID, Name: second.Name, Breed: second.Breed, Trait: second.Trait, SpeechStyle: second.SpeechStyle},
		YardBanter: context,
	}
	if relationship != nil {
		request.Relationships = []ai.RelationshipContext{{
			CatAName: relationship.CatAName, CatBName: relationship.CatBName,
			Friendship: relationship.Friendship, Rivalry: relationship.Rivalry, Respect: relationship.Respect,
		}}
	}
	return request
}

func chooseBanterPair(speakers []banterSpeaker, relationships []domain.CatRelationship) (banterSpeaker, banterSpeaker, *domain.CatRelationship) {
	sorted := append([]banterSpeaker(nil), speakers...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	byID := make(map[int64]banterSpeaker, len(sorted))
	for _, speaker := range sorted {
		byID[speaker.ID] = speaker
	}
	first, second := sorted[0], sorted[1]
	var best *domain.CatRelationship
	bestScore := -1
	for index := range relationships {
		relationship := &relationships[index]
		catA, okA := byID[relationship.CatAID]
		catB, okB := byID[relationship.CatBID]
		if !okA || !okB {
			continue
		}
		score := max(relationship.Friendship, max(relationship.Rivalry, relationship.Respect))*100 +
			catA.Contribution + catB.Contribution
		if catA.Choice != "" && catB.Choice != "" && catA.Choice != catB.Choice {
			score += 10
		}
		if catA.MVP || catB.MVP {
			score += 20
		}
		if score > bestScore {
			first, second, best, bestScore = catA, catB, relationship, score
		}
	}
	if first.MVP && !second.MVP {
		first, second = second, first
	}
	return first, second, best
}

func notableEventBanter(result domain.YardEventResult, speakers []banterSpeaker, relationships []domain.CatRelationship) bool {
	if result.OutcomeTier == domain.YardOutcomeFailure || result.OutcomeTier == domain.YardOutcomeExceptional || result.SecretFound {
		return true
	}
	for first := 0; first < len(speakers); first++ {
		for second := first + 1; second < len(speakers); second++ {
			if speakers[first].Choice != "" && speakers[first].Choice != speakers[second].Choice {
				return true
			}
		}
	}
	eligible := make(map[int64]struct{}, len(speakers))
	for _, speaker := range speakers {
		eligible[speaker.ID] = struct{}{}
	}
	for _, relationship := range relationships {
		_, firstEligible := eligible[relationship.CatAID]
		_, secondEligible := eligible[relationship.CatBID]
		if firstEligible && secondEligible && max(relationship.Friendship, max(relationship.Rivalry, relationship.Respect)) >= 3 {
			return true
		}
	}
	return false
}
