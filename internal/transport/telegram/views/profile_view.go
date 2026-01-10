package views

import (
	"strconv"
	"time"

	"catforge/internal/domain"
)

func FormatCatProfile(c *domain.Cat, now time.Time) string {
	power := domain.Power(c)
	energy := domain.RegenEnergy(c.Energy, c.EnergyUpdatedAt, now)

	return "Профиль кота\n" +
		"Имя: " + c.Name + "\n" +
		"Порода: " + BreedRU(c.Breed) + "\n" +
		"Черта: " + TraitRU(c.Trait) + "\n" +
		"Уровень: " + strconv.Itoa(c.Level) + " (XP: " + strconv.FormatInt(c.XP, 10) + ")\n" +
		"Энергия: " + strconv.Itoa(energy) + "/100\n" +
		"Сила: " + strconv.Itoa(power) + "\n\n" +
		"База статов:\n" +
		"HP: " + strconv.Itoa(c.HPBase) + "\n" +
		"ATK: " + strconv.Itoa(c.ATKBase) + "\n" +
		"DEF: " + strconv.Itoa(c.DEFBase) + "\n" +
		"SPD: " + strconv.Itoa(c.SPDBase)
}
