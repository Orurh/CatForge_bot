package views

import (
	"strconv"
	"strings"
	"time"

	"catforge/internal/domain"
	"catforge/internal/transport/telegram/narrative"
)

func FormatTrainingScreen(c *domain.Cat, now time.Time) (text string, canTrain bool) {
	energy := domain.RegenEnergy(c.Energy, c.EnergyUpdatedAt, now)
	canTrain = energy >= domain.TrainingMinEnergy

	_, effPercent := domain.TrainingEfficiency(c.LastTrainAt, now)

	suffix := ""
	if !c.LastTrainAt.IsZero() {
		delta := now.Sub(c.LastTrainAt)
		if delta > 0 && delta < domain.TrainingEfficiencyWindow {
			left := domain.TrainingEfficiencyWindow - delta
			sec := int(left.Seconds() + 0.5)
			if sec < 1 {
				sec = 1
			}
			suffix = " (до 100% через " + strconv.Itoa(sec) + "с)"
		}
	}

	text = "Охота\n" +
		"Имя: " + c.Name + "\n" +
		"Уровень: " + strconv.Itoa(c.Level) + " (XP: " + strconv.FormatInt(c.XP, 10) + "/" + strconv.FormatInt(int64(c.Level)*100, 10) + ")\n" +
		"Энергия: " + strconv.Itoa(energy) + "/100 (нужно минимум: 25)\n" +
		"Эффективность: " + strconv.Itoa(effPercent) + "%\n" +
		"Коэффициент XP: " + strconv.Itoa(effPercent) + "%" + suffix + "\n"

	return text, canTrain
}

func FormatTrainingResultText(catName string, res domain.TrainResult) string {
	switch res.Outcome {
	case domain.TrainingNotEnoughEnergy:
		if strings.TrimSpace(catName) == "" {
			catName = "Кот"
		}
		return "⚡ " + catName + ": нужно больше энергии для охоты. Минимум: 25."

	default:
		if strings.TrimSpace(catName) == "" {
			catName = "Кот"
		}
		story := narrative.HuntStory(res.Encounter, res.Flavor)
		msg := "🐾 " + catName + " " + story + ": +" + strconv.FormatInt(res.XPGain, 10) +
			" XP, -" + strconv.Itoa(res.EnergyCost) + " энергии."
		if res.Crit {
			msg += " ✨ КРИТ! x2 XP"
		}
		if res.EffPercent < 100 {
			msg += " (эффективность: " + strconv.Itoa(res.EffPercent) + "%)"
		}
		if res.LeveledUp > 0 {
			msg += " 🎉 Уровень повышен!"

			// показываем прибавку
			d := res.StatsGained
			parts := make([]string, 0, 4)
			if d.HP != 0 {
				parts = append(parts, "HP +"+strconv.Itoa(d.HP))
			}
			if d.ATK != 0 {
				parts = append(parts, "ATK +"+strconv.Itoa(d.ATK))
			}
			if d.DEF != 0 {
				parts = append(parts, "DEF +"+strconv.Itoa(d.DEF))
			}
			if d.SPD != 0 {
				parts = append(parts, "SPD +"+strconv.Itoa(d.SPD))
			}

			if len(parts) > 0 {
				msg += " (" + strings.Join(parts, ", ") + ")"
			}
		}
		return msg
	}
}
