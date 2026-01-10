package domain

import "time"

const (
	EnergyRegenInterval = 1 * time.Second // 1 energy per 20s
)

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
