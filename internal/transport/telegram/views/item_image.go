package views

import (
	"catforge/internal/assets"
	"catforge/internal/gamedata"
)

func ItemImageURL(itemID string) string {
	if _, ok := gamedata.ItemByID(itemID); !ok {
		return ""
	}
	path := "static/items/" + itemID + ".png"
	if _, err := assets.FS.ReadFile(path); err != nil {
		return ""
	}
	return "/" + path
}
