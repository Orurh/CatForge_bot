package domain

import "time"

// RegenEnergy returns energy value at "now" based on last update timestamp.
// NOTE: For MVP we only need correct display/canTrain logic. Persistence happens on Train().
func RegenEnergy(cur int, updatedAt, now time.Time) int {
	if cur >= EnergyMax {
		return EnergyMax
	}
	if updatedAt.IsZero() {
		return cur
	}
	d := now.Sub(updatedAt)
	if d <= 0 {
		return cur
	}
	add := int(d / EnergyRegenInterval)
	if add <= 0 {
		return cur
	}
	n := min(cur+add, EnergyMax)
	return n
}

// TrainingEnergyCost spends the entire accumulated reserve once training is available.
func TrainingEnergyCost(energy int) int {
	if energy < TrainingMinEnergy {
		return 0
	}
	return min(energy, EnergyMax)
}

// EnergyWait accounts for the unfinished regeneration interval, not just whole points.
func EnergyWait(cur int, updatedAt, now time.Time, target int) time.Duration {
	energy := RegenEnergy(cur, updatedAt, now)
	if energy >= target {
		return 0
	}
	wait := time.Duration(target-energy) * EnergyRegenInterval
	if !updatedAt.IsZero() {
		elapsed := now.Sub(updatedAt)
		if elapsed >= 0 {
			wait -= elapsed % EnergyRegenInterval
		} else {
			wait -= elapsed
		}
	}
	return wait
}
