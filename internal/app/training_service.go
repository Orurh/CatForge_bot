package app

import (
	"context"
	"fmt"
	"strconv"

	"catforge/internal/ai"
	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

type TrainingService struct {
	cats          CatRepository
	personalities PersonalityRepository
	quota         AIQuotaRepository
	yards         YardRepository
	engine        gameengine.Engine
	voice         ai.Generator
	clock         Clock
	rng           RNG
	events        GameEventSink
}

func NewTrainingService(cats CatRepository, users UserRepository, personalities PersonalityRepository, quota AIQuotaRepository, yards YardRepository, engine gameengine.Engine, voice ai.Generator, clock Clock, rng RNG, events GameEventSink) *TrainingService {
	return &TrainingService{
		cats:          cats,
		personalities: personalities,
		quota:         quota,
		yards:         yards,
		engine:        engine,
		voice:         voice,
		clock:         clock,
		rng:           rng,
		events:        events,
	}
}

func (s *TrainingService) Train(
	ctx context.Context,
	userID int64,
	telegramID int64,
	sourceChatID int64,
	sourceChatType string,
) (*domain.Cat, domain.TrainResult, ai.Generation, error) {

	now := s.clock.Now()
	random := gameengine.RollTraining(s.rng)

	var cat *domain.Cat
	var res domain.TrainResult
	for attempt := 0; attempt < 3; attempt++ {
		current, err := s.cats.GetByUserID(ctx, userID)
		if err != nil {
			return nil, domain.TrainResult{}, ai.Generation{}, err
		}
		out, err := s.engine.Train(ctx, gameengine.TrainingInput{
			RulesVersion:   gameengine.CurrentRulesVersion,
			ContentVersion: gameengine.CurrentContentVersion,
			Cat:            *current,
			Now:            now,
			Random:         random,
		})
		if err != nil {
			return nil, domain.TrainResult{}, ai.Generation{}, err
		}
		saved, err := s.cats.SaveProgress(ctx, userID, current.StateVersion, out.Cat)
		if err != nil {
			return nil, domain.TrainResult{}, ai.Generation{}, err
		}
		if !saved {
			continue
		}
		out.Cat.StateVersion = current.StateVersion + 1
		cat = &out.Cat
		res = out.Result
		break
	}
	if cat == nil {
		return nil, domain.TrainResult{}, ai.Generation{}, domain.ErrConcurrentUpdate
	}
	targetChatID := int64(0)
	if sourceChatType != "" && sourceChatType != "private" {
		targetChatID = sourceChatID
	}
	generation, rivalName, rivalry := s.generateNarrative(ctx, userID, sourceChatID, targetChatID, cat, res)

	if s.events != nil {
		baseKey := fmt.Sprintf("user:%d:cat:%d:state:%d", userID, cat.ID, cat.StateVersion)
		energyNow := domain.RegenEnergy(cat.Energy, cat.EnergyUpdatedAt, now)
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: baseKey + ":trained", Kind: GameEventCatTrained,
			UserID: userID, CatID: cat.ID, OccurredAt: now,
			RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion,
			PayloadVersion: GameEventPayloadVersion, Notable: res.Crit,
			Payload: CatTrainedPayload{
				TargetChatID: targetChatID, TelegramID: telegramID, CatName: cat.Name,
				Breed: cat.Breed, Trait: cat.Trait, Energy: energyNow, Level: cat.Level, Result: res,
				Narrative: generation.Text, RivalName: rivalName, Rivalry: rivalry,
				Provider: generation.Provider, Model: generation.Model, Fallback: generation.Fallback,
			},
		})
		if res.LeveledUp > 0 {
			_ = s.events.Publish(ctx, GameEvent{
				DedupeKey: baseKey + ":leveled-up", Kind: GameEventCatLeveledUp,
				UserID: userID, CatID: cat.ID, OccurredAt: now,
				RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion,
				PayloadVersion: GameEventPayloadVersion, Notable: true,
				Payload: CatLeveledUpPayload{
					TargetChatID: targetChatID, CatName: cat.Name, Level: cat.Level,
					LevelsGained: res.LeveledUp, StatsGained: res.StatsGained,
				},
			})
		}
	}

	return cat, res, generation, nil
}

func (s *TrainingService) generateNarrative(ctx context.Context, userID, sourceChatID, targetChatID int64, cat *domain.Cat, result domain.TrainResult) (ai.Generation, string, int) {
	if s.voice == nil || cat == nil || result.Outcome != domain.TrainingOK {
		return ai.Generation{}, "", 0
	}
	allowed := true
	if s.quota != nil {
		var err error
		allowed, err = s.quota.AllowAIRequest(
			ctx, userID, sourceChatID, s.clock.Now(), requestedAIUserHourlyLimit, requestedAIChatHourlyLimit,
		)
		if err != nil || !allowed {
			return ai.Generation{}, "", 0
		}
	}
	personality := &domain.CatPersonality{
		CatID: cat.ID, Trait: cat.Trait, SpeechStyle: domain.DefaultSpeechStyle(cat.Trait), HumorMode: domain.HumorNormal,
	}
	if s.personalities != nil {
		if stored, err := s.personalities.GetByCatID(ctx, cat.ID); err == nil && stored != nil {
			personality = stored
		}
	}
	rivalName, rivalry, yardID := s.strongestRival(ctx, cat.ID, sourceChatID, targetChatID)
	request := ai.GenerationRequest{
		YardID: yardID,
		Cat: ai.CatContext{
			ID: cat.ID, Name: cat.Name, Breed: cat.Breed, Trait: personality.Trait, SpeechStyle: personality.SpeechStyle,
		},
		HumorMode: personality.HumorMode,
		EventFacts: []string{
			"encounter=" + strconv.Quote(string(result.Encounter)),
			"energy_cost=" + strconv.Itoa(result.EnergyCost),
			"xp_gain=" + strconv.FormatInt(result.XPGain, 10),
			"coins_gain=" + strconv.FormatInt(result.CoinsGain, 10),
			"critical=" + strconv.FormatBool(result.Crit),
			"level=" + strconv.Itoa(cat.Level),
			"levels_gained=" + strconv.Itoa(result.LeveledUp),
			"rival_name=" + strconv.Quote(rivalName),
			"rivalry=" + strconv.Itoa(rivalry),
		},
		Training: &ai.TrainingContext{
			Encounter: string(result.Encounter), EnergyCost: result.EnergyCost, XPGain: result.XPGain, CoinsGain: result.CoinsGain,
			Critical: result.Crit, Level: cat.Level, LevelsGained: result.LeveledUp,
			RivalName: rivalName, Rivalry: rivalry,
		},
	}
	generation, err := s.voice.GenerateTrainingNarrative(ctx, request)
	if err != nil {
		return ai.Generation{}, rivalName, rivalry
	}
	return generation, rivalName, rivalry
}

func (s *TrainingService) strongestRival(ctx context.Context, catID, sourceChatID, targetChatID int64) (string, int, int64) {
	if s.yards == nil {
		return "", 0, 0
	}
	chatID := sourceChatID
	if targetChatID != 0 {
		chatID = targetChatID
	}
	yard, err := s.yards.GetByTelegramChatID(ctx, chatID)
	if err != nil || yard == nil {
		return "", 0, 0
	}
	relationships, err := s.yards.ListRelationships(ctx, yard.ID)
	if err != nil {
		return "", 0, yard.ID
	}
	var bestName string
	bestRivalry := 0
	bestCatID := int64(0)
	for _, relationship := range relationships {
		var otherID int64
		var otherName string
		switch catID {
		case relationship.CatAID:
			otherID, otherName = relationship.CatBID, relationship.CatBName
		case relationship.CatBID:
			otherID, otherName = relationship.CatAID, relationship.CatAName
		default:
			continue
		}
		if relationship.Rivalry > bestRivalry ||
			(relationship.Rivalry == bestRivalry && relationship.Rivalry > 0 && (bestCatID == 0 || otherID < bestCatID)) {
			bestName, bestRivalry, bestCatID = otherName, relationship.Rivalry, otherID
		}
	}
	return bestName, bestRivalry, yard.ID
}
