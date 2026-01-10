package views

func OpenPrivateKeyboard(botUsername string) map[string]any {
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{
					"text": "👉 Открыть бота",
					"url":  "https://t.me/" + botUsername,
				},
			},
		},
	}
}
