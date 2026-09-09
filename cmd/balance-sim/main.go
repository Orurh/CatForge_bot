package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

func main() {
	battles := flag.Int("battles", 1000, "number of deterministic seeds per scenario")
	level := flag.Int("level", 0, "single level to simulate; zero means levels 1..20")
	engineAddress := flag.String("engine-address", "127.0.0.1:50051", "C++ game engine gRPC address")
	flag.Parse()
	if *battles <= 0 || *level < 0 || *level > 20 {
		fmt.Fprintln(os.Stderr, "battles must be positive and level must be between 0 and 20")
		os.Exit(2)
	}

	levels := make([]int, 0, 20)
	if *level > 0 {
		levels = append(levels, *level)
	} else {
		for current := 1; current <= 20; current++ {
			levels = append(levels, current)
		}
	}
	breeds := []domain.Breed{domain.BreedMaineCoon, domain.BreedSiamese, domain.BreedBritish, domain.BreedBengal}
	locations := []domain.ExpeditionLocation{domain.ExpeditionAlley, domain.ExpeditionRooftop, domain.ExpeditionPark}
	difficulties := []domain.ExpeditionDifficulty{domain.ExpeditionEasy, domain.ExpeditionNormal, domain.ExpeditionHard}
	engine, err := gameengine.NewRemoteEngine(*engineAddress)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer engine.Close()

	fmt.Println("breed\tlevel\tlocation\tdifficulty\twin_rate\tavg_rounds\tavg_hp\txp_per_energy\tcoins_per_energy")
	for _, currentLevel := range levels {
		for _, breed := range breeds {
			for _, location := range locations {
				for _, difficulty := range difficulties {
					report, err := gameengine.SimulateBalance(context.Background(), engine, gameengine.BalanceScenario{
						Breed: breed, Level: currentLevel, Location: location, Difficulty: difficulty, Battles: *battles,
					})
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						os.Exit(1)
					}
					fmt.Printf("%s\t%d\t%s\t%s\t%.3f\t%.2f\t%.2f\t%.3f\t%.3f\n",
						breed, currentLevel, location, difficulty, report.WinRate, report.AverageRounds,
						report.AverageHPLeft, report.XPPerEnergy, report.CoinsPerEnergy)
				}
			}
		}
	}
}
