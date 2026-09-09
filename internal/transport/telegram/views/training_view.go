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

	text = "Тренировка\n" + EnergyTrainingStatus(c.Name, c.Energy, c.EnergyUpdatedAt, now) +
		"\n🔥 Крит тренировки: " + strconv.Itoa(5+min(c.TrainingCritBonusPercent, 5)) + "% (+100% базового XP)."
	if canTrain {
		text += "\nРасход: весь запас, " + strconv.Itoa(energyCost) + " энергии."
	}

	return text, canTrain
}

func FormatTrainingResultText(cat *domain.Cat, res domain.TrainResult, generatedNarrative string, now time.Time) string {
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
		if cat != nil {
			return EnergyTrainingStatus(catName, cat.Energy, cat.EnergyUpdatedAt, now)
		}
		return "⚡ " + catName + ": тренировка доступна с " + strconv.Itoa(domain.TrainingMinEnergy) + " энергии."

	default:
		if strings.TrimSpace(catName) == "" {
			catName = "Кот"
		}
		story := strings.TrimSpace(generatedNarrative)
		if story == "" {
			story = narrative.HuntStory(res.Encounter, trait, res.Flavor)
		}
		energyLine := "⚡ −" + strconv.Itoa(res.EnergyCost) + " энергии"
		if cat != nil {
			energyLine += " · осталось " + strconv.Itoa(domain.RegenEnergy(cat.Energy, cat.EnergyUpdatedAt, now)) + "/100"
		}
		msg := "🐾 " + catName + " " + story + "\n\n" + energyLine + "\n⭐ +" + strconv.FormatInt(res.XPGain, 10) + " XP"

		if res.Crit {
			msg = "🔥 КРИТИЧЕСКАЯ ТРЕНИРОВКА!\n" + msg + " · +100% базового XP за крит"
		}
		if res.LeveledUp > 0 {
			msg += "\n🎉 Уровень повышен!"

			if cat != nil {
				msg += "\n" + FormatFelineStats(cat.PhysicalStats())
			}
		}
		if cat != nil {
			msg += FormatProgressionFacts(cat.ProgressionFacts, cat.LootItemID)
			msg += "\n\n" + EnergyRecoveryText(catName, cat.Energy, cat.EnergyUpdatedAt, now)
		}

		return msg
	}
}
