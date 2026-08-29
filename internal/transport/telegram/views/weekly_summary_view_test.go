package views

import (
	"strings"
	"testing"

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
			UniqueParticipants: 4, TotalChoices: 7, FishTotal: 29, SecretsFound: 1,
		},
		CatOfWeek: cat, TopScout: cat, Narrative: ai.Generation{Text: "Неделя была подозрительно рыбной."},
	})
	for _, want := range []string{"Двор «Друзья»", "События: 3", "общий улов: 29", "Кот недели: Сметана", "Лучший разведчик: Сметана", "подозрительно рыбной"} {
		if !strings.Contains(text, want) {
			t.Fatalf("FormatYardWeeklySummary() = %q, missing %q", text, want)
		}
	}
}

func TestFormatYardWeeklySummaryExplainsEmptyWeek(t *testing.T) {
	t.Parallel()
	text := FormatYardWeeklySummary(app.YardWeeklySummaryResult{Summary: domain.YardWeeklySummary{YardName: "Тихий"}})
	if !strings.Contains(text, "нет завершённых событий") || !strings.Contains(text, "/event") {
		t.Fatalf("FormatYardWeeklySummary() = %q", text)
	}
}
