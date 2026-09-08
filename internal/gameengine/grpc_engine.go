package gameengine

import (
	"catforge/internal/observability"
	"context"
	"errors"
	"fmt"
	"time"

	"catforge/internal/domain"
	gameenginev1 "catforge/internal/gen/gameengine/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultRemoteTimeout = 2 * time.Second

type GRPCEngine struct {
	conn   *grpc.ClientConn
	client gameenginev1.GameEngineServiceClient
}

func NewRemoteEngine(address string) (*GRPCEngine, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(observability.UnaryClientInterceptor))
	if err != nil {
		return nil, fmt.Errorf("create game engine client: %w", err)
	}
	return &GRPCEngine{conn: conn, client: gameenginev1.NewGameEngineServiceClient(conn)}, nil
}

func newGRPCEngineForClient(client gameenginev1.GameEngineServiceClient) *GRPCEngine {
	return &GRPCEngine{client: client}
}

func (e *GRPCEngine) Close() error {
	if e.conn == nil {
		return nil
	}
	return e.conn.Close()
}

func (e *GRPCEngine) Train(ctx context.Context, input TrainingInput) (TrainingOutput, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultRemoteTimeout)
		defer cancel()
	}

	response, err := e.client.Train(ctx, trainingRequestToProto(input))
	if err != nil {
		return TrainingOutput{}, fmt.Errorf("remote game engine train: %w", err)
	}
	if response.GetCat() == nil || response.GetResult() == nil {
		return TrainingOutput{}, errors.New("remote game engine returned an incomplete response")
	}
	cat := catFromProto(response.GetCat())
	cat.LootItemID = response.GetResult().GetLootItemId()
	cat.ProgressionFacts = append(response.GetResult().GetProgressionFacts(), domain.CrossedUnlocks(input.Cat.Level, cat.Level)...)
	return TrainingOutput{
		Cat:    cat,
		Result: resultFromProto(response.GetResult()),
	}, nil
}

func (e *GRPCEngine) Expedition(ctx context.Context, input ExpeditionInput) (ExpeditionOutput, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultRemoteTimeout)
		defer cancel()
	}

	response, err := e.client.Expedition(ctx, &gameenginev1.ExpeditionRequest{
		RulesVersion:   input.RulesVersion,
		ContentVersion: input.ContentVersion,
		Cat:            catToProto(input.Cat),
		NowUnixNanos:   input.Now.UnixNano(),
		Seed:           input.Seed,
		Location:       expeditionLocationToProto(input.Location),
		Difficulty:     expeditionDifficultyToProto(input.Difficulty),
		EquipmentBonus: statDeltaToProto(input.EquipmentBonus),
		LootCounts: &gameenginev1.LootCounts{
			Common: int32(input.LootCounts.Common), Rare: int32(input.LootCounts.Rare), Epic: int32(input.LootCounts.Epic),
		},
		ForceLoot: input.ForceLoot,
	})
	if err != nil {
		return ExpeditionOutput{}, fmt.Errorf("remote game engine expedition: %w", err)
	}
	if response.GetCat() == nil || response.GetResult() == nil {
		return ExpeditionOutput{}, errors.New("remote game engine returned an incomplete expedition response")
	}
	return ExpeditionOutput{
		Cat:    catFromProto(response.GetCat()),
		Result: expeditionResultFromProto(response.GetResult()),
	}, nil
}

func (e *GRPCEngine) ResolveYardEvent(ctx context.Context, input YardEventInput) (domain.YardEventResult, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultRemoteTimeout)
		defer cancel()
	}
	participants := make([]*gameenginev1.YardEventParticipant, 0, len(input.Participants))
	for _, participant := range input.Participants {
		participants = append(participants, &gameenginev1.YardEventParticipant{
			CatId: participant.CatID, Choice: yardEventChoiceToProto(participant.Choice),
			Level: int32(participant.Level), Hp: int32(participant.HP), Atk: int32(participant.ATK),
			Def: int32(participant.DEF), Spd: int32(participant.SPD),
			Feline: felineToProto(participant.Feline), Effects: participant.Effects, SpecialAction: participant.SpecialAction,
		})
	}
	response, err := e.client.ResolveYardEvent(ctx, &gameenginev1.ResolveYardEventRequest{
		RulesVersion: input.RulesVersion, ContentVersion: input.ContentVersion, Seed: input.Seed,
		EventType: yardEventTypeToProto(input.EventType), Participants: participants,
	})
	if err != nil {
		return domain.YardEventResult{}, fmt.Errorf("remote game engine resolve yard event: %w", err)
	}
	return yardEventResultFromProto(response), nil
}

func (e *GRPCEngine) Fight(ctx context.Context, input FightInput) (domain.FightResult, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultRemoteTimeout)
		defer cancel()
	}
	response, err := e.client.Fight(ctx, &gameenginev1.FightRequest{
		RulesVersion: input.RulesVersion, ContentVersion: input.ContentVersion, Seed: input.Seed,
		CatA: catToProto(input.CatA), CatB: catToProto(input.CatB),
	})
	if err != nil {
		return domain.FightResult{}, fmt.Errorf("remote game engine fight: %w", err)
	}
	return fightResultFromProto(response), nil
}

func fightResultFromProto(value *gameenginev1.FightResponse) domain.FightResult {
	result := domain.FightResult{
		WinnerCatID: value.GetWinnerCatId(), LoserCatID: value.GetLoserCatId(),
		Rounds: int(value.GetRounds()), FinalHPA: int(value.GetFinalHpA()), FinalHPB: int(value.GetFinalHpB()),
		Turns: make([]domain.FightTurn, 0, len(value.GetTurns())),
	}
	for _, turn := range value.GetTurns() {
		result.Turns = append(result.Turns, domain.FightTurn{
			Round: int(turn.GetRound()), AttackerCatID: turn.GetAttackerCatId(), DefenderCatID: turn.GetDefenderCatId(),
			Damage: int(turn.GetDamage()), Crit: turn.GetCrit(), DefenderHPAfter: int(turn.GetDefenderHpAfter()),
		})
	}
	return result
}

func yardEventResultFromProto(value *gameenginev1.ResolveYardEventResponse) domain.YardEventResult {
	tier := yardEventOutcomeTierFromProto(value.GetOutcomeTier())
	if tier == domain.YardOutcomeFailure && value.GetSuccess() {
		// Rolling deploy compatibility with rules-v9 engines.
		tier = domain.YardOutcomeSuccess
	}
	result := domain.YardEventResult{
		OutcomeTier: tier,
		TeamScore:   int(value.GetTeamScore()), TargetScore: int(value.GetTargetScore()),
		YardScore: int(value.GetYardScore()), XPGain: value.GetXpGain(),
		SecretFound: value.GetSecretFound(), StrategyBonus: int(value.GetStrategyBonus()),
		Participants:        make([]domain.YardEventParticipantResult, 0, len(value.GetParticipants())),
		RelationshipEffects: make([]domain.YardRelationshipEffect, 0, len(value.GetRelationshipEffects())),
	}
	for _, participant := range value.GetParticipants() {
		result.Participants = append(result.Participants, domain.YardEventParticipantResult{
			CatID: participant.GetCatId(), Choice: yardEventChoiceFromProto(participant.GetChoice()),
			Contribution: int(participant.GetContribution()), MVP: participant.GetMvp(), ItemEffectTriggered: participant.GetItemEffectTriggered(),
		})
	}
	for _, effect := range value.GetRelationshipEffects() {
		result.RelationshipEffects = append(result.RelationshipEffects, domain.YardRelationshipEffect{
			CatAID: effect.GetCatAId(), CatBID: effect.GetCatBId(),
			FriendshipDelta: int(effect.GetFriendshipDelta()), RivalryDelta: int(effect.GetRivalryDelta()),
			RespectDelta: int(effect.GetRespectDelta()),
		})
	}
	return result
}

func yardEventOutcomeTierFromProto(value gameenginev1.YardEventOutcomeTier) domain.YardEventOutcomeTier {
	switch value {
	case gameenginev1.YardEventOutcomeTier_YARD_EVENT_OUTCOME_TIER_PARTIAL:
		return domain.YardOutcomePartial
	case gameenginev1.YardEventOutcomeTier_YARD_EVENT_OUTCOME_TIER_SUCCESS:
		return domain.YardOutcomeSuccess
	case gameenginev1.YardEventOutcomeTier_YARD_EVENT_OUTCOME_TIER_EXCEPTIONAL:
		return domain.YardOutcomeExceptional
	default:
		return domain.YardOutcomeFailure
	}
}

func yardEventTypeToProto(value domain.YardEventType) gameenginev1.YardEventType {
	switch value {
	case domain.YardEventFishTruck:
		return gameenginev1.YardEventType_YARD_EVENT_TYPE_FISH_TRUCK
	case domain.YardEventBigDog:
		return gameenginev1.YardEventType_YARD_EVENT_TYPE_BIG_DOG
	case domain.YardEventBigBox:
		return gameenginev1.YardEventType_YARD_EVENT_TYPE_BIG_BOX
	default:
		return gameenginev1.YardEventType_YARD_EVENT_TYPE_UNSPECIFIED
	}
}

func yardEventChoiceToProto(value domain.YardEventChoiceID) gameenginev1.YardEventChoice {
	switch value {
	case domain.YardChoiceSteal:
		return gameenginev1.YardEventChoice_YARD_EVENT_CHOICE_STEAL
	case domain.YardChoiceDistract:
		return gameenginev1.YardEventChoice_YARD_EVENT_CHOICE_DISTRACT
	case domain.YardChoiceScout:
		return gameenginev1.YardEventChoice_YARD_EVENT_CHOICE_SCOUT
	default:
		return gameenginev1.YardEventChoice_YARD_EVENT_CHOICE_UNSPECIFIED
	}
}

func yardEventChoiceFromProto(value gameenginev1.YardEventChoice) domain.YardEventChoiceID {
	switch value {
	case gameenginev1.YardEventChoice_YARD_EVENT_CHOICE_STEAL:
		return domain.YardChoiceSteal
	case gameenginev1.YardEventChoice_YARD_EVENT_CHOICE_DISTRACT:
		return domain.YardChoiceDistract
	case gameenginev1.YardEventChoice_YARD_EVENT_CHOICE_SCOUT:
		return domain.YardChoiceScout
	default:
		return ""
	}
}

func trainingRequestToProto(input TrainingInput) *gameenginev1.TrainRequest {
	return &gameenginev1.TrainRequest{
		TrainingCritBonusPercent: int32(input.TrainingCritBonusPercent),
		RulesVersion:             input.RulesVersion,
		ContentVersion:           input.ContentVersion,
		Cat:                      catToProto(input.Cat),
		NowUnixNanos:             input.Now.UnixNano(),
		Random: &gameenginev1.TrainingRandom{
			TrainingLootRoll: int32(input.Random.TrainingLootRoll),
			TrainingCritRoll: int32(input.Random.TrainingCritRoll),
			EnergyCostRoll:   int32(input.Random.EnergyCostRoll),
			XpGainRoll:       int32(input.Random.XPGainRoll),
			EncounterRoll:    int32(input.Random.EncounterRoll),
			FlavorRoll:       uint32(input.Random.FlavorRoll),
		},
	}
}

func catToProto(cat domain.Cat) *gameenginev1.CatState {
	value := &gameenginev1.CatState{
		Id:           cat.ID,
		UserId:       cat.UserID,
		StateVersion: cat.StateVersion,
		Name:         cat.Name,
		Breed:        string(cat.Breed),
		Trait:        string(cat.Trait),
		Level:        int32(cat.Level),
		Xp:           cat.XP,
		Coins:        cat.Coins,
		Energy:       int32(cat.Energy),
		HpBase:       int32(cat.HPBase),
		AtkBase:      int32(cat.ATKBase),
		DefBase:      int32(cat.DEFBase),
		SpdBase:      int32(cat.SPDBase),
		Feline:       felineToProto(cat.PhysicalStats()), FirstItemGranted: cat.FirstItemGranted,
	}
	if !cat.LastTrainAt.IsZero() {
		nanos := cat.LastTrainAt.UnixNano()
		value.LastTrainAtUnixNanos = &nanos
	}
	if !cat.EnergyUpdatedAt.IsZero() {
		nanos := cat.EnergyUpdatedAt.UnixNano()
		value.EnergyUpdatedAtUnixNanos = &nanos
	}
	return value
}

func catFromProto(value *gameenginev1.CatState) domain.Cat {
	cat := domain.Cat{
		ID:           value.GetId(),
		UserID:       value.GetUserId(),
		StateVersion: value.GetStateVersion(),
		Name:         value.GetName(),
		Breed:        domain.Breed(value.GetBreed()),
		Trait:        domain.Trait(value.GetTrait()),
		Level:        int(value.GetLevel()),
		XP:           value.GetXp(),
		Coins:        value.GetCoins(),
		Energy:       int(value.GetEnergy()),
		HPBase:       int(value.GetHpBase()),
		ATKBase:      int(value.GetAtkBase()),
		DEFBase:      int(value.GetDefBase()),
		SPDBase:      int(value.GetSpdBase()),
		Feline:       felineFromProto(value.GetFeline()), FirstItemGranted: value.GetFirstItemGranted(),
	}
	if value.LastTrainAtUnixNanos != nil {
		cat.LastTrainAt = time.Unix(0, value.GetLastTrainAtUnixNanos())
	}
	if value.EnergyUpdatedAtUnixNanos != nil {
		cat.EnergyUpdatedAt = time.Unix(0, value.GetEnergyUpdatedAtUnixNanos())
	}
	return cat
}

func resultFromProto(value *gameenginev1.TrainResult) domain.TrainResult {
	stats := value.GetStatsGained()
	result := domain.TrainResult{
		XPGain:     value.GetXpGain(),
		CoinsGain:  value.GetCoinsGain(),
		EnergyCost: int(value.GetEnergyCost()),
		Crit:       value.GetCrit(),
		EffPercent: int(value.GetEfficiencyPercent()),
		LeveledUp:  int(value.GetLevelsGained()),
		Flavor:     uint16(value.GetFlavor()),
	}
	if value.GetOutcome() == gameenginev1.TrainingOutcome_TRAINING_OUTCOME_NOT_ENOUGH_ENERGY {
		result.Outcome = domain.TrainingNotEnoughEnergy
	}
	if stats != nil {
		result.StatsGained = domain.StatDelta{
			HP: int(stats.GetHp()), ATK: int(stats.GetAtk()), DEF: int(stats.GetDef()), SPD: int(stats.GetSpd()),
		}
	}
	switch value.GetEncounter() {
	case gameenginev1.Encounter_ENCOUNTER_MICE_PACK:
		result.Encounter = domain.EncounterMicePack
	case gameenginev1.Encounter_ENCOUNTER_PIGEON:
		result.Encounter = domain.EncounterPigeon
	case gameenginev1.Encounter_ENCOUNTER_LIZARD:
		result.Encounter = domain.EncounterLizard
	case gameenginev1.Encounter_ENCOUNTER_BIG_RAT:
		result.Encounter = domain.EncounterBigRat
	}
	return result
}

func expeditionResultFromProto(value *gameenginev1.ExpeditionResult) domain.ExpeditionResult {
	result := domain.ExpeditionResult{
		EnergyCost:   int(value.GetEnergyCost()),
		XPGain:       value.GetXpGain(),
		Rounds:       int(value.GetRounds()),
		CatHPAfter:   int(value.GetCatHpAfter()),
		EnemyHPAfter: int(value.GetEnemyHpAfter()),
		LeveledUp:    int(value.GetLevelsGained()),
		CoinsGain:    value.GetCoinsGain(),
	}
	if loot := value.GetLoot(); loot != nil && loot.GetDropped() {
		result.Loot.Dropped = true
		result.Loot.ItemIndex = int(loot.GetItemIndex())
		switch loot.GetRarity() {
		case gameenginev1.ItemRarity_ITEM_RARITY_RARE:
			result.Loot.Rarity = domain.ItemRare
		case gameenginev1.ItemRarity_ITEM_RARITY_EPIC:
			result.Loot.Rarity = domain.ItemEpic
		default:
			result.Loot.Rarity = domain.ItemCommon
		}
	}
	switch value.GetOutcome() {
	case gameenginev1.ExpeditionOutcome_EXPEDITION_OUTCOME_DEFEAT:
		result.Outcome = domain.ExpeditionDefeat
	case gameenginev1.ExpeditionOutcome_EXPEDITION_OUTCOME_NOT_ENOUGH_ENERGY:
		result.Outcome = domain.ExpeditionNotEnoughEnergy
	default:
		result.Outcome = domain.ExpeditionVictory
	}
	switch value.GetLocation() {
	case gameenginev1.ExpeditionLocation_EXPEDITION_LOCATION_ALLEY:
		result.Location = domain.ExpeditionAlley
	case gameenginev1.ExpeditionLocation_EXPEDITION_LOCATION_ROOFTOP:
		result.Location = domain.ExpeditionRooftop
	case gameenginev1.ExpeditionLocation_EXPEDITION_LOCATION_PARK:
		result.Location = domain.ExpeditionPark
	}
	switch value.GetDifficulty() {
	case gameenginev1.ExpeditionDifficulty_EXPEDITION_DIFFICULTY_EASY:
		result.Difficulty = domain.ExpeditionEasy
	case gameenginev1.ExpeditionDifficulty_EXPEDITION_DIFFICULTY_NORMAL:
		result.Difficulty = domain.ExpeditionNormal
	case gameenginev1.ExpeditionDifficulty_EXPEDITION_DIFFICULTY_HARD:
		result.Difficulty = domain.ExpeditionHard
	}
	if enemy := value.GetEnemy(); enemy != nil {
		result.Enemy = domain.Enemy{
			Level: int(enemy.GetLevel()), HP: int(enemy.GetHp()), ATK: int(enemy.GetAtk()),
			DEF: int(enemy.GetDef()), SPD: int(enemy.GetSpd()),
		}
		switch enemy.GetKind() {
		case gameenginev1.EnemyKind_ENEMY_KIND_STRAY_DOG:
			result.Enemy.Kind = domain.EnemyStrayDog
		case gameenginev1.EnemyKind_ENEMY_KIND_WILD_LYNX:
			result.Enemy.Kind = domain.EnemyWildLynx
		case gameenginev1.EnemyKind_ENEMY_KIND_SEWER_RAT:
			result.Enemy.Kind = domain.EnemySewerRat
		case gameenginev1.EnemyKind_ENEMY_KIND_RAT_ACCOUNTANT:
			result.Enemy.Kind = domain.EnemyRatAccountant
		case gameenginev1.EnemyKind_ENEMY_KIND_COURIER_DOG:
			result.Enemy.Kind = domain.EnemyCourierDog
		case gameenginev1.EnemyKind_ENEMY_KIND_MOON_LYNX:
			result.Enemy.Kind = domain.EnemyMoonLynx
		}
	}
	if stats := value.GetStatsGained(); stats != nil {
		result.StatsGained = domain.StatDelta{
			HP: int(stats.GetHp()), ATK: int(stats.GetAtk()), DEF: int(stats.GetDef()), SPD: int(stats.GetSpd()),
		}
	}
	if len(value.GetTurns()) > 0 {
		result.Turns = make([]domain.BattleTurn, 0, len(value.GetTurns()))
	}
	for _, turn := range value.GetTurns() {
		actor := domain.BattleActorCat
		if turn.GetActor() == gameenginev1.BattleActor_BATTLE_ACTOR_ENEMY {
			actor = domain.BattleActorEnemy
		}
		result.Turns = append(result.Turns, domain.BattleTurn{
			Round: int(turn.GetRound()), Actor: actor, Damage: int(turn.GetDamage()),
			Crit: turn.GetCrit(), DefenderHPAfter: int(turn.GetDefenderHpAfter()),
		})
	}
	return result
}

func statDeltaToProto(stats domain.StatDelta) *gameenginev1.StatDelta {
	return &gameenginev1.StatDelta{Hp: int32(stats.HP), Atk: int32(stats.ATK), Def: int32(stats.DEF), Spd: int32(stats.SPD)}
}

func expeditionLocationToProto(location domain.ExpeditionLocation) gameenginev1.ExpeditionLocation {
	switch location {
	case domain.ExpeditionAlley:
		return gameenginev1.ExpeditionLocation_EXPEDITION_LOCATION_ALLEY
	case domain.ExpeditionRooftop:
		return gameenginev1.ExpeditionLocation_EXPEDITION_LOCATION_ROOFTOP
	case domain.ExpeditionPark:
		return gameenginev1.ExpeditionLocation_EXPEDITION_LOCATION_PARK
	default:
		return gameenginev1.ExpeditionLocation_EXPEDITION_LOCATION_UNSPECIFIED
	}
}

func expeditionDifficultyToProto(difficulty domain.ExpeditionDifficulty) gameenginev1.ExpeditionDifficulty {
	switch difficulty {
	case domain.ExpeditionEasy:
		return gameenginev1.ExpeditionDifficulty_EXPEDITION_DIFFICULTY_EASY
	case domain.ExpeditionNormal:
		return gameenginev1.ExpeditionDifficulty_EXPEDITION_DIFFICULTY_NORMAL
	case domain.ExpeditionHard:
		return gameenginev1.ExpeditionDifficulty_EXPEDITION_DIFFICULTY_HARD
	default:
		return gameenginev1.ExpeditionDifficulty_EXPEDITION_DIFFICULTY_UNSPECIFIED
	}
}

func felineToProto(s domain.FelineStats) *gameenginev1.FelineStats {
	return &gameenginev1.FelineStats{ClawsTenthMm: int32(s.ClawsTenthMM), WeightGrams: int32(s.WeightGrams), TailMm: int32(s.TailMM), WhiskerSpanMm: int32(s.WhiskerSpanMM)}
}
func felineFromProto(s *gameenginev1.FelineStats) domain.FelineStats {
	return domain.FelineStats{ClawsTenthMM: int(s.GetClawsTenthMm()), WeightGrams: int(s.GetWeightGrams()), TailMM: int(s.GetTailMm()), WhiskerSpanMM: int(s.GetWhiskerSpanMm())}
}
func (e *GRPCEngine) Progress(ctx context.Context, in ProgressInput) (domain.Cat, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultRemoteTimeout)
	defer cancel()
	out, err := e.client.Progress(ctx, &gameenginev1.ProgressRequest{RulesVersion: CurrentRulesVersion, Cat: catToProto(in.Cat), XpGain: in.XPGain, Seed: in.Seed, Source: in.Source, OutcomeTier: in.OutcomeTier, SecretFound: in.SecretFound, EnergySpent: int32(in.EnergySpent), TrainingLootRoll: int32(in.TrainingLootRoll)})
	if err != nil {
		return domain.Cat{}, err
	}
	if out.GetCat() == nil {
		return domain.Cat{}, errors.New("incomplete progression response")
	}
	cat := catFromProto(out.GetCat())
	cat.LootItemID = out.GetLootItemId()
	cat.ProgressionFacts = out.GetFacts()
	return cat, nil
}

// Check verifies that the remote engine accepts the current rules. The engine
// is stateless: this synthetic cat is never written to the database.
func (e *GRPCEngine) Check(ctx context.Context) error {
	_, err := e.Progress(ctx, ProgressInput{Cat: domain.Cat{ID: -1, Level: 1, Breed: domain.BreedBritish}, Source: "arena"})
	return err
}
