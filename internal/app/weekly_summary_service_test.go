package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
)

func TestWeeklySummaryBuildsDeterministicHighlightsAndNarrative(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	yard := &domain.Yard{ID: 9, TelegramChatID: -100, Name: "Друзья"}
	weekly := domain.YardWeeklySummary{
		EventsResolved: 3, SuccessfulEvents: 2, FishTotal: 37, SecretsFound: 1,
		Cats: []domain.YardWeeklyCatStats{
			{CatID: 42, CatName: "Барсик", EventsParticipated: 3, StealChoices: 3, Contribution: 30, FishReward: 14, MVPCount: 1},
			{CatID: 43, CatName: "Сметана", EventsParticipated: 2, ScoutChoices: 2, Contribution: 35, FishReward: 12, MVPCount: 1},
			{CatID: 44, CatName: "Батон", EventsParticipated: 2, DistractChoices: 2, Contribution: 18, FishReward: 11},
		},
	}
	quota := &stubAIQuota{allowed: true}
	events := &fakeEvents{}
	service := NewWeeklySummaryService(
		stubYards{yard: yard}, stubYardEvents{weekly: weekly}, quota,
		ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second), fakeClock{t: now}, events,
	)

	result, err := service.Build(context.Background(), -100, 7, "request:week:1")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if result.CatOfWeek == nil || result.CatOfWeek.CatName != "Сметана" {
		t.Fatalf("CatOfWeek = %+v", result.CatOfWeek)
	}
	if result.TopTroublemaker == nil || result.TopTroublemaker.CatName != "Барсик" {
		t.Fatalf("TopTroublemaker = %+v", result.TopTroublemaker)
	}
	if result.TopScout == nil || result.TopScout.CatName != "Сметана" {
		t.Fatalf("TopScout = %+v", result.TopScout)
	}
	if result.TopDistractor == nil || result.TopDistractor.CatName != "Батон" {
		t.Fatalf("TopDistractor = %+v", result.TopDistractor)
	}
	if result.Summary.TotalChoices != 7 || result.Summary.UniqueParticipants != 3 {
		t.Fatalf("summary counts = %+v", result.Summary)
	}
	if !strings.Contains(result.Narrative.Text, "37") || !strings.Contains(result.Narrative.Text, "Сметана") {
		t.Fatalf("narrative = %+v", result.Narrative)
	}
	if quota.calls != 1 || len(events.events) != 1 || events.events[0].Kind != GameEventYardWeeklySummary {
		t.Fatalf("quota/events = %d/%+v", quota.calls, events.events)
	}
}

func TestWeeklySummaryUsesStableCatIDAsFinalTieBreaker(t *testing.T) {
	t.Parallel()
	result := buildWeeklyHighlights(domain.YardWeeklySummary{Cats: []domain.YardWeeklyCatStats{
		{CatID: 9, CatName: "Поздний", MVPCount: 1, Contribution: 10},
		{CatID: 3, CatName: "Ранний", MVPCount: 1, Contribution: 10},
	}})
	if result.CatOfWeek == nil || result.CatOfWeek.CatID != 3 {
		t.Fatalf("CatOfWeek = %+v", result.CatOfWeek)
	}
}

func TestWeeklySummaryWithoutResolvedEventsSkipsAIAndAnalyticsEvent(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	quota := &stubAIQuota{allowed: true}
	events := &fakeEvents{}
	service := NewWeeklySummaryService(
		stubYards{yard: &domain.Yard{ID: 9, TelegramChatID: -100, Name: "Тихий двор"}},
		stubYardEvents{}, quota, ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second), fakeClock{t: now}, events,
	)

	result, err := service.Build(context.Background(), -100, 7, "request:week:empty")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if result.Summary.EventsResolved != 0 || result.Narrative.Text != "" || quota.calls != 0 || len(events.events) != 0 {
		t.Fatalf("unexpected result/quota/events: %+v/%d/%+v", result, quota.calls, events.events)
	}
}

func TestWeeklySummaryStillReturnsStructuredFactsWhenAIQuotaIsExhausted(t *testing.T) {
	t.Parallel()
	quota := &stubAIQuota{allowed: false}
	events := &fakeEvents{}
	service := NewWeeklySummaryService(
		stubYards{yard: &domain.Yard{ID: 9, TelegramChatID: -100, Name: "Двор"}},
		stubYardEvents{weekly: domain.YardWeeklySummary{EventsResolved: 1, FishTotal: 5}}, quota,
		ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second), fakeClock{t: time.Unix(123, 0)}, events,
	)

	result, err := service.Build(context.Background(), -100, 7, "request:week:limited")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !result.AIRateLimited || result.Summary.FishTotal != 5 || result.Narrative.Text != "" || len(events.events) != 1 {
		t.Fatalf("result/events = %+v/%+v", result, events.events)
	}
}
