package views

import (
	"fmt"
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
		"Энергия: " + strconv.Itoa(energy) + "/100 (нужно ≥ 25)\n" +
		"Форма: " + strconv.Itoa(effPercent) + "% (" + mulFromPercent(effPercent) + ")" + suffix + "\n" +
		"Форма влияет на XP: частая охота даёт меньше награды (восстановление ~60с).\n"

	return text, canTrain
}

func mulFromPercent(p int) string {
	switch {
	case p <= 0:
		return "x0.00"
	case p >= 100:
		return "x1.00"
	default:
		return fmt.Sprintf("x%.2f", float64(p)/100.0)
	}
}

func FormatTrainingResultText(cat *domain.Cat, now time.Time, res domain.TrainResult) string {
	catName := "Кот"
	energyNow := 0
	levelNow := 0
	if cat != nil {
		if strings.TrimSpace(cat.Name) != "" {
			catName = cat.Name
		}
		energyNow = domain.RegenEnergy(cat.Energy, cat.EnergyUpdatedAt, now)
		levelNow = cat.Level
	}

	switch res.Outcome {
	case domain.TrainingNotEnoughEnergy:
		return "⚡ " + catName + ": энергии мало. E: " + strconv.Itoa(energyNow) + "/100 (нужно 25)."

	default:
		if strings.TrimSpace(catName) == "" {
			catName = "Кот"
		}
		story := narrative.HuntStory(res.Encounter, res.Flavor)
		msg := "🐾 " + catName + " " + story + ": +" + strconv.FormatInt(res.XPGain, 10) + " XP" +
			" • E: " + strconv.Itoa(energyNow) + "/100 (-" + strconv.Itoa(res.EnergyCost) + ")" +
			" • Форма: " + strconv.Itoa(res.EffPercent) + "% (" + mulFromPercent(res.EffPercent) + ")"
 		
		if res.Crit {
			msg += " ✨КРИТ"
		}
		if res.LeveledUp > 0 {
			if levelNow > 0 {
				msg += " 🎉 Уровень " + strconv.Itoa(levelNow)
			} else {
				msg += " 🎉 Уровень повышен!"
			}

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
