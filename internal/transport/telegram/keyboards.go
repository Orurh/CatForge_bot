package telegram

func StarterBreedKeyboard() map[string]any {
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{"text": "Мейн-кун (HP)", "callback_data": CBStarterPrefix + "maine_coon"},
				{"text": "Сиам (SPD)", "callback_data": CBStarterPrefix + "siamese"},
			},
			{
				{"text": "Британец (DEF)", "callback_data": CBStarterPrefix + "british"},
				{"text": "Бенгал (ATK)", "callback_data": CBStarterPrefix + "bengal"},
			},
		},
	}
}

func MainMenuKeyboard() map[string]any {
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{"text": "Профиль кота", "callback_data": CBMenuCat},
				{"text": "Охота", "callback_data": CBMenuTrain},
			},
			{
				{"text": "🎁 Ежедневка", "callback_data": CBMenuDaily},
				{"text": "Экспедиция", "callback_data": CBMenuExp},
			},
			{{"text": "Арена", "callback_data": CBMenuPVP}},
			{{"text": "♻️ Убить котика и начать заново", "callback_data": CBResetAsk}},
		},
	}
}

func ResetConfirmKeyboard() map[string]any {
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{"text": "✅ Да, убить котика", "callback_data": CBResetConfirm},
			},
			{
				{"text": "❌ Отмена", "callback_data": CBNavMenu},
			},
		},
	}
}

func ProfileKeyboard() map[string]any {
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{"text": "✏️ Имя", "callback_data": CBNameAsk},
				{"text": "🔄 Обновить", "callback_data": CBProfileRefresh},
			},
			{{"text": "⬅️ Назад", "callback_data": CBNavMenu}},
		},
	}
}

func NamePromptKeyboard() map[string]any {
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{"text": "Пропустить (оставить текущее)", "callback_data": CBNameSkip},
			},
			{
				{"text": "⬅️ Назад", "callback_data": CBMenuCat},
				{"text": "🏠 Меню", "callback_data": CBNavMenu},
			},
		},
	}
}

func TrainingKeyboard(canTrain bool) map[string]any {
	btn := map[string]any{"text": "🏋️ Выйти на охоту", "callback_data": CBTrainDo}
	if !canTrain {
		btn = map[string]any{"text": "⏳ Охота недоступна", "callback_data": CBNoop}
	}
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{btn},
			{
				{"text": "🔄 Обновить", "callback_data": CBTrainRefresh},
				{"text": "⬅️ Назад", "callback_data": CBNavMenu},
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
