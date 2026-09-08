package domain

import "time"

// Game tuning constants (single source of truth for UI  repo).
const (
	EnergyMax = 100
	// One full training per roughly twelve hours.
	EnergyRegenInterval = 8 * time.Minute
	TrainingMinEnergy   = 50

	// Cat naming
	CatNameDefaultPrefix = "Мурчалкин"
	CatNameMaxLen        = 24
)

func BaseStatsByBreed(breed Breed) (hp, atk, def, spd int) {
	switch breed {
	case BreedMaineCoon:
		return 46, 20, 18, 14
	case BreedSiamese:
		return 42, 18, 18, 26
	case BreedBritish:
		return 42, 20, 22, 16
	case BreedBengal:
		return 40, 22, 16, 20
	default:
		return 40, 20, 20, 20
	}
}
