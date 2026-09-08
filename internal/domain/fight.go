package domain

import "time"

type FightKind string

const (
	FightKindRegular FightKind = "regular"
	FightKindRevenge FightKind = "revenge"
)

type FightTurn struct {
	Round           int   `json:"round"`
	AttackerCatID   int64 `json:"attacker_cat_id"`
	DefenderCatID   int64 `json:"defender_cat_id"`
	Damage          int   `json:"damage"`
	Crit            bool  `json:"crit"`
	DefenderHPAfter int   `json:"defender_hp_after"`
}

type FightResult struct {
	WinnerCatID int64       `json:"winner_cat_id"`
	LoserCatID  int64       `json:"loser_cat_id"`
	Rounds      int         `json:"rounds"`
	FinalHPA    int         `json:"final_hp_a"`
	FinalHPB    int         `json:"final_hp_b"`
	Turns       []FightTurn `json:"turns"`
}

type FightQueueEntry struct {
	YardID    int64
	UserID    int64
	CatID     int64
	QueuedAt  time.Time
	ExpiresAt time.Time
}

type FightQueueStatus string

const (
	FightQueueWaiting  FightQueueStatus = "waiting"
	FightQueueCanceled FightQueueStatus = "canceled"
	FightQueueMatched  FightQueueStatus = "matched"
)

type FightQueueToggle struct {
	Status   FightQueueStatus
	Opponent FightQueueEntry
}

type FightRecord struct {
	CatAVersion    int64
	CatBVersion    int64
	ID             int64
	YardID         int64
	CatAID         int64
	CatBID         int64
	WinnerCatID    int64
	LoserCatID     int64
	Seed           int64
	Rounds         int
	FinalHPA       int
	FinalHPB       int
	RivalryDelta   int
	CatAXPGain     int64
	CatBXPGain     int64
	Kind           FightKind
	ParentFightID  int64
	RulesVersion   uint32
	ContentVersion uint32
	CreatedAt      time.Time
}

type FightLimits struct {
	DailySince   time.Time
	DailyLimit   int
	PairSince    time.Time
	PairLimit    int
	RevengeSince time.Time
}

type FightRevenge struct {
	SourceFightID int64
	YardID        int64
	LoserUserID   int64
	LoserCatID    int64
	WinnerUserID  int64
	WinnerCatID   int64
}

type FightStats struct {
	CatAWins     int
	CatALosses   int
	CatBWins     int
	CatBLosses   int
	PairCatAWins int
	PairCatBWins int
	// PairWinnerStreak is the current consecutive win streak of the winner of
	// the fight that produced these stats.
	PairWinnerStreak int
}

type FightSaveResult struct {
	CatAFacts  []string
	CatBFacts  []string
	FightID    int64
	Friendship int
	Rivalry    int
	Respect    int
	Stats      FightStats
}

type ArenaStats struct {
	Fights        int
	Wins          int
	Losses        int
	CurrentStreak int
	StreakWins    bool
}
