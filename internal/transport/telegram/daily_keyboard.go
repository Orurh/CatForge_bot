package telegram

func DailyKeyboard(canClaim bool) any {
	rows := make([][]map[string]any, 0, 2)
	if canClaim {
		rows = append(rows, []map[string]any{
			{"text": "🎁 Получить награду", "callback_data": CBDailyClaim},
		})
	} else {
		rows = append(rows, []map[string]any{
			{"text": "🔄 Обновить", "callback_data": CBDailyRefresh},
		})
	}
	rows = append(rows, []map[string]any{
		{"text": "⬅️ Меню", "callback_data": CBNavMenu},
	})
	return map[string]any{"inline_keyboard": rows}
}