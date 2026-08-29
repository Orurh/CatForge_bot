package views

import (
	"strconv"
	"strings"

	"catforge/internal/app"
	"catforge/internal/domain"
)

func FormatCollection(entries []app.CollectionEntry) string {
	if len(entries) == 0 {
		return "🎒 Коллекция\n\nПока пусто. Первый предмет гарантирован за первую победную экспедицию."
	}
	lines := []string{"🎒 Коллекция", ""}
	for _, entry := range entries {
		mark := "▫️"
		if entry.Owned.Equipped {
			mark = "✅"
		}
		lines = append(lines, mark+" "+entry.Definition.Name+" • "+rarityRU(entry.Definition.Rarity)+
			" • ур. "+strconv.Itoa(entry.Owned.Level)+" • 🧩 "+strconv.Itoa(entry.Owned.Fragments))
	}
	return strings.Join(lines, "\n")
}

func FormatItem(entry app.CollectionEntry, prefix string) string {
	stats := entry.Definition.StatsAtLevel(entry.Owned.Level)
	text := "🎒 " + entry.Definition.Name + "\n" +
		"Редкость: " + rarityRU(entry.Definition.Rarity) + "\n" +
		"Слот: " + itemSlotRU(entry.Definition.Slot) + "\n" +
		"Уровень: " + strconv.Itoa(entry.Owned.Level) + "/" + strconv.Itoa(domain.ItemMaxLevel) + "\n" +
		"Фрагменты: " + strconv.Itoa(entry.Owned.Fragments) + "\n" +
		"Бонус: " + formatStatDelta(stats)
	if entry.Owned.Equipped {
		text += "\n\n✅ Сейчас надет"
	}
	if fragments, coins, ok := domain.UpgradeCost(entry.Owned.Level); ok {
		text += "\n\nУлучшение: 🧩 " + strconv.Itoa(fragments) + " + 🪙 " + strconv.FormatInt(coins, 10)
	} else {
		text += "\n\n⭐ Максимальный уровень"
	}
	if strings.TrimSpace(prefix) != "" {
		return prefix + "\n\n" + text
	}
	return text
}

func FormatProfileEquipment(entries []app.CollectionEntry) string {
	bySlot := map[domain.ItemSlot]string{
		domain.ItemSlotClaws: "—", domain.ItemSlotCollar: "—", domain.ItemSlotCharm: "—",
	}
	for _, entry := range entries {
		if entry.Owned.Equipped {
			bySlot[entry.Definition.Slot] = entry.Definition.Name + " (ур. " + strconv.Itoa(entry.Owned.Level) + ")"
		}
	}
	return "🐾 Когти: " + bySlot[domain.ItemSlotClaws] + "\n" +
		"🧥 Ошейник: " + bySlot[domain.ItemSlotCollar] + "\n" +
		"🧿 Талисман: " + bySlot[domain.ItemSlotCharm]
}

func itemSlotRU(slot domain.ItemSlot) string {
	switch slot {
	case domain.ItemSlotClaws:
		return "Когти"
	case domain.ItemSlotCollar:
		return "Ошейник"
	default:
		return "Талисман"
	}
}
