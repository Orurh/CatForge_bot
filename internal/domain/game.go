package domain

import "time"

// Game tuning constants (single source of truth for UI  repo).
const (
	EnergyMax = 100
	// Temporary dev tuning for fast Telegram testing. Restore to 8 minutes before production.
	EnergyRegenInterval   = time.Second
	TrainingMinEnergy     = 25
	TrainingEnergyReserve = 10
	TrainingMaxEnergyCost = 90

	// Soft limiter: if you train too soon, you still can train, but XP is scaled.
	TrainingEfficiencyWindow = 60 * time.Second
	TrainingMinEfficiency    = 0.5 // 50% XP when spamming immediately

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
