package telegram

import (
	"strconv"

	"catforge/internal/app"
	"catforge/internal/domain"
)

func StarterBreedKeyboard(ownerUserID int64) map[string]any {
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{"text": contentText("button.starter.maine_coon"), "callback_data": PersonalCallback(ownerUserID, CBStarterPrefix+"maine_coon")},
				{"text": contentText("button.starter.siamese"), "callback_data": PersonalCallback(ownerUserID, CBStarterPrefix+"siamese")},
			},
			{
				{"text": contentText("button.starter.british"), "callback_data": PersonalCallback(ownerUserID, CBStarterPrefix+"british")},
				{"text": contentText("button.starter.bengal"), "callback_data": PersonalCallback(ownerUserID, CBStarterPrefix+"bengal")},
			},
		},
	}
}

func MainMenuKeyboard(ownerUserID int64, levels ...int) map[string]any {
	result := map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{"text": contentText("button.train"), "callback_data": PersonalCallback(ownerUserID, CBTrainDo)},
				{"text": contentText("button.profile"), "callback_data": PersonalCallback(ownerUserID, CBMenuCat)},
			},
			{
				{"text": contentText("button.yard"), "callback_data": PersonalCallback(ownerUserID, CBMenuYard)},
				{"text": contentText("button.fight"), "callback_data": PersonalCallback(ownerUserID, CBMenuFight)},
			},
			{{"text": contentText("button.askcat"), "callback_data": PersonalCallback(ownerUserID, CBMenuAskCat)}},
		},
	}
	if len(levels) > 0 && levels[0] < 3 {
		rows := result["inline_keyboard"].([][]map[string]any)
		rows[1] = rows[1][:1]
		result["inline_keyboard"] = rows
	}
	return result
}

func BestiaryKeyboard() map[string]any {
	return map[string]any{"inline_keyboard": [][]map[string]any{{{"text": "⬅️ Назад", "callback_data": CBNavMenu}}}}
}

func ResetConfirmKeyboard(ownerUserID int64) map[string]any {
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{"text": contentText("button.reset_confirm"), "callback_data": PersonalCallback(ownerUserID, CBResetConfirm)},
			},
			{
				{"text": contentText("button.cancel"), "callback_data": PersonalCallback(ownerUserID, CBMenuCat)},
			},
		},
	}
}

func ProfileKeyboard(ownerUserID int64, personalities ...*domain.CatPersonality) map[string]any {
	autoLabel, humorLabel := "💬 Реплики: ?", "😼 Дерзкий юмор: ?"
	autoAction, humorAction := CBProfileRefresh, CBProfileRefresh
	if len(personalities) > 0 && personalities[0] != nil {
		p := personalities[0]
		autoLabel = "💬 Реплики: OFF"
		autoValue := "on"
		if p.AutoSpeakEnabled {
			autoLabel = "💬 Реплики: ON"
			autoValue = "off"
		}
		humorLabel = "😼 Дерзкий юмор: OFF"
		humorValue := "bold"
		if p.HumorMode == domain.HumorBold {
			humorLabel = "😼 Дерзкий юмор: ON"
			humorValue = "normal"
		}
		autoAction = preferenceAction("a", p.CatID, autoValue)
		humorAction = preferenceAction("h", p.CatID, humorValue)
	}

	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{{"text": contentText("button.train"), "callback_data": PersonalCallback(ownerUserID, CBTrainDo)}},
			{
				{"text": contentText("button.name"), "callback_data": PersonalCallback(ownerUserID, CBNameAsk)},
				{"text": contentText("button.refresh"), "callback_data": PersonalCallback(ownerUserID, CBProfileRefresh)},
			},
			{
				{"text": autoLabel, "callback_data": PersonalCallback(ownerUserID, autoAction)},
				{"text": humorLabel, "callback_data": PersonalCallback(ownerUserID, humorAction)},
			},
			{{"text": "❤️ Поддержать CatForge", "callback_data": PersonalCallback(ownerUserID, "support:open")}},
			{{"text": contentText("button.reset"), "callback_data": PersonalCallback(ownerUserID, CBResetAsk)}},
			{{"text": contentText("button.back"), "callback_data": PersonalCallback(ownerUserID, CBNavMenu)}},
		},
	}
}

func CollectionKeyboard(entries []app.CollectionEntry) map[string]any {
	rows := make([][]map[string]any, 0, len(entries)+1)
	for _, entry := range entries {
		mark := "▫️ "
		if entry.Owned.Equipped {
			mark = "✅ "
		}
		rows = append(rows, []map[string]any{{"text": mark + entry.Definition.Name, "callback_data": CBCollectionItemPrefix + entry.Definition.ID}})
	}
	rows = append(rows, []map[string]any{{"text": "⬅️ В профиль", "callback_data": CBMenuCat}})
	return map[string]any{"inline_keyboard": rows}
}

func CollectionItemKeyboard(entry app.CollectionEntry, levels ...int) map[string]any {
	rows := make([][]map[string]any, 0, 3)
	if !entry.Owned.Equipped && (len(levels) == 0 || domain.SlotUnlocked(entry.Definition.Slot, levels[0])) {
		rows = append(rows, []map[string]any{{"text": "✅ Надеть", "callback_data": CBCollectionEquipPrefix + entry.Definition.ID}})
	}

	rows = append(rows, []map[string]any{{"text": "⬅️ К коллекции", "callback_data": CBCollection}})
	return map[string]any{"inline_keyboard": rows}
}

func NamePromptKeyboard(ownerUserID int64) map[string]any {
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{"text": "Пропустить (оставить текущее)", "callback_data": PersonalCallback(ownerUserID, CBNameSkip)},
			},
			{
				{"text": "⬅️ Назад", "callback_data": PersonalCallback(ownerUserID, CBMenuCat)},
				{"text": "🏠 Меню", "callback_data": PersonalCallback(ownerUserID, CBNavMenu)},
			},
		},
	}
}

func TrainingKeyboard(ownerUserID int64, canTrain bool) map[string]any {
	btn := map[string]any{"text": contentText("button.train"), "callback_data": PersonalCallback(ownerUserID, CBTrainDo)}
	if !canTrain {
		btn = map[string]any{"text": "⏳ Тренировка недоступна", "callback_data": PersonalCallback(ownerUserID, CBNoop)}
	}
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{btn},
			{
				{"text": contentText("button.refresh"), "callback_data": PersonalCallback(ownerUserID, CBTrainRefresh)},
				{"text": contentText("button.back"), "callback_data": PersonalCallback(ownerUserID, CBNavMenu)},
			},
		},
	}
}

func ArenaKeyboard() map[string]any {
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{"text": "⬅️ Назад", "callback_data": CBNavMenu},
			},
		},
	}
}

func ExpeditionKeyboard(canExplore bool) map[string]any {
	if !canExplore {
		return map[string]any{"inline_keyboard": [][]map[string]any{
			{{"text": "⏳ Не хватает энергии", "callback_data": CBNoop}},
			{{"text": "🔄 Обновить", "callback_data": CBExpeditionRefresh}, {"text": "⬅️ Назад", "callback_data": CBNavMenu}},
		}}
	}
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{{"text": "🛹 Переулки", "callback_data": CBExpeditionChoosePrefix + "alley"}},
			{{"text": "🏙 Крыши", "callback_data": CBExpeditionChoosePrefix + "rooftop"}},
			{{"text": "🌲 Старый парк", "callback_data": CBExpeditionChoosePrefix + "park"}},
			{
				{"text": "🔄 Обновить", "callback_data": CBExpeditionRefresh},
				{"text": "⬅️ Назад", "callback_data": CBNavMenu},
			},
		},
	}
}

func ExpeditionLootKeyboard(canExplore bool, itemID string) map[string]any {
	base := ExpeditionKeyboard(canExplore)
	rows, _ := base["inline_keyboard"].([][]map[string]any)
	lootRows := [][]map[string]any{
		{{"text": "✅ Надеть предмет", "callback_data": CBCollectionEquipPrefix + itemID}},
		{{"text": "🎒 Открыть коллекцию", "callback_data": CBCollection}},
	}
	base["inline_keyboard"] = append(lootRows, rows...)
	return base
}

func ExpeditionDifficultyKeyboard(location string, energy int) map[string]any {
	button := func(text, difficulty string, cost int) map[string]any {
		if energy < cost {
			return map[string]any{"text": "🔒 " + text, "callback_data": CBNoop}
		}
		return map[string]any{"text": text, "callback_data": CBExpeditionDoPrefix + location + ":" + difficulty}
	}
	return map[string]any{"inline_keyboard": [][]map[string]any{
		{button("🟢 Лёгко ("+strconv.Itoa(domain.ExpeditionCost(domain.ExpeditionEasy))+")", "easy", domain.ExpeditionCost(domain.ExpeditionEasy))},
		{button("🟡 Нормально ("+strconv.Itoa(domain.ExpeditionCost(domain.ExpeditionNormal))+")", "normal", domain.ExpeditionCost(domain.ExpeditionNormal))},
		{button("🔴 Сложно ("+strconv.Itoa(domain.ExpeditionCost(domain.ExpeditionHard))+")", "hard", domain.ExpeditionCost(domain.ExpeditionHard))},
		{{"text": "⬅️ К локациям", "callback_data": CBExpeditionRefresh}},
	}}
}

func YardEventKeyboard(eventID int64, eventType domain.YardEventType) map[string]any {
	prefix := CBYardChoicePrefix + strconv.FormatInt(eventID, 10) + ":"
	rows := [][]map[string]any{
		{{"text": contentText(yardEventContentKey(eventType, "button.steal")), "callback_data": prefix + string(domain.YardChoiceSteal)}},
		{{"text": contentText(yardEventContentKey(eventType, "button.distract")), "callback_data": prefix + string(domain.YardChoiceDistract)}},
		{{"text": contentText(yardEventContentKey(eventType, "button.scout")), "callback_data": prefix + string(domain.YardChoiceScout)}},
	}
	if action, ok := domain.FeaturedSpecial(eventType, eventID); ok {
		rows = append(rows, []map[string]any{{"text": action.Label, "callback_data": prefix + action.ID}})
	}
	return map[string]any{"inline_keyboard": rows}
}
