package app

import (
	"context"
	"testing"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
)

type idleBanterYards struct {
	candidates []domain.Yard
	err        error
}

func (s idleBanterYards) ListIdleBanterCandidates(context.Context, time.Time, time.Time, int) ([]domain.Yard, error) {
	return s.candidates, s.err
}

type banterPersonalities map[int64]*domain.CatPersonality

func (s banterPersonalities) GetByCatID(_ context.Context, catID int64) (*domain.CatPersonality, error) {
	return s[catID], nil
}
func (s banterPersonalities) SetTrait(context.Context, int64, domain.Trait, string) (*domain.CatPersonality, error) {
	return nil, nil
}
func (s banterPersonalities) SetHumorMode(context.Context, int64, domain.HumorMode) error {
	return nil
}
func (s banterPersonalities) SetAutoSpeak(context.Context, int64, bool) error { return nil }

func TestCatBanterSchedulerPublishesEligiblePair(t *testing.T) {
	t.Parallel()
	now := time.Unix(10_000, 0)
	claimed := []domain.AutoMessageKind{}
	yard := domain.Yard{
		ID: 9, TelegramChatID: -100, Name: "Друзья", HumorMode: domain.HumorBold,
		AutoMessagesEnabled: true, MaxAutoMessagesDay: 2, CatToCatBanter: true, FightsEnabled: true,
	}
	yards := stubYards{
		members: []domain.YardMember{
			{YardID: 9, CatID: 42, CatName: "Барсик", Breed: domain.BreedBengal, Level: 4, LastActiveAt: now.Add(-time.Hour)},
			{YardID: 9, CatID: 50, CatName: "Батон", Breed: domain.BreedBritish, Level: 5, LastActiveAt: now.Add(-time.Hour)},
		},
		relationships: []domain.CatRelationship{{CatAID: 42, CatBID: 50, CatAName: "Барсик", CatBName: "Батон", Rivalry: 8}},
		claimedKinds:  &claimed,
	}
	personalities := banterPersonalities{
		42: {CatID: 42, Trait: domain.TraitBully, SpeechStyle: "задиристо", HumorMode: domain.HumorBold, AutoSpeakEnabled: true},
		50: {CatID: 50, Trait: domain.TraitLazy, SpeechStyle: "лениво", HumorMode: domain.HumorBold, AutoSpeakEnabled: true},
	}
	events := &fakeEvents{}
	service := NewCatBanterService(
		yards, idleBanterYards{candidates: []domain.Yard{yard}}, personalities,
		ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second), fakeClock{t: now}, events,
	)

	count, err := service.RunDue(context.Background(), 20)
	if err != nil || count != 1 {
		t.Fatalf("RunDue() = %d, %v", count, err)
	}
	if len(claimed) != 1 || claimed[0] != domain.AutoMessageBanter {
		t.Fatalf("claimed kinds = %v", claimed)
	}
	if len(events.events) != 1 || events.events[0].Kind != GameEventCatBanter {
		t.Fatalf("events = %+v", events.events)
	}
	payload, ok := events.events[0].Payload.(CatBanterPayload)
	if !ok || payload.Trigger != BanterTriggerIdle || payload.FirstLine == "" || payload.SecondLine == "" || !payload.Fallback {
		t.Fatalf("payload = %+v", events.events[0].Payload)
	}
}

func TestCatBanterPublishesNotableYardEventConversation(t *testing.T) {
	t.Parallel()
	now := time.Unix(20_000, 0)
	claimed := []domain.AutoMessageKind{}
	yard := &domain.Yard{
		ID: 9, TelegramChatID: -100, HumorMode: domain.HumorNormal,
		AutoMessagesEnabled: true, MaxAutoMessagesDay: 2, CatToCatBanter: true,
	}
	yards := stubYards{claimedKinds: &claimed}
	events := &fakeEvents{}
	service := NewCatBanterService(yards, nil, nil, ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second), fakeClock{t: now}, events)
	input := domain.YardEventResolutionInput{
		Event: domain.YardEvent{ID: 17, YardID: 9, Type: domain.YardEventBigBox},
		Participants: []domain.YardEventParticipant{
			{CatID: 42, CatName: "Барсик", Breed: domain.BreedBengal, Trait: domain.TraitBully, Level: 4, SpeechStyle: "задиристо", AutoSpeakEnabled: true, Choice: domain.YardChoiceSteal},
			{CatID: 50, CatName: "Батон", Breed: domain.BreedBritish, Trait: domain.TraitLazy, Level: 5, SpeechStyle: "лениво", AutoSpeakEnabled: true, Choice: domain.YardChoiceScout},
		},
	}
	result := domain.YardEventResult{
		OutcomeTier: domain.YardOutcomeSuccess,
		Participants: []domain.YardEventParticipantResult{
			{CatID: 42, Choice: domain.YardChoiceSteal, Contribution: 8},
			{CatID: 50, Choice: domain.YardChoiceScout, Contribution: 12, MVP: true},
		},
	}

	published, err := service.PublishYardEvent(context.Background(), yard, input, result, nil, now)
	if err != nil || !published {
		t.Fatalf("PublishYardEvent() = %v, %v", published, err)
	}
	if len(events.events) != 1 || len(claimed) != 1 || claimed[0] != domain.AutoMessageBanter {
		t.Fatalf("events/claims = %+v/%v", events.events, claimed)
	}
	payload, ok := events.events[0].Payload.(CatBanterPayload)
	if !ok || payload.Trigger != BanterTriggerYardEvent || payload.SourceEventID != 17 || payload.SecondCatID != 50 {
		t.Fatalf("payload = %+v", events.events[0].Payload)
	}
}

func TestCatBanterNeedsTwoOptedInCats(t *testing.T) {
	t.Parallel()
	now := time.Unix(30_000, 0)
	yard := domain.Yard{ID: 9, AutoMessagesEnabled: true, MaxAutoMessagesDay: 2, CatToCatBanter: true}
	yards := stubYards{members: []domain.YardMember{
		{CatID: 42, LastActiveAt: now}, {CatID: 50, LastActiveAt: now},
	}}
	personalities := banterPersonalities{
		42: {CatID: 42, AutoSpeakEnabled: true},
		50: {CatID: 50, AutoSpeakEnabled: false},
	}
	events := &fakeEvents{}
	service := NewCatBanterService(yards, idleBanterYards{candidates: []domain.Yard{yard}}, personalities, ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second), fakeClock{t: now}, events)

	count, err := service.RunDue(context.Background(), 20)
	if err != nil || count != 0 || len(events.events) != 0 {
		t.Fatalf("RunDue() = %d, %v; events=%+v", count, err, events.events)
	}
}
