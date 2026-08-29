package app

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

const weeklySummaryPeriod = 7 * 24 * time.Hour

type WeeklySummaryService struct {
	yards      YardRepository
	eventsRepo YardEventRepository
	quota      AIQuotaRepository
	voice      ai.Generator
	clock      Clock
	events     GameEventSink
}

type YardWeeklySummaryResult struct {
	Summary         domain.YardWeeklySummary
	CatOfWeek       *domain.YardWeeklyCatStats
	TopTroublemaker *domain.YardWeeklyCatStats
	TopScout        *domain.YardWeeklyCatStats
	TopDistractor   *domain.YardWeeklyCatStats
	Narrative       ai.Generation
	AIRateLimited   bool
}

func NewWeeklySummaryService(yards YardRepository, eventsRepo YardEventRepository, quota AIQuotaRepository, voice ai.Generator, clock Clock, events GameEventSink) *WeeklySummaryService {
	return &WeeklySummaryService{yards: yards, eventsRepo: eventsRepo, quota: quota, voice: voice, clock: clock, events: events}
}

func (s *WeeklySummaryService) Build(ctx context.Context, telegramChatID, userID int64, requestKey string) (YardWeeklySummaryResult, error) {
	yard, err := s.yards.GetByTelegramChatID(ctx, telegramChatID)
	if err != nil {
		return YardWeeklySummaryResult{}, err
	}
	periodEnd := s.clock.Now()
	periodStart := periodEnd.Add(-weeklySummaryPeriod)
	summary, err := s.eventsRepo.WeeklySummary(ctx, yard.ID, periodStart, periodEnd)
	if err != nil {
		return YardWeeklySummaryResult{}, err
	}
	summary.YardID = yard.ID
	summary.TelegramChatID = telegramChatID
	summary.YardName = yard.Name
	summary.PeriodStart = periodStart
	summary.PeriodEnd = periodEnd

	result := buildWeeklyHighlights(summary)
	if summary.EventsResolved == 0 {
		return result, nil
	}

	if s.voice != nil {
		allowed := true
		if s.quota != nil {
			allowed, err = s.quota.AllowAIRequest(
				ctx, userID, telegramChatID, periodEnd, requestedAIUserHourlyLimit, requestedAIChatHourlyLimit,
			)
			if err != nil {
				return YardWeeklySummaryResult{}, err
			}
		}
		if allowed {
			result.Narrative, err = s.voice.GenerateWeeklySummary(ctx, weeklySummaryGenerationRequest(result, yard.HumorMode))
			if err != nil {
				return YardWeeklySummaryResult{}, err
			}
		} else {
			result.AIRateLimited = true
		}
	}

	if s.events != nil {
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: requestKey, Kind: GameEventYardWeeklySummary,
			UserID: userID, YardID: yard.ID, OccurredAt: periodEnd,
			ContentVersion: gameengine.CurrentContentVersion, PayloadVersion: GameEventPayloadVersion, Notable: true,
			Payload: weeklySummaryPayload(result),
		})
	}
	return result, nil
}

func WeeklySummaryRequestKey(chatID int64, messageID int) string {
	return fmt.Sprintf("chat:%d:message:%d:yard-weekly-summary", chatID, messageID)
}

func buildWeeklyHighlights(summary domain.YardWeeklySummary) YardWeeklySummaryResult {
	result := YardWeeklySummaryResult{Summary: summary}
	result.Summary.UniqueParticipants = len(result.Summary.Cats)
	result.Summary.TotalChoices = 0
	for i := range result.Summary.Cats {
		candidate := &result.Summary.Cats[i]
		result.Summary.TotalChoices += candidate.EventsParticipated
		if betterWeeklyCat(candidate, result.CatOfWeek,
			func(cat *domain.YardWeeklyCatStats) []int {
				return []int{cat.MVPCount, cat.Contribution, cat.FishReward, cat.EventsParticipated}
			}) {
			result.CatOfWeek = candidate
		}
		if candidate.StealChoices > 0 && betterWeeklyCat(candidate, result.TopTroublemaker,
			func(cat *domain.YardWeeklyCatStats) []int {
				return []int{cat.StealChoices, cat.Contribution, cat.MVPCount}
			}) {
			result.TopTroublemaker = candidate
		}
		if candidate.ScoutChoices > 0 && betterWeeklyCat(candidate, result.TopScout,
			func(cat *domain.YardWeeklyCatStats) []int {
				return []int{cat.ScoutChoices, cat.Contribution, cat.MVPCount}
			}) {
			result.TopScout = candidate
		}
		if candidate.DistractChoices > 0 && betterWeeklyCat(candidate, result.TopDistractor,
			func(cat *domain.YardWeeklyCatStats) []int {
				return []int{cat.DistractChoices, cat.Contribution, cat.MVPCount}
			}) {
			result.TopDistractor = candidate
		}
	}
	return result
}

func betterWeeklyCat(candidate, current *domain.YardWeeklyCatStats, score func(*domain.YardWeeklyCatStats) []int) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	candidateScore, currentScore := score(candidate), score(current)
	for i := range candidateScore {
		if candidateScore[i] != currentScore[i] {
			return candidateScore[i] > currentScore[i]
		}
	}
	return candidate.CatID < current.CatID
}

func weeklySummaryGenerationRequest(result YardWeeklySummaryResult, humorMode domain.HumorMode) ai.GenerationRequest {
	summary := result.Summary
	context := ai.WeeklySummaryContext{
		YardName: summary.YardName, EventsResolved: summary.EventsResolved,
		SuccessfulEvents: summary.SuccessfulEvents, TotalChoices: summary.TotalChoices,
		UniqueParticipants: summary.UniqueParticipants, FishTotal: summary.FishTotal, SecretsFound: summary.SecretsFound,
		CatOfWeek: catName(result.CatOfWeek), TopTroublemaker: catName(result.TopTroublemaker),
		TopScout: catName(result.TopScout), TopDistractor: catName(result.TopDistractor),
	}
	facts := []string{
		"yard_name=" + strconv.Quote(summary.YardName),
		"events_resolved=" + strconv.Itoa(summary.EventsResolved),
		"successful_events=" + strconv.Itoa(summary.SuccessfulEvents),
		"total_choices=" + strconv.Itoa(summary.TotalChoices),
		"unique_participants=" + strconv.Itoa(summary.UniqueParticipants),
		"fish_total=" + strconv.Itoa(summary.FishTotal),
		"secrets_found=" + strconv.Itoa(summary.SecretsFound),
		"cat_of_week=" + strconv.Quote(context.CatOfWeek),
		"top_troublemaker=" + strconv.Quote(context.TopTroublemaker),
		"top_scout=" + strconv.Quote(context.TopScout),
		"top_distractor=" + strconv.Quote(context.TopDistractor),
	}
	for _, cat := range summary.Cats {
		context.Cats = append(context.Cats, ai.WeeklyCatContext{
			CatName: cat.CatName, EventsParticipated: cat.EventsParticipated,
			StealChoices: cat.StealChoices, DistractChoices: cat.DistractChoices, ScoutChoices: cat.ScoutChoices,
			Contribution: cat.Contribution, FishReward: cat.FishReward, MVPCount: cat.MVPCount,
		})
		facts = append(facts, fmt.Sprintf(
			"cat=%q events=%d steal=%d distract=%d scout=%d contribution=%d fish=%d mvp=%d",
			cat.CatName, cat.EventsParticipated, cat.StealChoices, cat.DistractChoices,
			cat.ScoutChoices, cat.Contribution, cat.FishReward, cat.MVPCount,
		))
	}
	return ai.GenerationRequest{
		YardID: summary.YardID, HumorMode: humorMode, EventFacts: facts, WeeklySummary: &context,
	}
}

func weeklySummaryPayload(result YardWeeklySummaryResult) YardWeeklySummaryPayload {
	summary := result.Summary
	return YardWeeklySummaryPayload{
		TelegramChatID: summary.TelegramChatID, PeriodStart: summary.PeriodStart, PeriodEnd: summary.PeriodEnd,
		EventsResolved: summary.EventsResolved, SuccessfulEvents: summary.SuccessfulEvents,
		Participants: summary.UniqueParticipants, FishTotal: summary.FishTotal, SecretsFound: summary.SecretsFound,
		CatOfWeek: catName(result.CatOfWeek), TopTroublemaker: catName(result.TopTroublemaker),
		TopScout: catName(result.TopScout), TopDistractor: catName(result.TopDistractor),
		Narrative: result.Narrative.Text, Provider: result.Narrative.Provider,
		Model: result.Narrative.Model, Fallback: result.Narrative.Fallback, AIRateLimited: result.AIRateLimited,
	}
}

func catName(cat *domain.YardWeeklyCatStats) string {
	if cat == nil {
		return ""
	}
	return cat.CatName
}
