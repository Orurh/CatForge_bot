package views

import (
	"strconv"
	"time"

	"catforge/internal/app"
	"catforge/internal/domain"
)

func FormatCatProfile(c *domain.Cat, now time.Time) string {
	energy := domain.RegenEnergy(c.Energy, c.EnergyUpdatedAt, now)
	nextLevelXP := int64(c.Level) * 100
	return BreedIcon(c.Breed) + " " + c.Name + "\n\n" +
		BreedRU(c.Breed) + " • Lv." + strconv.Itoa(c.Level) + "\n" +
		"😼 " + TraitRU(c.Trait) + "\n\n" +
		"⭐ " + strconv.FormatInt(c.XP, 10) + " / " + strconv.FormatInt(nextLevelXP, 10) + " XP\n" +
		"⚡ " + strconv.Itoa(energy) + " / 100\n" +
		"🪙 " + strconv.FormatInt(c.Coins, 10) + "\n\n" +
		"❤️ " + strconv.Itoa(c.HPBase) + "\n" +
		"⚔️ " + strconv.Itoa(c.ATKBase) + "\n" +
		"🛡 " + strconv.Itoa(c.DEFBase) + "\n" +
		"💨 " + strconv.Itoa(c.SPDBase)
}

func FormatCatProfileWithEquipment(c *domain.Cat, now time.Time, entries []app.CollectionEntry) string {
	power := domain.Power(c)
	energy := domain.RegenEnergy(c.Energy, c.EnergyUpdatedAt, now)
	bonus := app.EffectiveItemStats(entries)
	effectivePower := (c.ATKBase+bonus.ATK)*3 + (c.DEFBase+bonus.DEF)*2 + (c.HPBase+bonus.HP)/2 + c.SPDBase + bonus.SPD + c.Level*5

	return "Профиль кота (legacy equipment view)\n" +
		"Имя: " + c.Name + "\n" +
		"Порода: " + BreedRU(c.Breed) + "\n" +
		"Черта: " + TraitRU(c.Trait) + "\n" +
		"Уровень: " + strconv.Itoa(c.Level) + " (XP: " + strconv.FormatInt(c.XP, 10) + ")\n" +
		"Энергия: " + strconv.Itoa(energy) + "/100\n" +
		"Монеты: " + strconv.FormatInt(c.Coins, 10) + "\n" +
		"Сила: " + strconv.Itoa(power) + " → " + strconv.Itoa(effectivePower) + "\n\n" +
		"База статов:\n" +
		"HP: " + strconv.Itoa(c.HPBase) + statBonus(bonus.HP) + "\n" +
		"ATK: " + strconv.Itoa(c.ATKBase) + statBonus(bonus.ATK) + "\n" +
		"DEF: " + strconv.Itoa(c.DEFBase) + statBonus(bonus.DEF) + "\n" +
		"SPD: " + strconv.Itoa(c.SPDBase) + statBonus(bonus.SPD) + "\n\n" +
		"Снаряжение:\n" + FormatProfileEquipment(entries)
}

func statBonus(value int) string {
	if value == 0 {
		return ""
	}
	return " + " + strconv.Itoa(value)
}
