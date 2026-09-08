package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
)

type stubPersonalities struct {
	personality *domain.CatPersonality
	err         error
}

func (s stubPersonalities) GetByCatID(context.Context, int64) (*domain.CatPersonality, error) {
	return s.personality, s.err
}
func (s stubPersonalities) SetTrait(context.Context, int64, domain.Trait, string) (*domain.CatPersonality, error) {
	return s.personality, s.err
}
func (s stubPersonalities) SetHumorMode(context.Context, int64, domain.HumorMode) error {
	return s.err
}
func (s stubPersonalities) SetAutoSpeak(context.Context, int64, bool) error { return s.err }

type stubAIQuota struct {
	allowed bool
	err     error
	calls   int
}

func (q *stubAIQuota) AllowAIRequest(context.Context, int64, int64, time.Time, int, int) (bool, error) {
	q.calls++
	return q.allowed, q.err
}

func TestCatReplyServiceGeneratesTypedEvent(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	cat := &domain.Cat{ID: 42, UserID: 7, Name: "Батон", Breed: domain.BreedBritish, Trait: domain.TraitLazy, Level: 3}
	personalities := stubPersonalities{personality: &domain.CatPersonality{CatID: 42, Trait: domain.TraitLazy, SpeechStyle: domain.DefaultSpeechStyle(domain.TraitLazy)}}
	quota := &stubAIQuota{allowed: true}
	events := &fakeEvents{}
	voice := ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second)
	yards := stubYards{
		yard: &domain.Yard{ID: 9, TelegramChatID: -100},
		relationships: []domain.CatRelationship{{
			CatAID: 42, CatBID: 50, CatAName: "Батон", CatBName: "Мурзик", Rivalry: 8,
		}},
	}
	svc := NewCatReplyService(stubCats{cat: cat}, personalities, quota, yards, voice, fakeClock{t: now}, events)

	gotCat, generation, err := svc.Reply(context.Background(), 7, -100, "request:1", "что там Мурзик?")
	if err != nil {
		t.Fatalf("Reply() error = %v", err)
	}
	if gotCat != cat || !strings.Contains(generation.Text, "Мурзик") || !generation.Fallback {
		t.Fatalf("unexpected reply: cat=%+v generation=%+v", gotCat, generation)
	}
	if len(events.events) != 1 || events.events[0].Kind != GameEventCatReplyGenerated || events.events[0].DedupeKey != "request:1" || events.events[0].YardID != 9 {
		t.Fatalf("unexpected events: %+v", events.events)
	}
}

func TestCatReplyServiceRejectsRateLimit(t *testing.T) {
	t.Parallel()
	cat := &domain.Cat{ID: 42, UserID: 7}
	quota := &stubAIQuota{allowed: false}
	svc := NewCatReplyService(stubCats{cat: cat}, stubPersonalities{}, quota, nil, ai.NewGateway(nil, nil, nil, time.Second), fakeClock{}, nil)

	_, _, err := svc.Reply(context.Background(), 7, -100, "request:1", "message")
	if !errors.Is(err, domain.ErrAIRateLimited) {
		t.Fatalf("Reply() error = %v, want ErrAIRateLimited", err)
	}
}

func TestPersonalityServiceGeneratesFirstLineAndEvent(t *testing.T) {
	t.Parallel()
	now := time.Unix(321, 0)
	cat := &domain.Cat{ID: 42, UserID: 7, Name: "Барсик", Breed: domain.BreedBengal, Trait: domain.TraitBully}
	personalities := stubPersonalities{personality: &domain.CatPersonality{CatID: 42, Trait: domain.TraitBully, SpeechStyle: domain.DefaultSpeechStyle(domain.TraitBully)}}
	events := &fakeEvents{}
	voice := ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second)
	svc := NewPersonalityService(personalities, voice, events, fakeClock{t: now})

	generation, err := svc.FirstLine(context.Background(), cat)
	if err != nil {
		t.Fatalf("FirstLine() error = %v", err)
	}
	if generation.Text == "" || len(events.events) != 1 {
		t.Fatalf("generation/events = %+v/%+v", generation, events.events)
	}
	if event := events.events[0]; event.Kind != GameEventFirstPersonalityLine || !event.OccurredAt.Equal(now) {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestPersonalityServicePublishesAutoSpeakSetting(t *testing.T) {
	t.Parallel()
	now := time.Unix(456, 0)
	events := &fakeEvents{}
	svc := NewPersonalityService(stubPersonalities{}, nil, events, fakeClock{t: now})

	if err := svc.SetAutoSpeak(context.Background(), 42, false); err != nil {
		t.Fatalf("SetAutoSpeak() error = %v", err)
	}
	if len(events.events) != 1 {
		t.Fatalf("events = %d, want 1", len(events.events))
	}
	event := events.events[0]
	payload, ok := event.Payload.(CatAutoSpeakChangedPayload)
	if !ok || payload.Enabled || event.Kind != GameEventCatAutoSpeakChanged || event.CatID != 42 || !event.OccurredAt.Equal(now) {
		t.Fatalf("unexpected event: %+v", event)
	}
}
