package views

import (
	"catforge/internal/domain"
	"fmt"
	"strconv"
	"time"
)

func energyDuration(wait time.Duration) string {
	minutes := int((wait + time.Minute - 1) / time.Minute)
	if minutes <= 0 {
		return "0м"
	}
	if minutes < 60 {
		return strconv.Itoa(minutes) + "м"
	}
	if minutes%60 == 0 {
		return strconv.Itoa(minutes/60) + "ч"
	}
	return fmt.Sprintf("%dч %02dм", minutes/60, minutes%60)
}

func EnergyProfileStatus(energy int, updatedAt, now time.Time) string {
	current := domain.RegenEnergy(energy, updatedAt, now)
	line := "⚡ " + strconv.Itoa(current) + "/100 · "
	switch {
	case current >= domain.EnergyMax:
		return line + "полная, дальше не копится"
	case current >= domain.TrainingMinEnergy:
		return line + "можно тренироваться"
	default:
		return line + "тренировка через ~" + energyDuration(domain.EnergyWait(energy, updatedAt, now, domain.TrainingMinEnergy))
	}
}

func EnergyTrainingStatus(name string, energy int, updatedAt, now time.Time) string {
	current := domain.RegenEnergy(energy, updatedAt, now)
	line := "⚡ Энергия: " + strconv.Itoa(current) + "/100"
	switch {
	case current >= domain.EnergyMax:
		return "🟢 " + name + ": полный запас.\n" + line + " — полный запас.\nДальше энергия не копится. Самое время тренироваться."
	case current >= domain.TrainingMinEnergy:
		return "🟡 " + name + " уже может тренироваться.\n" + line + "\nМожно идти сейчас или подождать: чем больше энергии накоплено, тем больше будет тренировка.\nДо полного запаса: ~" + energyDuration(domain.EnergyWait(energy, updatedAt, now, domain.EnergyMax)) + "."
	default:
		return "🔴 " + name + " ещё отдыхает.\n" + line + "\nТренироваться можно с " + strconv.Itoa(domain.TrainingMinEnergy) + " — примерно через " + energyDuration(domain.EnergyWait(energy, updatedAt, now, domain.TrainingMinEnergy)) + ".\nДо полного запаса: ~" + energyDuration(domain.EnergyWait(energy, updatedAt, now, domain.EnergyMax)) + "."
	}
}

func EnergyRecoveryText(energy int, updatedAt, now time.Time) string {
	current := domain.RegenEnergy(energy, updatedAt, now)
	if current >= domain.EnergyMax {
		return "Полный запас: дальше энергия не копится."
	}
	next := "Тренировка уже доступна"
	if current < domain.TrainingMinEnergy {
		next = "Тренировка через ~" + energyDuration(domain.EnergyWait(energy, updatedAt, now, domain.TrainingMinEnergy))
	}
	return next + " · полный запас через ~" + energyDuration(domain.EnergyWait(energy, updatedAt, now, domain.EnergyMax)) + ".\n"
}
