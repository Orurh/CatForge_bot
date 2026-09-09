package telegram

import "catforge/internal/gamecontent"

func contentText(key string) string {
	return gamecontent.Render(key, 0, nil)
}
