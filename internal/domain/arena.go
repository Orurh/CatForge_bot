package domain

import (
	"hash/crc32"
	"math"
	"time"
)

const (
	ArenaTicketsCap    = 3
	ArenaTicketRegen   = 8 * time.Hour
	ArenaBaseRating    = 1000
	ArenaWinRating     = 20
	ArenaLoseRating    = -15
	ArenaK             = 180.0 // чем больше, тем “плавнее”
)

type ArenaState struct {
	Tickets          int
	TicketsUpdatedAt time.Time
	Rating           int
	SeasonPoints     int
	Rage             int
}

type ArenaOpponent struct {
	UserID int64
	Name   string
	Breed  Breed
	Level  int
	Power  int
	Kind   ArenaOpponentKind 
}

type ArenaOpponentKind string

const (
	ArenaOppWeaker   ArenaOpponentKind = "weaker"
	ArenaOppEven     ArenaOpponentKind = "even"
	ArenaOppStronger ArenaOpponentKind = "stronger"
)

type ArenaFightResult struct {
	AttackerPower int
	DefenderPower int
	WinProb       float64
	AttackerWon   bool
	RatingDelta   int
	NewRating     int
}

func RegenArenaTickets(s ArenaState, now time.Time) ArenaState {
	if s.TicketsUpdatedAt.IsZero() {
		s.Tickets = ArenaTicketsCap
		s.TicketsUpdatedAt = now
		if s.Rating == 0 {
			s.Rating = ArenaBaseRating
		}
		return s
	}

	if s.Tickets >= ArenaTicketsCap {
		return s
	}

	delta := now.Sub(s.TicketsUpdatedAt)
	if delta <= 0 {
		return s
	}

	add := int(delta / ArenaTicketRegen)
	if add <= 0 {
		return s
	}

	s.Tickets += add
	if s.Tickets > ArenaTicketsCap {
		s.Tickets = ArenaTicketsCap
	}

	s.TicketsUpdatedAt = s.TicketsUpdatedAt.Add(time.Duration(add) * ArenaTicketRegen)
	return s
}

const (
    ArenaRageCap = 5
    ArenaRageBonusPerStack = 0.04 
)



func ArenaWinProbability(attackerPower, defenderPower int) float64 {
	// по разнице сил
	d := float64(attackerPower - defenderPower)
	return 1.0 / (1.0 + math.Exp(-d/ArenaK))
}

func ArenaResolve(seed string, attackerPower, defenderPower int) ArenaFightResult {
	p := ArenaWinProbability(attackerPower, defenderPower)

	h := crc32.ChecksumIEEE([]byte(seed))
	roll := float64(h%10000) / 10000.0

	win := roll < p
	delta := ArenaLoseRating
	if win {
		delta = ArenaWinRating
	}

	return ArenaFightResult{
		AttackerPower: attackerPower,
		DefenderPower: defenderPower,
		WinProb:       p,
		AttackerWon:   win,
		RatingDelta:   delta,
	}
}

func clamp01(x float64) float64 {
    if x < 0 { return 0 }
    if x > 1 { return 1 }
    return x
}

func ArenaWinProbabilityWithRage(attackerPower, defenderPower int, rage int) float64 {
    p := ArenaWinProbability(attackerPower, defenderPower)
    if rage <= 0 {
        return p
    }
    if rage > ArenaRageCap {
        rage = ArenaRageCap
    }
    p = p + float64(rage)*ArenaRageBonusPerStack
    return clamp01(p)
}

func ArenaResolveWithRage(seed string, attackerPower, defenderPower int, rage int) ArenaFightResult {
    p := ArenaWinProbabilityWithRage(attackerPower, defenderPower, rage)
    h := crc32.ChecksumIEEE([]byte(seed))
    roll := float64(h%10000) / 10000.0
    win := roll < p
    delta := ArenaLoseRating
    if win { delta = ArenaWinRating }
    return ArenaFightResult{
        AttackerPower: attackerPower,
        DefenderPower: defenderPower,
        WinProb: p,
        AttackerWon: win,
        RatingDelta: delta,
    }
}

