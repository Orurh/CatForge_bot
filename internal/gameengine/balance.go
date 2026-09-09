package gameengine

import (
	"context"
	"fmt"
	"time"

	"catforge/internal/domain"
)

type BalanceScenario struct {
	Breed      domain.Breed
	Level      int
	Location   domain.ExpeditionLocation
	Difficulty domain.ExpeditionDifficulty
	Battles    int
}

type BalanceReport struct {
	Scenario       BalanceScenario
	Wins           int
	WinRate        float64
	AverageRounds  float64
	AverageHPLeft  float64
	XPPerEnergy    float64
	CoinsPerEnergy float64
}

func SimulateBalance(ctx context.Context, engine Engine, scenario BalanceScenario) (BalanceReport, error) {
	if scenario.Battles <= 0 || scenario.Level < 1 {
		return BalanceReport{}, fmt.Errorf("invalid balance scenario")
	}

	cat := catAtLevel(scenario.Breed, scenario.Level)
	now := time.Unix(1_000_000, 0)
	var wins, rounds, hpLeft int
	var xp, coins, energy int64
	for seed := 1; seed <= scenario.Battles; seed++ {
		out, err := engine.Expedition(ctx, ExpeditionInput{
			RulesVersion:   CurrentRulesVersion,
			ContentVersion: CurrentContentVersion,
			Cat:            cat,
			Now:            now,
			Seed:           uint64(seed),
			Location:       scenario.Location,
			Difficulty:     scenario.Difficulty,
		})
		if err != nil {
			return BalanceReport{}, err
		}
		if out.Result.Outcome == domain.ExpeditionVictory {
			wins++
		}
		rounds += out.Result.Rounds
		hpLeft += out.Result.CatHPAfter
		xp += out.Result.XPGain
		coins += out.Result.CoinsGain
		energy += int64(out.Result.EnergyCost)
	}

	battles := float64(scenario.Battles)
	report := BalanceReport{
		Scenario:      scenario,
		Wins:          wins,
		WinRate:       float64(wins) / battles,
		AverageRounds: float64(rounds) / battles,
		AverageHPLeft: float64(hpLeft) / battles,
	}
	if energy > 0 {
		report.XPPerEnergy = float64(xp) / float64(energy)
		report.CoinsPerEnergy = float64(coins) / float64(energy)
	}
	return report, nil
}

func catAtLevel(breed domain.Breed, level int) domain.Cat {
	hp, atk, def, spd := domain.BaseStatsByBreed(breed)
	cat := domain.Cat{
		Breed: breed, Level: 1, Energy: domain.EnergyMax,
		HPBase: hp, ATKBase: atk, DEFBase: def, SPDBase: spd,
	}
	for cat.Level < level {
		cat.Level++
		domain.ApplyLevelUps(&cat, 1)
	}
	return cat
}
