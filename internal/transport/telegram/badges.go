package telegram

import "catforge/internal/domain"

// publicCatBadge returns a compact "mini-icon" for public logs (groups/channels).
// Uses unicode emoji (works everywhere, no premium/custom-emoji dependencies).
func publicCatBadge(b domain.Breed, level int) string {
	// breed icon
	breedIcon := "🐱"
	switch b {
	case domain.BreedMaineCoon:
		breedIcon = "🦁" 
	case domain.BreedSiamese:
		breedIcon = "🐈‍⬛"
	case domain.BreedBritish:
		breedIcon = "🐻" 
	case domain.BreedBengal:
		breedIcon = "🐆"
	}

	tier := ""
	switch {
	case level >= 20:
		tier = "💎"
	case level >= 10:
		tier = "🥇"
	case level >= 5:
		tier = "🥈"
	default:
		tier = "🥉"
	}

	return breedIcon + tier
}
