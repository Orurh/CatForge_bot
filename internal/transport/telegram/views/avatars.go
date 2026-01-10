package views

import (
	"fmt"

	"catforge/internal/domain"
)

func StarterScreenURL(publicBase string) string {
	return publicBase + "/static/ui/starter.png"
}

func CatAvatarURL(publicBase string, c *domain.Cat) string {
	tier := "base"
	switch {
	case c.Level >= 20:
		tier = "diamond"
	case c.Level >= 10:
		tier = "gold"
	case c.Level >= 5:
		tier = "silver"
	}

	// v=level чтобы Telegram охотнее перезапрашивал при апе (кеш).
	return fmt.Sprintf("%s/static/cats/%s_%s.png?v=%d", publicBase, c.Breed, tier, c.Level)
}
