package views

import (
	"strings"
	"testing"
	"unicode/utf8"

	"catforge/internal/ai"
	"catforge/internal/app"
	"catforge/internal/domain"
)

func TestFormatYardWeeklySummaryShowsFactsHighlightsAndNarrative(t *testing.T) {
	t.Parallel()
	cat := &domain.YardWeeklyCatStats{CatID: 7, CatName: "Сметана"}
	text := FormatYardWeeklySummary(app.YardWeeklySummaryResult{
		Summary: domain.YardWeeklySummary{
			YardName: "Друзья", EventsResolved: 3, SuccessfulEvents: 2,
			UniqueParticipants: 4, TotalChoices: 7, YardScore: 29, SecretsFound: 1,
			Cats: []domain.YardWeeklyCatStats{{CatID: 7, CatName: "Сметана", ArenaPoints: 5, YardPoints: 7, TrainingPoints: 7, TotalPoints: 19, WeeklyTitle: "🗿 Сигма-кот"}},
		},
		CatOfWeek: cat, TopScout: cat, Narrative: ai.Generation{Text: "Неделя была подозрительно рыбной."},
	})
	for _, want := range []string{
		"🏆 ИТОГИ НЕДЕЛИ\nДвор «Друзья»",
		"📊 СОБЫТИЯ\n├ Всего: 3",
		"🏘 ДВОР\n├ Очки: 29\n└ Тайники: 1",
		"🥇 Сметана\n├ Звание: ",
		"Сигма-кот\n├ ⭐ Всего: 19/21",
		"├ 🥊 Арена: 5/7\n│  ├ Победы: 0\n│  └ Поражения: 0",
		"├ 🏘 События: 7/7\n│  ├ Участие: 0\n│  └ MVP: 0",
		"└ 🐾 Тренировки: 7/7\n   └ Энергия: 0",
		"💬 КОММЕНТАРИЙ КОТОВ\nНеделя была подозрительно рыбной.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("FormatYardWeeklySummary() = %q, missing %q", text, want)
		}
	}
}

func TestFormatYardWeeklySummaryExplainsEmptyWeek(t *testing.T) {
	t.Parallel()
	text := FormatYardWeeklySummary(app.YardWeeklySummaryResult{Summary: domain.YardWeeklySummary{YardName: "Тихий"}})
	if !strings.Contains(text, "Нет тренировок, боёв или участия") {
		t.Fatalf("FormatYardWeeklySummary() = %q", text)
	}
}

func TestFormatYardWeeklySummaryUsesStableVerticalRows(t *testing.T) {
	t.Parallel()
	text := FormatYardWeeklySummary(app.YardWeeklySummaryResult{Summary: domain.YardWeeklySummary{
		YardName: "Двор", EventsResolved: 1,
		Cats: []domain.YardWeeklyCatStats{
			{CatName: "Первый", ArenaPoints: 1, YardPoints: 2, TrainingPoints: 3, TotalPoints: 6},
			{CatName: "Второй", ArenaPoints: 4, YardPoints: 5, TrainingPoints: 6, TotalPoints: 15},
		},
	}})
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "🥊") && strings.Contains(line, "🏘") {
			t.Fatalf("categories were packed into one fragile row: %q", line)
		}
		if strings.Contains(line, "Победы:") && strings.Contains(line, "Поражения:") {
			t.Fatalf("arena values were packed into one fragile row: %q", line)
		}
	}
}

func TestFormatYardWeeklySummaryTopTenFitsTelegramMessage(t *testing.T) {
	t.Parallel()
	cats := make([]domain.YardWeeklyCatStats, 10)
	for index := range cats {
		cats[index] = domain.YardWeeklyCatStats{
			CatID: int64(index + 1), CatName: strings.Repeat("Я", domain.CatNameMaxLen),
			ArenaPoints: 7, YardPoints: 7, TrainingPoints: 7, TotalPoints: 21,
			Wins: 999, Losses: 999, EventsParticipated: 999, MVPCount: 999,
			TrainingEnergySpent: 999_999, WeeklyTitle: "🏘 В каждой бочке кот",
		}
	}
	text := FormatYardWeeklySummary(app.YardWeeklySummaryResult{
		Summary:   domain.YardWeeklySummary{YardName: "Двор", EventsResolved: 999, Cats: cats},
		Narrative: ai.Generation{Text: strings.Repeat("м", 500)},
	})
	if runes := utf8.RuneCountInString(text); runes > 4096 {
		t.Fatalf("weekly summary has %d runes, Telegram limit is 4096", runes)
	}
}
