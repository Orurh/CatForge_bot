package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

type report struct {
	fights           int
	catAWins         int
	rounds           int
	winnerHP         int
	firstTurnWins    int
	critFights       int
	catACritFights   int
	catACritWins     int
	catANoCritFights int
	catANoCritWins   int
}

func main() {
	battles := flag.Int("battles", 2000, "deterministic seeds per matchup")
	baseLevel := flag.Int("base-level", 5, "level of the lower-level cat")
	diffsFlag := flag.String("diffs", "0,3,5", "comma-separated level advantages for cat A")
	engineAddress := flag.String("engine-address", "127.0.0.1:50051", "C++ game engine gRPC address")
	flag.Parse()
	if *battles <= 0 || *baseLevel < 1 || *baseLevel > 20 {
		fmt.Fprintln(os.Stderr, "battles must be positive and base-level must be between 1 and 20")
		os.Exit(2)
	}
	diffs, err := parseDiffs(*diffsFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	engine, err := gameengine.NewRemoteEngine(*engineAddress)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer engine.Close()

	breeds := []domain.Breed{domain.BreedMaineCoon, domain.BreedSiamese, domain.BreedBritish, domain.BreedBengal}
	fmt.Println("breed_a\tbreed_b\tlevel_a\tlevel_b\twin_rate_a\tavg_rounds\tavg_winner_hp\tfirst_turn_win_rate\tcrit_fight_rate\ta_win_with_crit\ta_win_without_crit")
	for _, diff := range diffs {
		levelA := *baseLevel + diff
		if levelA > 20 {
			continue
		}
		for _, breedA := range breeds {
			for _, breedB := range breeds {
				result, runErr := simulate(context.Background(), engine, breedA, breedB, levelA, *baseLevel, *battles)
				if runErr != nil {
					fmt.Fprintln(os.Stderr, runErr)
					os.Exit(1)
				}
				fmt.Printf("%s\t%s\t%d\t%d\t%.3f\t%.2f\t%.2f\t%.3f\t%.3f\t%.3f\t%.3f\n",
					breedA, breedB, levelA, *baseLevel,
					ratio(result.catAWins, result.fights), ratioInt(result.rounds, result.fights), ratioInt(result.winnerHP, result.fights),
					ratio(result.firstTurnWins, result.fights), ratio(result.critFights, result.fights),
					ratio(result.catACritWins, result.catACritFights), ratio(result.catANoCritWins, result.catANoCritFights),
				)
			}
		}
	}
}

func simulate(ctx context.Context, engine gameengine.Engine, breedA, breedB domain.Breed, levelA, levelB, battles int) (report, error) {
	catA := catAtLevel(1, breedA, levelA)
	catB := catAtLevel(2, breedB, levelB)
	var result report
	for seed := 1; seed <= battles; seed++ {
		fight, err := engine.Fight(ctx, gameengine.FightInput{
			RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion,
			Seed: uint64(seed), CatA: catA, CatB: catB,
		})
		if err != nil {
			return report{}, fmt.Errorf("fight %s L%d vs %s L%d seed %d: %w", breedA, levelA, breedB, levelB, seed, err)
		}
		result.fights++
		result.rounds += fight.Rounds
		if fight.WinnerCatID == catA.ID {
			result.catAWins++
			result.winnerHP += fight.FinalHPA
		} else {
			result.winnerHP += fight.FinalHPB
		}
		if len(fight.Turns) > 0 && fight.Turns[0].AttackerCatID == fight.WinnerCatID {
			result.firstTurnWins++
		}
		catACrit, anyCrit := false, false
		for _, turn := range fight.Turns {
			if !turn.Crit {
				continue
			}
			anyCrit = true
			if turn.AttackerCatID == catA.ID {
				catACrit = true
			}
		}
		if anyCrit {
			result.critFights++
		}
		if catACrit {
			result.catACritFights++
			if fight.WinnerCatID == catA.ID {
				result.catACritWins++
			}
		} else {
			result.catANoCritFights++
			if fight.WinnerCatID == catA.ID {
				result.catANoCritWins++
			}
		}
	}
	return result, nil
}

func catAtLevel(id int64, breed domain.Breed, level int) domain.Cat {
	hp, atk, def, spd := domain.BaseStatsByBreed(breed)
	delta := domain.LevelUpDelta(breed)
	levels := level - 1
	physical := domain.BaseFelineStats(breed)
	growth := domain.FelineGrowth(breed)
	for i := 0; i < levels; i++ {
		physical.Add(growth)
	}
	return domain.Cat{
		ID: id, Breed: breed, Level: level, Feline: physical,
		HPBase: hp + delta.HP*levels, ATKBase: atk + delta.ATK*levels,
		DEFBase: def + delta.DEF*levels, SPDBase: spd + delta.SPD*levels,
	}
}

func parseDiffs(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	diffs := make([]int, 0, len(parts))
	for _, part := range parts {
		diff, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || diff < 0 || diff > 19 {
			return nil, fmt.Errorf("invalid level difference %q", part)
		}
		diffs = append(diffs, diff)
	}
	if len(diffs) == 0 {
		return nil, fmt.Errorf("at least one level difference is required")
	}
	return diffs, nil
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func ratioInt(numerator, denominator int) float64 {
	return ratio(numerator, denominator)
}
