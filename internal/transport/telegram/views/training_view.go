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
	energyCost := domain.TrainingEnergyCost(energy)
	canTrain = energyCost > 0

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

	energyLine := "Энергия: " + strconv.Itoa(energy) + "/100 (минимум для тренировки: " + strconv.Itoa(domain.TrainingMinEnergy) + ")\n"
	if canTrain {
		energyLine = "Энергия: " + strconv.Itoa(energy) + "/100 (стоимость сейчас: " + strconv.Itoa(energyCost) + ")\n"
	}

	text = "Тренировка\n" +
		"Имя: " + c.Name + "\n" +
		"Уровень: " + strconv.Itoa(c.Level) + " (XP: " + strconv.FormatInt(c.XP, 10) + "/" + strconv.FormatInt(int64(c.Level)*100, 10) + ")\n" +
		energyLine +
		"Эффективность: " + strconv.Itoa(effPercent) + "%\n" +
		"Коэффициент XP: " + strconv.Itoa(effPercent) + "%" + suffix + "\n"

	return text, canTrain
}

func FormatTrainingResultText(cat *domain.Cat, res domain.TrainResult, generatedNarrative string) string {
	catName := ""
	trait := domain.Trait("")
	if cat != nil {
		catName = cat.Name
		trait = cat.Trait
	}
	switch res.Outcome {
	case domain.TrainingNotEnoughEnergy:
		if strings.TrimSpace(catName) == "" {
			catName = "Кот"
		}
		return "⚡ " + catName + ": нужно больше энергии для тренировки. Минимум: " + strconv.Itoa(domain.TrainingMinEnergy) + "."

	default:
		if strings.TrimSpace(catName) == "" {
			catName = "Кот"
		}
		story := strings.TrimSpace(generatedNarrative)
		if story == "" {
			story = narrative.HuntStory(res.Encounter, trait, res.Flavor)
		}
		msg := "🐾 " + catName + " " + story + "\n\n" +
			"⚡ −" + strconv.Itoa(res.EnergyCost) + " энергии\n" +
			"⭐ +" + strconv.FormatInt(res.XPGain, 10) + " XP\n" +
			"🪙 +" + strconv.FormatInt(res.CoinsGain, 10) + " монет"
		if res.Crit {
			msg += "\n✨ КРИТ! x2 XP"
		}
		if res.EffPercent < 100 {
			msg += "\nЭффективность: " + strconv.Itoa(res.EffPercent) + "%"
		}
		if res.LeveledUp > 0 {
			msg += "\n🎉 Уровень повышен!"

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
