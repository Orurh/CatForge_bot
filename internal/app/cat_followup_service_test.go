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

type stubCatMessageRefs struct {
	ref        *domain.CatMessageReference
	claim      bool
	err        error
	remembered *domain.CatMessageReference
}

func (s *stubCatMessageRefs) Remember(_ context.Context, chatID int64, messageID int, catID int64, expiresAt time.Time) error {
	s.remembered = &domain.CatMessageReference{
		TelegramChatID: chatID, TelegramMessageID: messageID, CatID: catID, ExpiresAt: expiresAt,
	}
	return s.err
}

func (s *stubCatMessageRefs) Claim(context.Context, int64, int, time.Time) (*domain.CatMessageReference, bool, error) {
	return s.ref, s.claim, s.err
}

func TestCatFollowupRepliesAsRememberedCat(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_000, 0)
	refs := &stubCatMessageRefs{
		ref:   &domain.CatMessageReference{TelegramChatID: -100, TelegramMessageID: 55, CatID: 42},
		claim: true,
	}
	yard := &domain.Yard{
		ID: 9, TelegramChatID: -100, HumorMode: domain.HumorBold,
		AutoMessagesEnabled: true, MaxAutoMessagesDay: 2,
	}
	yards := stubYards{
		yard: yard,
		members: []domain.YardMember{{
			YardID: 9, UserID: 7, CatID: 42, CatName: "Барсик",
			Breed: domain.BreedBengal, Trait: domain.TraitBully, Level: 4,
		}},
		relationships: []domain.CatRelationship{{
			CatAID: 42, CatBID: 50, CatAName: "Барсик", CatBName: "Батон", Rivalry: 8,
		}},
	}
	personalities := stubPersonalities{personality: &domain.CatPersonality{
		CatID: 42, Trait: domain.TraitBully, SpeechStyle: domain.DefaultSpeechStyle(domain.TraitBully), AutoSpeakEnabled: true,
	}}
	quota := &stubAIQuota{allowed: true}
	events := &fakeEvents{}
	service := NewCatFollowupService(
		refs, yards, personalities, quota,
		ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second), fakeClock{t: now}, events,
	)

	result, matched, err := service.Reply(context.Background(), 81, -100, 55, 77, "🐆 Барсик: Батон опять проиграл", "зато я всё видел")
	if err != nil || !matched {
		t.Fatalf("Reply() matched/error = %v/%v", matched, err)
	}
	if result == nil || result.Cat.ID != 42 || result.Cat.Name != "Барсик" || !strings.Contains(result.Generation.Text, "Батон") {
		t.Fatalf("unexpected result: %+v", result)
	}
	if quota.calls != 1 {
		t.Fatalf("AI quota calls = %d, want 1", quota.calls)
	}
	if len(events.events) != 2 || events.events[0].Kind != GameEventHumanRepliedToCat || events.events[1].Kind != GameEventCatFollowupGenerated {
		t.Fatalf("unexpected events: %+v", events.events)
	}
}

func TestCatFollowupSilentlyHonorsAutoSpeakAndQuietMode(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_000, 0)
	tests := []struct {
		name        string
		yard        *domain.Yard
		personality *domain.CatPersonality
	}{
		{
			name:        "yard quiet",
			yard:        &domain.Yard{ID: 9, AutoMessagesEnabled: true, MaxAutoMessagesDay: 2, QuietUntil: now.Add(time.Hour)},
			personality: &domain.CatPersonality{CatID: 42, AutoSpeakEnabled: true},
		},
		{
			name:        "cat autospeak disabled",
			yard:        &domain.Yard{ID: 9, AutoMessagesEnabled: true, MaxAutoMessagesDay: 2},
			personality: &domain.CatPersonality{CatID: 42, AutoSpeakEnabled: false},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			refs := &stubCatMessageRefs{ref: &domain.CatMessageReference{CatID: 42}, claim: true}
			quota := &stubAIQuota{allowed: true}
			service := NewCatFollowupService(
				refs,
				stubYards{yard: test.yard, members: []domain.YardMember{{CatID: 42, CatName: "Барсик"}}},
				stubPersonalities{personality: test.personality}, quota,
				ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second), fakeClock{t: now}, nil,
			)
			result, matched, err := service.Reply(context.Background(), 1, -100, 55, 77, "old", "reply")
			if err != nil || !matched || result != nil {
				t.Fatalf("Reply() = result=%+v matched=%v err=%v", result, matched, err)
			}
			if quota.calls != 0 {
				t.Fatalf("AI quota calls = %d, want 0", quota.calls)
			}
		})
	}
}

func TestCatFollowupRejectsUsedReferenceAndRateLimit(t *testing.T) {
	t.Parallel()
	service := NewCatFollowupService(
		&stubCatMessageRefs{}, stubYards{}, stubPersonalities{}, &stubAIQuota{allowed: true},
		ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second), fakeClock{}, nil,
	)
	if result, matched, err := service.Reply(context.Background(), 1, -100, 55, 77, "old", "reply"); err != nil || matched || result != nil {
		t.Fatalf("unclaimed Reply() = result=%+v matched=%v err=%v", result, matched, err)
	}

	quota := &stubAIQuota{allowed: false}
	service = NewCatFollowupService(
		&stubCatMessageRefs{ref: &domain.CatMessageReference{CatID: 42}, claim: true},
		stubYards{
			yard:    &domain.Yard{ID: 9, AutoMessagesEnabled: true, MaxAutoMessagesDay: 2},
			members: []domain.YardMember{{CatID: 42, CatName: "Барсик"}},
		},
		stubPersonalities{personality: &domain.CatPersonality{CatID: 42, AutoSpeakEnabled: true}}, quota,
		ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second), fakeClock{}, nil,
	)
	if _, matched, err := service.Reply(context.Background(), 1, -100, 55, 77, "old", "reply"); !matched || !errors.Is(err, domain.ErrAIRateLimited) {
		t.Fatalf("rate-limited Reply() matched/error = %v/%v", matched, err)
	}
}

func TestCatFollowupRememberUsesFortyEightHourTTL(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_000, 0)
	refs := &stubCatMessageRefs{}
	service := NewCatFollowupService(refs, nil, nil, nil, nil, fakeClock{t: now}, nil)
	if err := service.Remember(context.Background(), -100, 55, 42); err != nil {
		t.Fatalf("Remember() error = %v", err)
	}
	if refs.remembered == nil || !refs.remembered.ExpiresAt.Equal(now.Add(48*time.Hour)) {
		t.Fatalf("remembered ref = %+v", refs.remembered)
	}
}
