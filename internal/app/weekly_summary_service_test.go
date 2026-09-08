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
		EventsResolved: 3, SuccessfulEvents: 2, YardScore: 37, SecretsFound: 1,
		Cats: []domain.YardWeeklyCatStats{
			{CatID: 42, CatName: "Барсик", EventsParticipated: 3, StealChoices: 3, Contribution: 30, MVPCount: 1, Fights: 7, Wins: 4, RivalryGained: 7, TrainingEnergySpent: 540},
			{CatID: 43, CatName: "Сметана", EventsParticipated: 2, ScoutChoices: 2, Contribution: 35, MVPCount: 1, Fights: 5, Wins: 4, TrainingEnergySpent: 630},
			{CatID: 44, CatName: "Батон", EventsParticipated: 2, DistractChoices: 2, Contribution: 18, Fights: 2, Wins: 1, TrainingEnergySpent: 180},
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
	if result.CatOfWeek == nil || result.CatOfWeek.CatName != "Барсик" || result.CatOfWeek.TotalPoints != 20 {
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
	if !strings.Contains(result.Narrative.Text, "37") || !strings.Contains(result.Narrative.Text, "Барсик") {
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

func TestWeeklyScoresAreNormalizedAndTitlesAreUnique(t *testing.T) {
	t.Parallel()
	result := buildWeeklyHighlights(domain.YardWeeklySummary{
		EventsResolved: 4,
		Cats: []domain.YardWeeklyCatStats{
			{CatID: 1, CatName: "Барсик", Fights: 7, Wins: 5, EventsParticipated: 4, TrainingEnergySpent: 540},
			{CatID: 2, CatName: "Батон", Fights: 7, Wins: 3, EventsParticipated: 3, TrainingEnergySpent: 630},
			{CatID: 3, CatName: "Сметана", Fights: 5, Wins: 4, EventsParticipated: 2, TrainingEnergySpent: 360, MVPCount: 2},
		},
	})
	if got := result.Summary.Cats[0]; got.CatID != 1 || got.ArenaPoints != 7 || got.YardPoints != 7 || got.TrainingPoints != 6 || got.TotalPoints != 20 {
		t.Fatalf("leader = %+v", got)
	}
	byID := map[int64]domain.YardWeeklyCatStats{}
	seenTitles := map[string]bool{}
	for _, cat := range result.Summary.Cats {
		byID[cat.CatID] = cat
		if cat.WeeklyTitle != "" {
			if seenTitles[cat.WeeklyTitle] {
				t.Fatalf("title %q assigned twice", cat.WeeklyTitle)
			}
			seenTitles[cat.WeeklyTitle] = true
		}
	}
	if byID[2].YardPoints != 5 || byID[3].YardPoints != 4 || byID[2].TrainingPoints != 7 {
		t.Fatalf("normalized cats = %+v", byID)
	}
}

func TestWeeklyPeriodStartsOnMonday(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 4, 13, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
	want := time.Date(2026, time.August, 31, 0, 0, 0, 0, now.Location())
	if got := weeklyPeriodStart(now); !got.Equal(want) {
		t.Fatalf("weeklyPeriodStart() = %v, want %v", got, want)
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
		stubYardEvents{weekly: domain.YardWeeklySummary{EventsResolved: 1, YardScore: 5, Cats: []domain.YardWeeklyCatStats{{CatID: 1, CatName: "Кот", EventsParticipated: 1}}}}, quota,
		ai.NewGateway(nil, ai.NewFallbackProvider(), nil, time.Second), fakeClock{t: time.Unix(123, 0)}, events,
	)

	result, err := service.Build(context.Background(), -100, 7, "request:week:limited")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !result.AIRateLimited || result.Summary.YardScore != 5 || result.Narrative.Text != "" || len(events.events) != 1 {
		t.Fatalf("result/events = %+v/%+v", result, events.events)
	}
}

func TestWeeklyTitlesPrioritiesAndRecords(t *testing.T) {
	cats := []domain.YardWeeklyCatStats{
		{CatID: 1, TotalPoints: 21, Fights: 7, Wins: 7, MVPCount: 9},
		{CatID: 2, TotalPoints: 18, Fights: 7, TrainingEnergySpent: 900},
		{CatID: 3, TotalPoints: 17, MVPCount: 8, RivalryGained: 10},
		{CatID: 4, TotalPoints: 16, TrainingEnergySpent: 800},
		{CatID: 5, TotalPoints: 15, Fights: 7, Wins: 3},
		{CatID: 6, TotalPoints: 14, EventsParticipated: 3},
		{CatID: 7, TotalPoints: 0, TrainingEnergySpent: 1},
		{CatID: 8, TotalPoints: 0},
	}
	assignWeeklyTitles(cats, 3)
	want := []string{"🗿 Сигма-кот", "🥲 Пакет для битья", "😾 Ты с какого лотка?", "", "🥊 Лапами объясню", "🏘 В каждой бочке кот", "🛋 Я чисто посмотреть", ""}
	for i, cat := range cats {
		if cat.WeeklyTitle != want[i] {
			t.Errorf("cat %d title = %q, want %q", cat.CatID, cat.WeeklyTitle, want[i])
		}
	}
	// Rebuilding a summary clears stale titles and is deterministic.
	assignWeeklyTitles(cats, 3)
	for i, cat := range cats {
		if cat.WeeklyTitle != want[i] {
			t.Errorf("repeated title = %q", cat.WeeklyTitle)
		}
	}
}

func TestWeeklyTrainingAndMVPRecordTitles(t *testing.T) {
	cats := []domain.YardWeeklyCatStats{
		{CatID: 1, TotalPoints: 21},
		{CatID: 2, TotalPoints: 18, MVPCount: 3},
		{CatID: 3, TotalPoints: 17, TrainingEnergySpent: 630},
	}
	assignWeeklyTitles(cats, 3)
	if cats[1].WeeklyTitle != "🏆 Всё на мне, мяу" || cats[2].WeeklyTitle != "🏋️ Шкаф с усами" {
		t.Fatalf("titles = %+v", cats)
	}
}

func TestWeeklyEligibilityAndScoreBounds(t *testing.T) {
	three, zero := 3, 0
	result := buildWeeklyHighlights(domain.YardWeeklySummary{EventsResolved: 10, Cats: []domain.YardWeeklyCatStats{
		{CatID: 1, AvailableEvents: &three, EventsParticipated: 3, Fights: 50, TrainingEnergySpent: 9000},
		{CatID: 2, AvailableEvents: &zero, EventsParticipated: 0, Fights: -1, TrainingEnergySpent: -90},
	}})
	if c := result.Summary.Cats[0]; c.TotalPoints != 21 || c.YardPoints != 7 {
		t.Fatalf("late member score = %+v", c)
	}
	if c := result.Summary.Cats[1]; c.TotalPoints != 0 || c.YardPoints != 0 {
		t.Fatalf("zero opportunities = %+v", c)
	}
	cats := []domain.YardWeeklyCatStats{{CatID: 1}, {CatID: 2, EventsParticipated: 1}}
	assignWeeklyTitles(cats, 1)
	if cats[1].WeeklyTitle == "🏘 В каждой бочке кот" {
		t.Fatal("one event must not award participation title")
	}
}
