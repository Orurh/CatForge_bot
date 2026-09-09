package gameengine

import (
	"reflect"
	"testing"

	"catforge/internal/domain"
	gameenginev1 "catforge/internal/gen/gameengine/v1"
)

func TestExpeditionResultFromProtoMapsRareEnemy(t *testing.T) {
	t.Parallel()
	result := expeditionResultFromProto(&gameenginev1.ExpeditionResult{
		Outcome: gameenginev1.ExpeditionOutcome_EXPEDITION_OUTCOME_VICTORY,
		Enemy: &gameenginev1.Enemy{
			Kind: gameenginev1.EnemyKind_ENEMY_KIND_RAT_ACCOUNTANT, Level: 3, Hp: 50, Atk: 20, Def: 18, Spd: 14,
		},
		Location:   gameenginev1.ExpeditionLocation_EXPEDITION_LOCATION_ALLEY,
		Difficulty: gameenginev1.ExpeditionDifficulty_EXPEDITION_DIFFICULTY_NORMAL,
	})
	if result.Enemy.Kind != domain.EnemyRatAccountant || !domain.IsRareEnemy(result.Enemy.Kind) {
		t.Fatalf("rare enemy mapping = %+v", result.Enemy)
	}
}

func TestCatProtoRoundTrip(t *testing.T) {
	t.Parallel()
	want := domain.Cat{ID: 1, UserID: 2, StateVersion: 3, Name: "Мур", Breed: domain.BreedSiamese, Trait: "sleepy", Level: 4, XP: 55, Coins: 12, Energy: 80, HPBase: 40, ATKBase: 20, DEFBase: 18, SPDBase: 25}
	want.Feline = want.PhysicalStats()
	got := catFromProto(catToProto(want))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cat round trip = %+v, want %+v", got, want)
	}
}

func TestTrainingResultFromProtoMapsCoinReward(t *testing.T) {
	t.Parallel()
	result := resultFromProto(&gameenginev1.TrainResult{
		Outcome: gameenginev1.TrainingOutcome_TRAINING_OUTCOME_OK,
		XpGain:  140, CoinsGain: 9, EnergyCost: 90,
	})
	if result.XPGain != 140 || result.CoinsGain != 9 || result.EnergyCost != 90 {
		t.Fatalf("training result mapping = %+v", result)
	}
}

func TestYardEventResultFromProto(t *testing.T) {
	t.Parallel()
	got := yardEventResultFromProto(&gameenginev1.ResolveYardEventResponse{
		Success: true, OutcomeTier: gameenginev1.YardEventOutcomeTier_YARD_EVENT_OUTCOME_TIER_EXCEPTIONAL,
		TeamScore: 41, TargetScore: 34, YardScore: 20, XpGain: 30, SecretFound: true, StrategyBonus: 6,
		Participants: []*gameenginev1.YardEventParticipantResult{{
			CatId: 9, Choice: gameenginev1.YardEventChoice_YARD_EVENT_CHOICE_SCOUT,
			Contribution: 15, Mvp: true,
		}},
		RelationshipEffects: []*gameenginev1.YardRelationshipEffect{{
			CatAId: 9, CatBId: 10, FriendshipDelta: 1, RespectDelta: 1,
		}},
	})
	if got.OutcomeTier != domain.YardOutcomeExceptional || got.YardScore != 20 || got.XPGain != 30 || got.TeamScore != 41 || len(got.Participants) != 1 ||
		got.Participants[0].Choice != domain.YardChoiceScout || !got.Participants[0].MVP ||
		len(got.RelationshipEffects) != 1 || got.RelationshipEffects[0].RespectDelta != 1 {
		t.Fatalf("yard event result mapping = %+v", got)
	}
}

func TestYardEventTypeToProtoMapsAllTemplates(t *testing.T) {
	t.Parallel()
	tests := map[domain.YardEventType]gameenginev1.YardEventType{
		domain.YardEventFishTruck: gameenginev1.YardEventType_YARD_EVENT_TYPE_FISH_TRUCK,
		domain.YardEventBigDog:    gameenginev1.YardEventType_YARD_EVENT_TYPE_BIG_DOG,
		domain.YardEventBigBox:    gameenginev1.YardEventType_YARD_EVENT_TYPE_BIG_BOX,
	}
	for eventType, want := range tests {
		if got := yardEventTypeToProto(eventType); got != want {
			t.Errorf("yardEventTypeToProto(%q) = %v, want %v", eventType, got, want)
		}
	}
}

func TestFightResultFromProto(t *testing.T) {
	t.Parallel()
	got := fightResultFromProto(&gameenginev1.FightResponse{
		WinnerCatId: 10, LoserCatId: 20, Rounds: 3, FinalHpA: 9, FinalHpB: 0,
		Turns: []*gameenginev1.FightTurn{{
			Round: 3, AttackerCatId: 10, DefenderCatId: 20, Damage: 11, Crit: true, DefenderHpAfter: 0,
		}},
	})
	if got.WinnerCatID != 10 || got.LoserCatID != 20 || got.Rounds != 3 || len(got.Turns) != 1 || !got.Turns[0].Crit {
		t.Fatalf("fight result mapping = %+v", got)
	}
}
