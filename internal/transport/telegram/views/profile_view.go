package views

import (
	"strconv"
	"time"

	"catforge/internal/app"
	"catforge/internal/domain"
)

func FormatCatProfile(c *domain.Cat, now time.Time) string {
	return FormatCatProfileWithArena(c, now, domain.ArenaStats{})
}

func FormatCatProfileWithArena(c *domain.Cat, now time.Time, arena domain.ArenaStats) string {
	nextLevelXP := int64(c.Level) * 100
	streak := "нет"
	if arena.CurrentStreak > 0 {
		kind := "поражений"
		if arena.StreakWins {
			kind = "побед"
		}
		streak = strconv.Itoa(arena.CurrentStreak) + " " + kind
	}
	text := BreedIcon(c.Breed) + " " + c.Name + "\n\n" + BreedRU(c.Breed) + " · уровень " + strconv.Itoa(c.Level) + "\n😼 " + TraitRU(c.Trait) + "\n\n" +
		"⭐ XP: " + strconv.FormatInt(c.XP, 10) + "/" + strconv.FormatInt(nextLevelXP, 10) + "\n" + EnergyProfileStatus(c.Energy, c.EnergyUpdatedAt, now) + "\n\n" + FormatFelineStats(c.PhysicalStats())
	if c.Level >= 3 {
		text += "\n\n🥊 АРЕНА\nПобеды: " + strconv.Itoa(arena.Wins) + " · Поражения: " + strconv.Itoa(arena.Losses) + "\nСерия: " + streak
	}
	if u, ok := domain.NextUnlock(c.Level); ok {
		text += "\n\n🔒 Следующее открытие: " + u.Label + " — Lv" + strconv.Itoa(u.Level) + "."
	}
	return text

}

func FormatCatProfileWithEquipment(c *domain.Cat, now time.Time, entries []app.CollectionEntry) string {
	power := domain.Power(c)
	bonus := app.EffectiveItemStats(entries)
	effectivePower := (c.ATKBase+bonus.ATK)*3 + (c.DEFBase+bonus.DEF)*2 + (c.HPBase+bonus.HP)/2 + c.SPDBase + bonus.SPD + c.Level*5

	return "Профиль кота (legacy equipment view)\n" +
		"Имя: " + c.Name + "\n" +
		"Порода: " + BreedRU(c.Breed) + "\n" +
		"Черта: " + TraitRU(c.Trait) + "\n" +
		"Уровень: " + strconv.Itoa(c.Level) + " (XP: " + strconv.FormatInt(c.XP, 10) + ")\n" +
		EnergyProfileStatus(c.Energy, c.EnergyUpdatedAt, now) + "\n" +
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
