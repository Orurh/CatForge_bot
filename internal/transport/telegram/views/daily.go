package views

import (
	"fmt"
	"time"

	"catforge/internal/domain"
)


func FormatDailyScreen(cat *domain.Cat, v domain.DailyView, now time.Time) string {
	name := "Кот"
	if cat != nil && cat.Name != "" {
		name = cat.Name
	}

	if v.CanClaim {
		return fmt.Sprintf(
			"🎁 Ежедневная награда\n\n"+
				"Кот: %s\n"+
				"Серия: %d дн.\n\n"+
				"Награда доступна сегодня.\n"+
				"Нажми кнопку ниже, чтобы получить XP и энергию.",
			name, v.Streak,
		)
	}

	d := v.NextAt.Sub(now)
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf(
		"🎁 Ежедневная награда\n\n"+
			"Кот: %s\n"+
			"Серия: %d дн.\n\n"+
			"Сегодня уже получено.\n"+
			"Следующая будет через %s.",
		name, v.Streak, formatHM(d),
	)
}

func FormatDailyClaimResult(cat *domain.Cat, res domain.DailyClaimResult, now time.Time) string {
	name := "Кот"
	if cat != nil && cat.Name != "" {
		name = cat.Name
	}

	switch res.Outcome {
	case domain.DailyClaimAlreadyClaimed:
		d := res.NextAt.Sub(now)
		if d < 0 {
			d = 0
		}
		return fmt.Sprintf("🎁 %s: награда уже получена сегодня.\nСледующая через %s.", name, formatHM(d))
	default:
		return fmt.Sprintf(
			"🎁 %s получает награду дня!\n\n"+
				"+%d XP\n"+
				"+%d энергии\n"+
				"Серия: %d дн.",
			name, res.XPGain, res.EnergyGain, res.Streak,
		)
	}
}

func formatHM(d time.Duration) string {
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	return fmt.Sprintf("%dч %dм", h, m)
}