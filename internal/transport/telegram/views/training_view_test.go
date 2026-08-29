package views

import (
	"strings"
	"testing"
	"time"

	"catforge/internal/domain"
)

func TestFormatTrainingScreenShowsDynamicEnergyCost(t *testing.T) {
	t.Parallel()
	now := time.Unix(100, 0)
	text, canTrain := FormatTrainingScreen(&domain.Cat{
		Name: "Барсик", Level: 2, Energy: 75, EnergyUpdatedAt: now,
	}, now)
	if !canTrain || !strings.Contains(text, "стоимость сейчас: 65") {
		t.Fatalf("FormatTrainingScreen() = %q, canTrain=%t", text, canTrain)
	}
}

func TestFormatTrainingScreenShowsMinimumWhenEnergyIsLow(t *testing.T) {
	t.Parallel()
	now := time.Unix(100, 0)
	text, canTrain := FormatTrainingScreen(&domain.Cat{
		Name: "Батон", Level: 1, Energy: 24, EnergyUpdatedAt: now,
	}, now)
	if canTrain || !strings.Contains(text, "минимум для тренировки: 25") || strings.Contains(text, "стоимость сейчас: 0") {
		t.Fatalf("FormatTrainingScreen() = %q, canTrain=%t", text, canTrain)
	}
}

func TestFormatTrainingResultPrefersGeneratedNarrative(t *testing.T) {
	t.Parallel()
	text := FormatTrainingResultText(
		&domain.Cat{Name: "Барсик", Trait: domain.TraitBully},
		domain.TrainResult{Outcome: domain.TrainingOK, Encounter: domain.EncounterPigeon, XPGain: 135, CoinsGain: 9, EnergyCost: 90, EffPercent: 100},
		"тренировался подозрительно усердно после спора с Батоном.",
	)
	if !strings.Contains(text, "после спора с Батоном") || !strings.Contains(text, "+135 XP") ||
		!strings.Contains(text, "−90 энергии") || !strings.Contains(text, "+9 монет") {
		t.Fatalf("FormatTrainingResultText() = %q", text)
	}
}
