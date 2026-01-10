package domain

import "time"

// Game tuning constants (single source of truth for UI  repo).
const (
	EnergyMax = 100

	TrainingMinEnergy = 25

	// Soft limiter: if you train too soon, you still can train, but XP is scaled.
	TrainingEfficiencyWindow = 60 * time.Second
	TrainingMinEfficiency    = 0.5 // 50% XP when spamming immediately

	// Cat naming
	CatNameDefaultPrefix = "Мурчалкин"
	CatNameMaxLen        = 24
)
