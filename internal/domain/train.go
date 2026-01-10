package domain

// TrainingOutcome describes what happened during Train().
type TrainingOutcome int

const (
	TrainingOK TrainingOutcome = iota
	TrainingNotEnoughEnergy
)

// TrainResult is a pure data result (UI/log formatting lives in transport layer).
type TrainResult struct {
	Outcome     TrainingOutcome
	XPGain      int64
	EnergyCost  int
	Crit        bool
	EffPercent  int // 50..100 (soft limiter)
	LeveledUp   int // how many levels gained in one action
	StatsGained StatDelta

	Encounter Encounter
	Flavor    uint16
}
