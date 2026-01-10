package telegram

import (
	"strconv"

	"catforge/internal/domain"
)

func ArenaKeyboard(state domain.ArenaState, ops []domain.ArenaOpponent) any {
	rows := make([][]map[string]any, 0, 6)

	for _, op := range ops {
		label := "⚔️ Бой"
		switch op.Kind {
		case domain.ArenaOppWeaker:
			label = "✅ Лёгкая цель"
		case domain.ArenaOppEven:
			label = "⚔️ Равный бой"
		case domain.ArenaOppStronger:
			label = "💀 Риск"
		}
		btn := map[string]any{
			"text":          label + ": " + op.Name + " (L" + strconv.Itoa(op.Level) + ", " + strconv.Itoa(op.Power) + ")",
			"callback_data": CBArenaFightPref + strconv.FormatInt(op.UserID, 10),
		}
		rows = append(rows, []map[string]any{btn})
	}

	rows = append(rows, []map[string]any{
		{"text": "🔄 Обновить", "callback_data": CBArenaRefresh},
		{"text": "🎲 Реролл", "callback_data": CBArenaReroll},
	})
	rows = append(rows, []map[string]any{
		{"text": "⬅️ Меню", "callback_data": CBNavMenu},
	})

	return map[string]any{"inline_keyboard": rows}
}
