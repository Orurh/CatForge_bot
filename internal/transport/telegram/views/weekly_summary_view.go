package views

import (
	"strconv"
	"strings"

	"catforge/internal/app"
	"catforge/internal/domain"
)

func FormatYardWeeklySummary(result app.YardWeeklySummaryResult) string {
	summary := result.Summary
	if len(summary.Cats) == 0 && summary.EventsResolved == 0 {
		return weeklySummaryTitle(summary.YardName) + "\n\nПока тихо.\nНет тренировок, боёв или участия в событиях."
	}

	lines := []string{
		weeklySummaryTitle(summary.YardName),
		"",
		"📊 СОБЫТИЯ",
		"├ Всего: " + strconv.Itoa(summary.EventsResolved),
		"├ 💀 Провал: " + strconv.Itoa(summary.FailedEvents),
		"├ 😼 Частично: " + strconv.Itoa(summary.PartialEvents),
		"├ 🏆 Успех: " + strconv.Itoa(summary.SuccessfulEvents),
		"└ ✨ Отлично: " + strconv.Itoa(summary.ExceptionalEvents),
		"",
		"🏘 ДВОР",
		"├ Очки: " + strconv.Itoa(summary.YardScore),
		"└ Тайники: " + strconv.Itoa(summary.SecretsFound),
	}
	if len(summary.Cats) == 0 {
		lines = append(lines, "", "🐾 РЕЙТИНГ КОТОВ", "Пока пуст.")
	} else {
		lines = append(lines, "", "🐾 РЕЙТИНГ КОТОВ", "Максимум — 21 очко")
	}
	limit := min(10, len(summary.Cats))
	for index := 0; index < limit; index++ {
		cat := &summary.Cats[index]
		lines = append(lines, "", weeklyRank(index)+" "+weeklyCatName(cat))
		if cat.WeeklyTitle != "" {
			lines = append(lines, "├ Звание: "+cat.WeeklyTitle)
		}
		lines = append(lines,
			"├ ⭐ Всего: "+strconv.Itoa(cat.TotalPoints)+"/21",
			"├ 🥊 Арена: "+strconv.Itoa(cat.ArenaPoints)+"/7",
			"│  ├ Победы: "+strconv.Itoa(cat.Wins),
			"│  └ Поражения: "+strconv.Itoa(cat.Losses),
			"├ 🏘 События: "+strconv.Itoa(cat.YardPoints)+"/7",
			"│  ├ Участие: "+strconv.Itoa(cat.EventsParticipated),
			"│  └ MVP: "+strconv.Itoa(cat.MVPCount),
			"└ 🐾 Тренировки: "+strconv.Itoa(cat.TrainingPoints)+"/7",
			"   └ Энергия: "+strconv.Itoa(cat.TrainingEnergySpent),
		)
	}
	if narrative := strings.TrimSpace(result.Narrative.Text); narrative != "" {
		lines = append(lines, "", "💬 КОММЕНТАРИЙ КОТОВ", narrative)
	} else if result.AIRateLimited {
		lines = append(lines, "", "💬 КОММЕНТАРИЙ КОТОВ", "Комментатор в этом часу уже исчерпал лимит реплик.")
	}
	return strings.Join(lines, "\n")
}

func weeklySummaryTitle(yardName string) string {
	return "🏆 ИТОГИ НЕДЕЛИ\nДвор «" + yardName + "»"
}

func weeklyRank(index int) string {
	switch index {
	case 0:
		return "🥇"
	case 1:
		return "🥈"
	case 2:
		return "🥉"
	default:
		return strconv.Itoa(index+1) + "."
	}
}

func weeklyCatName(cat *domain.YardWeeklyCatStats) string {
	if cat == nil || strings.TrimSpace(cat.CatName) == "" {
		return "не определён"
	}
	return cat.CatName
}
