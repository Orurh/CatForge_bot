package views

import "strings"

func OpenPrivateKeyboard(botUsername string) map[string]any {
	if strings.TrimSpace(botUsername) == "" {
		return nil
	}
	privateURL := "https://t.me/" + strings.TrimPrefix(strings.TrimSpace(botUsername), "@")
	return map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{
					"text": "💬 Открыть личку",
					"url":  privateURL,
				},
			},
		},
	}
}
