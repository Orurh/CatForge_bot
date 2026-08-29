package domain

import "time"

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
	RulesVersion   uint32
	ContentVersion uint32
	CreatedAt      time.Time
}
