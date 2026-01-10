package domain

import "time"

func TrainingEfficiency(lastTrainAt, now time.Time) (float64, int) {
	eff := 1.0
	if lastTrainAt.IsZero() {
		return eff, 100
	}

	delta := now.Sub(lastTrainAt)
	if delta <= 0 {
		min := float64(TrainingMinEfficiency)
		return min, int(min*100 + 0.5)
	}

	if delta < TrainingEfficiencyWindow {
		ratio := float64(delta) / float64(TrainingEfficiencyWindow)
		min := float64(TrainingMinEfficiency)
		eff = min + (1.0-min)*ratio
		if eff < min {
			eff = min
		}
	}

	return eff, int(eff*100 + 0.5)
}
