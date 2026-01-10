package domain

// Encounter describes what the cat met during hunt (pure data; text lives in transport).
type Encounter string

const (
	EncounterMicePack Encounter = "mice_pack"
	EncounterPigeon   Encounter = "pigeon"
	EncounterLizard   Encounter = "lizard"
	EncounterBigRat   Encounter = "big_rat" // implies crit
)
