package views

import (
	"fmt"

	"catforge/internal/domain"
)

func StarterScreenURL(publicBase string) string {
	return publicBase + "/static/ui/starter.png"
}

func CatAvatarURL(publicBase string, c *domain.Cat) string {
	// Only base assets exist for now. Keep the URL valid at every level; visual
	// tiers can be enabled as their optimized assets are added.
	return fmt.Sprintf("%s/static/cats/%s_base.png?v=%d", publicBase, c.Breed, c.Level)
}
