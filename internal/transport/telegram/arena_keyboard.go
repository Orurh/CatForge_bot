package telegram

import (
	"strconv"
	"time"

	"catforge/internal/app"
	"catforge/internal/domain"
)

func ArenaKeyboard(state domain.ArenaState, ops []app.ArenaOpponentView, canFree bool, wait time.Duration, canPay bool, payCost int) any {
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
			"text":          label + ": " + op.Name + " (L" + strconv.Itoa(op.Level) + ", " + strconv.Itoa(op.Power) + ") ~" + strconv.Itoa(op.WinProbPct) + "%",
			"callback_data": CBArenaFightPref + strconv.FormatInt(op.UserID, 10),
		}
		rows = append(rows, []map[string]any{btn})
	}

	rerollRow := make([]map[string]any, 0, 3)
	rerollRow = append(rerollRow, map[string]any{"text": "🔄 Обновить", "callback_data": CBArenaRefresh})

	if canFree {
		rerollRow = append(rerollRow, map[string]any{"text": "🎲 Реролл", "callback_data": CBArenaReroll})
	} else {
		sec := int(wait.Seconds() + 0.5)
		if sec < 1 {
			sec = 1
		}
		min := sec / 60
		s := sec % 60
		label := "⏳ Реролл через " + itoa(min) + "м " + itoa(s) + "с"
		rerollRow = append(rerollRow, map[string]any{"text": label, "callback_data": CBNoop})

		if canPay {
			rerollRow = append(rerollRow, map[string]any{
				"text":          "⚡ Реролл за " + itoa(payCost) + "E",
				"callback_data": CBArenaRerollPay,
			})
		}
	}

	rows = append(rows, rerollRow)

	rows = append(rows, []map[string]any{
		{"text": "⬅️ Меню", "callback_data": CBNavMenu},
	})

	return map[string]any{"inline_keyboard": rows}
}
