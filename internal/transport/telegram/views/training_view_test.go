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
	if !canTrain || !strings.Contains(text, "Расход: весь запас, 75 энергии") {
		t.Fatalf("FormatTrainingScreen() = %q, canTrain=%t", text, canTrain)
	}
}

func TestFormatTrainingScreenShowsMinimumWhenEnergyIsLow(t *testing.T) {
	t.Parallel()
	now := time.Unix(100, 0)
	text, canTrain := FormatTrainingScreen(&domain.Cat{
		Name: "Батон", Level: 1, Energy: 49, EnergyUpdatedAt: now,
	}, now)
	if canTrain || !strings.Contains(text, "Тренироваться можно с 50") || strings.Contains(text, "стоимость сейчас: 0") {
		t.Fatalf("FormatTrainingScreen() = %q, canTrain=%t", text, canTrain)
	}
}

func TestFormatTrainingResultPrefersGeneratedNarrative(t *testing.T) {
	t.Parallel()
	text := FormatTrainingResultText(
		&domain.Cat{Name: "Барсик", Trait: domain.TraitBully},
		domain.TrainResult{Outcome: domain.TrainingOK, Encounter: domain.EncounterPigeon, XPGain: 135, CoinsGain: 9, EnergyCost: 90, EffPercent: 100},
		"тренировался подозрительно усердно после спора с Батоном.", time.Unix(100, 0),
	)
	if !strings.Contains(text, "после спора с Батоном") || !strings.Contains(text, "+135 XP") ||
		!strings.Contains(text, "−90 энергии") || strings.Contains(text, "монет") {
		t.Fatalf("FormatTrainingResultText() = %q", text)
	}
}

func TestEnergyUXStatesAndRecovery(t *testing.T) {
	now := time.Unix(10000, 0)
	for _, tt := range []struct {
		energy int
		want   string
	}{
		{49, "ещё отдыхает"}, {50, "уже может тренироваться"}, {74, "уже может тренироваться"}, {100, "Дальше энергия не копится"},
	} {
		if text := EnergyTrainingStatus("Мур", tt.energy, now, now); !strings.Contains(text, tt.want) {
			t.Fatal(text)
		}
	}
	text := EnergyRecoveryText("Мур", 0, now, now)
	for _, want := range []string{"6ч 40м", "13ч 20м", "энергия больше не копится"} {
		if !strings.Contains(text, want) {
			t.Fatal(text)
		}
	}
	if text := EnergyProfileStatus(49, now.Add(-7*time.Minute), now); !strings.Contains(text, "через ~1м") {
		t.Fatal(text)
	}
	if text := EnergyProfileStatus(100, now, now); !strings.Contains(text, "полная, дальше не копится") {
		t.Fatal(text)
	}
}
