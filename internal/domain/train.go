package domain

// TrainingOutcome describes what happened during Train().
type TrainingOutcome int

const (
	TrainingOK TrainingOutcome = iota
	TrainingNotEnoughEnergy
)

// TrainResult is a pure data result (UI/log formatting lives in transport layer).
type TrainResult struct {
	Outcome     TrainingOutcome `json:"outcome"`
	XPGain      int64           `json:"xp_gain"`
	CoinsGain   int64           `json:"coins_gain"`
	EnergyCost  int             `json:"energy_cost"`
	Crit        bool            `json:"crit"`
	EffPercent  int             `json:"efficiency_percent"` // legacy field; Training v2 always reports 100
	LeveledUp   int             `json:"levels_gained"`      // how many levels gained in one action
	StatsGained StatDelta       `json:"stats_gained"`

	Encounter Encounter `json:"encounter"`
	Flavor    uint16    `json:"flavor"`
}
