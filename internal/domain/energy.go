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

// TrainingEnergyCost makes Training a single meaningful action instead of a
// button that is optimal to spam. A rested cat keeps a small energy reserve;
// a cat at the minimum threshold spends the full minimum amount.
func TrainingEnergyCost(energy int) int {
	if energy < TrainingMinEnergy {
		return 0
	}
	cost := energy - TrainingEnergyReserve
	if cost < TrainingMinEnergy {
		return TrainingMinEnergy
	}
	if cost > TrainingMaxEnergyCost {
		return TrainingMaxEnergyCost
	}
	return cost
}
