package domain

import "time"

// Kept for legacy callers; Training v2 has no frequency multiplier.
func TrainingEfficiency(lastTrainAt, now time.Time) (float64, int) { return 1, 100 }
