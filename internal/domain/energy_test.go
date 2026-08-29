package domain

import (
	"testing"
	"time"
)

func TestRegenEnergy(t *testing.T) {
	t.Parallel()

	updatedAt := time.Unix(100, 0)
	tests := []struct {
		name string
		cur  int
		now  time.Time
		want int
	}{
		{name: "adds elapsed intervals", cur: 40, now: updatedAt.Add(3 * EnergyRegenInterval), want: 43},
		{name: "caps at maximum", cur: 99, now: updatedAt.Add(5 * EnergyRegenInterval), want: EnergyMax},
		{name: "does not regen backwards", cur: 40, now: updatedAt.Add(-time.Second), want: 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := RegenEnergy(tt.cur, updatedAt, tt.now); got != tt.want {
				t.Fatalf("RegenEnergy() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestTrainingEnergyCostUsesMostAvailableEnergy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		energy int
		want   int
	}{
		{energy: 100, want: 90},
		{energy: 75, want: 65},
		{energy: 40, want: 30},
		{energy: 25, want: 25},
		{energy: 24, want: 0},
	}
	for _, test := range tests {
		if got := TrainingEnergyCost(test.energy); got != test.want {
			t.Errorf("TrainingEnergyCost(%d) = %d, want %d", test.energy, got, test.want)
		}
	}
}

func TestTrainingEfficiency(t *testing.T) {
	t.Parallel()

	now := time.Unix(1000, 0)
	if _, got := TrainingEfficiency(time.Time{}, now); got != 100 {
		t.Fatalf("first training efficiency = %d, want 100", got)
	}
	if _, got := TrainingEfficiency(now, now); got != 50 {
		t.Fatalf("immediate training efficiency = %d, want 50", got)
	}
	if _, got := TrainingEfficiency(now.Add(-TrainingEfficiencyWindow), now); got != 100 {
		t.Fatalf("rested training efficiency = %d, want 100", got)
	}
}
