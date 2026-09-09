package views

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"catforge/internal/domain"
	"catforge/internal/gamedata"
	"catforge/internal/transport/telegram/narrative"
)

func FormatExpeditionScreen(cat *domain.Cat, now time.Time) (string, bool) {
	energy := domain.RegenEnergy(cat.Energy, cat.EnergyUpdatedAt, now)
	return "🗺️ Экспедиция\n" +
		"Кот: " + cat.Name + " • уровень " + strconv.Itoa(cat.Level) + "\n" +
		"Энергия: " + strconv.Itoa(energy) + "/100\n" +
		"Монеты: " + strconv.FormatInt(cat.Coins, 10) + "\n\n" +
		"🛹 Переулки — канализационные крысы\n" +
		"🏙 Крыши — бродячие собаки\n" +
		"🌲 Старый парк — дикие рыси\n\n" +
		"Выбери локацию:", energy >= domain.ExpeditionCost(domain.ExpeditionEasy)
}

func FormatExpeditionDifficulty(cat *domain.Cat, now time.Time, location domain.ExpeditionLocation) string {
	energy := domain.RegenEnergy(cat.Energy, cat.EnergyUpdatedAt, now)
	return "🗺️ " + ExpeditionLocationRU(location) + "\n" +
		"Энергия: " + strconv.Itoa(energy) + "/100\n\n" +
		"🟢 Лёгко — " + strconv.Itoa(domain.ExpeditionCost(domain.ExpeditionEasy)) + " энергии\n" +
		"🟡 Нормально — " + strconv.Itoa(domain.ExpeditionCost(domain.ExpeditionNormal)) + " энергии\n" +
		"🔴 Сложно — " + strconv.Itoa(domain.ExpeditionCost(domain.ExpeditionHard)) + " энергии\n\n" +
		"Чем выше сложность, тем сильнее враг и больше XP и монет."
}

func FormatExpeditionResultText(cat *domain.Cat, result domain.ExpeditionResult) string {
	catName := ""
	trait := domain.Trait("")
	if cat != nil {
		catName = cat.Name
		trait = cat.Trait
	}
	if strings.TrimSpace(catName) == "" {
		catName = "Кот"
	}
	if result.Outcome == domain.ExpeditionNotEnoughEnergy {
		return "⚡ " + catName + ": не хватает энергии. Нужно " + strconv.Itoa(domain.ExpeditionCost(result.Difficulty)) + "."
	}

	resultWord := "❌ Поражение"
	if result.Outcome == domain.ExpeditionVictory {
		resultWord = "✅ Победа"
	}
	rareLine := ""
	if domain.IsRareEnemy(result.Enemy.Kind) {
		rareLine = "✨ Редкая встреча! Награда ×1.5\n"
	}
	msg := "📖 " + catName + " " + narrative.ExpeditionStory(result, trait) + "\n\n" +
		rareLine + resultWord + " • " + ExpeditionLocationRU(result.Location) + " • " + ExpeditionDifficultyRU(result.Difficulty) + "\n" +
		"Враг: " + enemyName(result.Enemy.Kind) + " (ур. " + strconv.Itoa(result.Enemy.Level) + ")\n" +
		"Раундов: " + strconv.Itoa(result.Rounds) +
		" • XP: +" + strconv.FormatInt(result.XPGain, 10) +
		" • монеты: +" + strconv.FormatInt(result.CoinsGain, 10) +
		" • энергия: -" + strconv.Itoa(result.EnergyCost) + "\n" +
		"HP " + catName + ": " + strconv.Itoa(result.CatHPAfter) +
		" • HP врага: " + strconv.Itoa(result.EnemyHPAfter)
	if result.LeveledUp > 0 {
		msg += "\n🎉 Уровень повышен!"
	}
	if result.Loot.Dropped {
		name := result.Loot.ItemID
		stats := domain.StatDelta{}
		if item, ok := gamedata.ItemByID(result.Loot.ItemID); ok {
			name = item.Name
			stats = item.StatsAtLevel(1)
		}
		if result.Loot.New {
			msg += "\n\n🎁 Новый предмет: " + name + " [" + rarityRU(result.Loot.Rarity) + "]"
			msg += "\n" + formatStatDelta(stats)
		} else {
			msg += "\n\n🧩 Дубликат: " + name + " → +1 фрагмент (всего " + strconv.Itoa(result.Loot.Fragments) + ")"
		}
	}
	if log := compactBattleLog(result.Turns, catName); log != "" {
		msg += "\n\n⚔️ Ключевые ходы:\n" + log
	}
	return msg
}

func rarityRU(rarity domain.ItemRarity) string {
	switch rarity {
	case domain.ItemRare:
		return "Rare"
	case domain.ItemEpic:
		return "Epic"
	default:
		return "Common"
	}
}

func formatStatDelta(stats domain.StatDelta) string {
	parts := make([]string, 0, 4)
	if stats.HP != 0 {
		parts = append(parts, "HP +"+strconv.Itoa(stats.HP))
	}
	if stats.ATK != 0 {
		parts = append(parts, "ATK +"+strconv.Itoa(stats.ATK))
	}
	if stats.DEF != 0 {
		parts = append(parts, "DEF +"+strconv.Itoa(stats.DEF))
	}
	if stats.SPD != 0 {
		parts = append(parts, "SPD +"+strconv.Itoa(stats.SPD))
	}
	return strings.Join(parts, " • ")
}

func compactBattleLog(turns []domain.BattleTurn, catName string) string {
	if len(turns) == 0 {
		return ""
	}
	selected := turns
	if len(turns) > 6 {
		selected = append(append([]domain.BattleTurn{}, turns[:3]...), turns[len(turns)-3:]...)
	}
	lines := make([]string, 0, len(selected)+1)
	for index, turn := range selected {
		if len(turns) > 6 && index == 3 {
			lines = append(lines, "…")
		}
		actor, target := catName, "врага"
		if turn.Actor == domain.BattleActorEnemy {
			actor, target = "Враг", catName
		}
		crit := ""
		if turn.Crit {
			crit = " КРИТ"
		}
		lines = append(lines, fmt.Sprintf("%d. %s → %s: %d%s (HP %d)", turn.Round, actor, target, turn.Damage, crit, turn.DefenderHPAfter))
	}
	return strings.Join(lines, "\n")
}

func ExpeditionLocationRU(location domain.ExpeditionLocation) string {
	switch location {
	case domain.ExpeditionRooftop:
		return "Крыши"
	case domain.ExpeditionPark:
		return "Старый парк"
	default:
		return "Переулки"
	}
}

func ExpeditionDifficultyRU(difficulty domain.ExpeditionDifficulty) string {
	switch difficulty {
	case domain.ExpeditionEasy:
		return "легко"
	case domain.ExpeditionHard:
		return "сложно"
	default:
		return "нормально"
	}
}

func enemyName(kind domain.EnemyKind) string {
	return EnemyName(kind)
}
