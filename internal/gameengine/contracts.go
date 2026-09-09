// Package gameengine contains the Go application boundary and gRPC client for
// the single C++ game engine.
package gameengine

import (
	"context"
	"time"

	"catforge/internal/domain"
	"catforge/internal/pkg/randx"
)

const CurrentRulesVersion uint32 = 14
const CurrentContentVersion uint32 = 5

type TrainingRandom struct {
	TrainingLootRoll int
	TrainingCritRoll int
	EnergyCostRoll   int
	XPGainRoll       int
	EncounterRoll    int
	FlavorRoll       uint16
}

type TrainingInput struct {
	TrainingCritBonusPercent int
	RulesVersion             uint32
	ContentVersion           uint32
	Cat                      domain.Cat
	Now                      time.Time
	Random                   TrainingRandom
}

type TrainingOutput struct {
	Cat    domain.Cat
	Result domain.TrainResult
}

type ExpeditionInput struct {
	RulesVersion   uint32
	ContentVersion uint32
	Cat            domain.Cat
	Now            time.Time
	Seed           uint64
	Location       domain.ExpeditionLocation
	Difficulty     domain.ExpeditionDifficulty
	EquipmentBonus domain.StatDelta
	LootCounts     LootCounts
	ForceLoot      bool
}

type LootCounts struct {
	Common int
	Rare   int
	Epic   int
}

type ExpeditionOutput struct {
	Cat    domain.Cat
	Result domain.ExpeditionResult
}

type YardEventInput struct {
	RulesVersion   uint32
	ContentVersion uint32
	Seed           uint64
	EventType      domain.YardEventType
	Participants   []domain.YardEventParticipant
}

type FightInput struct {
	RulesVersion   uint32
	ContentVersion uint32
	Seed           uint64
	CatA           domain.Cat
	CatB           domain.Cat
}

// Engine calculates state transitions but never persists them.
type Engine interface {
	Train(ctx context.Context, input TrainingInput) (TrainingOutput, error)
	Expedition(ctx context.Context, input ExpeditionInput) (ExpeditionOutput, error)
	ResolveYardEvent(ctx context.Context, input YardEventInput) (domain.YardEventResult, error)
	Fight(ctx context.Context, input FightInput) (domain.FightResult, error)
}

func RollTraining(rng randx.RNG) TrainingRandom {
	return TrainingRandom{
		TrainingLootRoll: rng.Intn(10000),
		EnergyCostRoll:   0,
		TrainingCritRoll: rng.Intn(100),
		XPGainRoll:       rng.Intn(51),
		EncounterRoll:    rng.Intn(100),
		FlavorRoll:       uint16(rng.Intn(1 << 16)),
	}
}

type ProgressInput struct {
	TrainingLootRoll int
	Cat              domain.Cat
	XPGain           int64
	Seed             uint64
	Source           string
	OutcomeTier      string
	SecretFound      bool
	EnergySpent      int
}
type ProgressionEngine interface {
	Progress(context.Context, ProgressInput) (domain.Cat, error)
}
