package views

import (
	"strconv"
	"strings"

	"catforge/internal/app"
	"catforge/internal/domain"
)

func FormatCollection(entries []app.CollectionEntry) string {
	if len(entries) == 0 {
		return "🎒 Коллекция\n\nПока пусто. Первый предмет гарантирован в тренировке или событии после Lv2."
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
	text := "🎒 " + entry.Definition.Name + "\n" + rarityRU(entry.Definition.Rarity) + " · ур. " + strconv.Itoa(entry.Owned.Level) + "\n\n" + entry.Definition.Ability
	bonus := entry.Definition.PhysicalBonus(entry.Owned.Level)
	switch {
	case bonus.ClawsTenthMM > 0:
		text += "\n🩸 Когти +" + tenths(bonus.ClawsTenthMM) + " мм"
	case bonus.WeightGrams > 0:
		text += "\n🐈 Вес +" + tenths(bonus.WeightGrams/100) + " кг"
	case bonus.TailMM > 0:
		text += "\n🐾 Хвост +" + tenths(bonus.TailMM) + " см"
	case bonus.WhiskerSpanMM > 0:
		text += "\n〰️ Усы +" + tenths(bonus.WhiskerSpanMM) + " см"
	}
	if entry.Owned.Equipped {
		text += "\n\n✅ Сейчас надет"
	}
	text += "\nПовторы усиливают предмет автоматически."

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
