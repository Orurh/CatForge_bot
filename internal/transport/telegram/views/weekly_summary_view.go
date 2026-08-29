package views

import (
	"strconv"
	"strings"

	"catforge/internal/app"
	"catforge/internal/domain"
)

func FormatYardWeeklySummary(result app.YardWeeklySummaryResult) string {
	summary := result.Summary
	if summary.EventsResolved == 0 {
		return "🏆 Двор «" + summary.YardName + "» за неделю\n\nПока нет завершённых событий. Откройте первое командой /event."
	}

	lines := []string{
		"🏆 Двор «" + summary.YardName + "» за неделю",
		"",
		"События: " + strconv.Itoa(summary.EventsResolved) +
			" · успехи: " + strconv.Itoa(summary.SuccessfulEvents) +
			" · участвовали: " + strconv.Itoa(summary.UniqueParticipants) + " котов",
		"Выборов: " + strconv.Itoa(summary.TotalChoices) +
			" · общий улов: " + strconv.Itoa(summary.FishTotal) + " рыб",
	}
	if summary.SecretsFound > 0 {
		lines = append(lines, "✨ Найдено тайников: "+strconv.Itoa(summary.SecretsFound))
	}
	lines = append(lines, "")
	if result.CatOfWeek != nil {
		lines = append(lines, "🐾 Кот недели: "+weeklyCatName(result.CatOfWeek))
	}
	if result.TopTroublemaker != nil {
		lines = append(lines, "😼 Главный дебошир: "+weeklyCatName(result.TopTroublemaker))
	}
	if result.TopScout != nil {
		lines = append(lines, "🔎 Лучший разведчик: "+weeklyCatName(result.TopScout))
	}
	if result.TopDistractor != nil {
		lines = append(lines, "🛡 Лучший отвлекающий: "+weeklyCatName(result.TopDistractor))
	}
	if narrative := strings.TrimSpace(result.Narrative.Text); narrative != "" {
		lines = append(lines, "", narrative)
	} else if result.AIRateLimited {
		lines = append(lines, "", "Статистика готова; кошачий комментатор в этом часу уже исчерпал лимит реплик.")
	}
	return strings.Join(lines, "\n")
}

func weeklyCatName(cat *domain.YardWeeklyCatStats) string {
	if cat == nil || strings.TrimSpace(cat.CatName) == "" {
		return "не определён"
	}
	return cat.CatName
}
