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

const (
	weeklyCategoryMax        = 7
	weeklyTrainingEnergyStep = 90
)

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
	periodStart := weeklyPeriodStart(GameTime(periodEnd))
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
	if len(summary.Cats) == 0 && summary.EventsResolved == 0 {
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

func weeklyPeriodStart(now time.Time) time.Time {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	daysSinceMonday := (int(start.Weekday()) + 6) % 7
	return start.AddDate(0, 0, -daysSinceMonday)
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
		candidate.ArenaPoints = max(0, min(candidate.Fights, weeklyCategoryMax))
		candidate.TrainingPoints = max(0, min(candidate.TrainingEnergySpent/weeklyTrainingEnergyStep, weeklyCategoryMax))
		available := weeklyAvailableEvents(*candidate, result.Summary.EventsResolved)
		candidate.YardPoints = 0
		if available > 0 {
			candidate.YardPoints = min(
				max(0, (candidate.EventsParticipated*weeklyCategoryMax+available/2)/available),
				weeklyCategoryMax,
			)
		}
		candidate.TotalPoints = candidate.ArenaPoints + candidate.YardPoints + candidate.TrainingPoints
		if candidate.RivalryGained > 0 && betterWeeklyCat(candidate, result.TopTroublemaker,
			func(cat *domain.YardWeeklyCatStats) []int {
				return []int{cat.RivalryGained, cat.Fights, cat.Wins}
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
	topTroublemakerID, topScoutID, topDistractorID := weeklyCatID(result.TopTroublemaker), weeklyCatID(result.TopScout), weeklyCatID(result.TopDistractor)
	sort.SliceStable(result.Summary.Cats, func(i, j int) bool {
		left, right := result.Summary.Cats[i], result.Summary.Cats[j]
		if left.TotalPoints != right.TotalPoints {
			return left.TotalPoints > right.TotalPoints
		}
		if left.Wins != right.Wins {
			return left.Wins > right.Wins
		}
		if left.MVPCount != right.MVPCount {
			return left.MVPCount > right.MVPCount
		}
		if left.Contribution != right.Contribution {
			return left.Contribution > right.Contribution
		}
		return left.CatID < right.CatID
	})
	if len(result.Summary.Cats) > 0 {
		result.CatOfWeek = &result.Summary.Cats[0]
	}
	assignWeeklyTitles(result.Summary.Cats, result.Summary.EventsResolved)
	// Rebind pointers after sorting the backing slice.
	result.TopTroublemaker = findWeeklyCat(result.Summary.Cats, topTroublemakerID)
	result.TopScout = findWeeklyCat(result.Summary.Cats, topScoutID)
	result.TopDistractor = findWeeklyCat(result.Summary.Cats, topDistractorID)
	return result
}

func weeklyCatID(cat *domain.YardWeeklyCatStats) int64 {
	if cat == nil {
		return 0
	}
	return cat.CatID
}

func findWeeklyCat(cats []domain.YardWeeklyCatStats, catID int64) *domain.YardWeeklyCatStats {
	if catID == 0 {
		return nil
	}
	for index := range cats {
		if cats[index].CatID == catID {
			return &cats[index]
		}
	}
	return nil
}

// A nil count is used by historical summaries without per-cat eligibility.
// An explicit zero means the cat had no opportunity to join an event.
func weeklyAvailableEvents(cat domain.YardWeeklyCatStats, fallback int) int {
	if cat.AvailableEvents != nil {
		return max(0, *cat.AvailableEvents)
	}
	return max(0, fallback)
}

func assignWeeklyTitles(cats []domain.YardWeeklyCatStats, availableEvents int) {
	if len(cats) == 0 {
		return
	}
	for i := range cats {
		cats[i].WeeklyTitle = ""
	}
	assign := func(index int, title string) {
		if index >= 0 && cats[index].WeeklyTitle == "" {
			cats[index].WeeklyTitle = title
		}
	}
	// Records are compared against every cat, including the Sigma. A runner-up
	// must not be called the week's record holder because the winner has a title.
	record := func(metric func(domain.YardWeeklyCatStats) int, least bool) int {
		chosen := -1
		for i, cat := range cats {
			value := metric(cat)
			if value < 0 {
				continue
			}
			if chosen < 0 || (!least && value > metric(cats[chosen])) ||
				(least && value < metric(cats[chosen])) ||
				(value == metric(cats[chosen]) && cat.CatID < cats[chosen].CatID) {
				chosen = i
			}
		}
		return chosen
	}
	positive := func(value int) int {
		if value <= 0 {
			return -1
		}
		return value
	}
	eligible := func(matches func(domain.YardWeeklyCatStats) bool) int {
		for i, cat := range cats {
			if cat.WeeklyTitle == "" && matches(cat) {
				return i
			}
		}
		return -1
	}
	assign(0, "🗿 Сигма-кот")
	assign(eligible(func(c domain.YardWeeklyCatStats) bool { return c.Fights >= 7 && c.Wins == 0 }), "🥲 Пакет для битья")
	assign(record(func(c domain.YardWeeklyCatStats) int { return positive(c.MVPCount) }, false), "🏆 Всё на мне, мяу")
	assign(record(func(c domain.YardWeeklyCatStats) int { return positive(c.RivalryGained) }, false), "😾 Ты с какого лотка?")
	assign(record(func(c domain.YardWeeklyCatStats) int { return positive(c.TrainingEnergySpent) }, false), "🏋️ Шкаф с усами")
	assign(eligible(func(c domain.YardWeeklyCatStats) bool { return c.Fights >= 7 }), "🥊 Лапами объясню")
	assign(eligible(func(c domain.YardWeeklyCatStats) bool {
		count := weeklyAvailableEvents(c, availableEvents)
		return count >= 3 && c.EventsParticipated >= count
	}), "🏘 В каждой бочке кот")
	assign(record(func(c domain.YardWeeklyCatStats) int {
		if c.Fights <= 0 && c.EventsParticipated <= 0 && c.TrainingEnergySpent <= 0 {
			return -1
		}
		return c.TotalPoints
	}, true), "🛋 Я чисто посмотреть")
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
		FailedEvents: summary.FailedEvents, PartialEvents: summary.PartialEvents,
		SuccessfulEvents: summary.SuccessfulEvents, ExceptionalEvents: summary.ExceptionalEvents,
		TotalChoices:       summary.TotalChoices,
		UniqueParticipants: summary.UniqueParticipants, YardScore: summary.YardScore, SecretsFound: summary.SecretsFound,
		CatOfWeek: catName(result.CatOfWeek), TopTroublemaker: catName(result.TopTroublemaker),
		TopScout: catName(result.TopScout), TopDistractor: catName(result.TopDistractor),
	}
	facts := []string{
		"yard_name=" + strconv.Quote(summary.YardName),
		"events_resolved=" + strconv.Itoa(summary.EventsResolved),
		"successful_events=" + strconv.Itoa(summary.SuccessfulEvents),
		"failed_events=" + strconv.Itoa(summary.FailedEvents),
		"partial_events=" + strconv.Itoa(summary.PartialEvents),
		"exceptional_events=" + strconv.Itoa(summary.ExceptionalEvents),
		"total_choices=" + strconv.Itoa(summary.TotalChoices),
		"unique_participants=" + strconv.Itoa(summary.UniqueParticipants),
		"yard_score=" + strconv.Itoa(summary.YardScore),
		"secrets_found=" + strconv.Itoa(summary.SecretsFound),
		"cat_of_week=" + strconv.Quote(context.CatOfWeek),
		"top_troublemaker=" + strconv.Quote(context.TopTroublemaker),
		"top_scout=" + strconv.Quote(context.TopScout),
		"top_distractor=" + strconv.Quote(context.TopDistractor),
	}
	topCats := summary.Cats
	if len(topCats) > 10 {
		topCats = topCats[:10]
	}
	for _, cat := range topCats {
		context.Cats = append(context.Cats, ai.WeeklyCatContext{
			CatName: cat.CatName, EventsParticipated: cat.EventsParticipated,
			StealChoices: cat.StealChoices, DistractChoices: cat.DistractChoices, ScoutChoices: cat.ScoutChoices,
			Contribution: cat.Contribution, MVPCount: cat.MVPCount,
			TrainingEnergySpent: cat.TrainingEnergySpent, TrainingXP: cat.TrainingXP,
			Fights: cat.Fights, Wins: cat.Wins, Losses: cat.Losses,
			ArenaPoints: cat.ArenaPoints, YardPoints: cat.YardPoints,
			TrainingPoints: cat.TrainingPoints, TotalPoints: cat.TotalPoints, WeeklyTitle: cat.WeeklyTitle,
		})
		facts = append(facts, fmt.Sprintf(
			"cat=%q arena=%d yard=%d training=%d total=%d title=%q fights=%d wins=%d losses=%d training_energy=%d training_xp=%d events=%d contribution=%d mvp=%d rivalry_gained=%d",
			cat.CatName, cat.ArenaPoints, cat.YardPoints, cat.TrainingPoints, cat.TotalPoints, cat.WeeklyTitle,
			cat.Fights, cat.Wins, cat.Losses, cat.TrainingEnergySpent, cat.TrainingXP,
			cat.EventsParticipated, cat.Contribution, cat.MVPCount, cat.RivalryGained,
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
		EventsResolved: summary.EventsResolved, FailedEvents: summary.FailedEvents, PartialEvents: summary.PartialEvents,
		SuccessfulEvents: summary.SuccessfulEvents, ExceptionalEvents: summary.ExceptionalEvents,
		Participants: summary.UniqueParticipants, YardScore: summary.YardScore, SecretsFound: summary.SecretsFound,
		CatOfWeek: catName(result.CatOfWeek), TopTroublemaker: catName(result.TopTroublemaker),
		TopScout: catName(result.TopScout), TopDistractor: catName(result.TopDistractor),
		Narrative: result.Narrative.Text, Provider: result.Narrative.Provider,
		Model: result.Narrative.Model, Fallback: result.Narrative.Fallback, AIRateLimited: result.AIRateLimited,
		Cats: summary.Cats,
	}
}

func catName(cat *domain.YardWeeklyCatStats) string {
	if cat == nil {
		return ""
	}
	return cat.CatName
}
