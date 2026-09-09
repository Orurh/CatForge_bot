package app

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

const (
	FightQueueTTL       = 12 * time.Hour
	FightRevengeTTL     = 15 * time.Minute
	FightPairCooldown   = 30 * time.Minute
	FightDailyWindow    = 24 * time.Hour
	FightDailyLimit     = 1
	FightPairFightLimit = 3
)

type FightOutcome struct {
	CatAFacts    []string
	CatBFacts    []string
	Status       domain.FightQueueStatus
	Yard         *domain.Yard
	Cat          *domain.Cat
	Opponent     *domain.Cat
	Result       domain.FightResult
	FightID      int64
	Friendship   int
	Rivalry      int
	Respect      int
	Seed         uint64
	Kind         domain.FightKind
	ParentID     int64
	Stats        domain.FightStats
	WinnerXPGain int64
	LoserXPGain  int64
	Banter       *FightBanter
}

type FightBanter struct {
	FirstCat   *domain.Cat
	SecondCat  *domain.Cat
	FirstLine  string
	SecondLine string
	Generation ai.Generation
}

type FightService struct {
	cats          CatRepository
	personalities PersonalityRepository
	yards         YardRepository
	fights        FightRepository
	engine        gameengine.Engine
	voice         ai.Generator
	clock         Clock
	rng           RNG
	events        GameEventSink
}

func NewFightService(cats CatRepository, personalities PersonalityRepository, yards YardRepository, fights FightRepository, engine gameengine.Engine, voice ai.Generator, clock Clock, rng RNG, events GameEventSink) *FightService {
	return &FightService{cats: cats, personalities: personalities, yards: yards, fights: fights, engine: engine, voice: voice, clock: clock, rng: rng, events: events}
}

// Toggle enters or leaves the caller's cat in the yard arena. Matching and
// removing a waiting opponent is performed atomically by FightRepository.
func (s *FightService) Toggle(ctx context.Context, telegramChatID, userID int64) (FightOutcome, error) {
	yard, err := s.yards.GetByTelegramChatID(ctx, telegramChatID)
	if err != nil {
		return FightOutcome{}, err
	}
	if !yard.FightsEnabled {
		return FightOutcome{}, domain.ErrFightsDisabled
	}
	cat, err := s.cats.GetByUserID(ctx, userID)
	if err != nil {
		return FightOutcome{}, err
	}

	if cat.Level < 3 {
		return FightOutcome{}, domain.ErrFeatureLocked
	}
	now := s.clock.Now()
	toggle, err := s.fights.ToggleQueue(ctx, yard.ID, userID, cat.ID, now, fightQueueExpiresAt(now), fightLimits(now))
	if err != nil {
		return FightOutcome{}, err
	}
	outcome := FightOutcome{Status: toggle.Status, Yard: yard, Cat: cat, Kind: domain.FightKindRegular}
	if toggle.Status != domain.FightQueueMatched {
		return outcome, nil
	}

	opponent, err := s.cats.GetByUserID(ctx, toggle.Opponent.UserID)
	if err != nil {
		return FightOutcome{}, fmt.Errorf("load arena opponent: %w", err)
	}
	return s.resolveFight(ctx, telegramChatID, userID, yard, opponent, cat, domain.FightKindRegular, 0)
}

func (s *FightService) Revenge(ctx context.Context, telegramChatID, sourceFightID, loserUserID int64) (FightOutcome, error) {
	yard, err := s.yards.GetByTelegramChatID(ctx, telegramChatID)
	if err != nil {
		return FightOutcome{}, err
	}
	if !yard.FightsEnabled {
		return FightOutcome{}, domain.ErrFightsDisabled
	}
	now := s.clock.Now()
	revenge, err := s.fights.GetRevenge(ctx, telegramChatID, sourceFightID, loserUserID, now, fightLimits(now))
	if err != nil {
		return FightOutcome{}, err
	}
	if revenge.YardID != yard.ID || revenge.LoserUserID != loserUserID {
		return FightOutcome{}, domain.ErrFightRevengeExpired
	}
	loser, err := s.cats.GetByUserID(ctx, revenge.LoserUserID)
	if err != nil {
		return FightOutcome{}, err
	}
	winner, err := s.cats.GetByUserID(ctx, revenge.WinnerUserID)
	if err != nil {
		return FightOutcome{}, err
	}
	if loser.ID != revenge.LoserCatID || winner.ID != revenge.WinnerCatID {
		return FightOutcome{}, domain.ErrFightRevengeExpired
	}
	return s.resolveFight(ctx, telegramChatID, loserUserID, yard, winner, loser, domain.FightKindRevenge, sourceFightID)
}

// Queue expires at the earlier of 12 hours and the next game day boundary (Moscow).
// The fight allowance remains a rolling 24-hour limit across all yards.
func fightQueueExpiresAt(now time.Time) time.Time {
	until := now.Add(FightQueueTTL)
	local := GameTime(now)
	midnight := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, local.Location())
	if midnight.Before(until) {
		return midnight
	}
	return until
}

func fightLimits(now time.Time) domain.FightLimits {
	return domain.FightLimits{
		DailySince: now.Add(-FightDailyWindow), DailyLimit: FightDailyLimit,
		PairSince: now.Add(-FightPairCooldown), PairLimit: FightPairFightLimit,
		RevengeSince: now.Add(-FightRevengeTTL),
	}
}

func (s *FightService) resolveFight(ctx context.Context, telegramChatID, initiatingUserID int64, yard *domain.Yard, catA, catB *domain.Cat, kind domain.FightKind, parentFightID int64) (FightOutcome, error) {
	if catA.Level < 3 || catB.Level < 3 {
		return FightOutcome{}, domain.ErrFeatureLocked
	}
	now := s.clock.Now()
	seed := s.fightSeed()
	combatA, combatB := *catA, *catB
	if equipment, ok := s.cats.(interface {
		EquippedStats(context.Context, int64, int) (domain.FelineStats, error)
	}); ok {
		for _, cat := range []*domain.Cat{&combatA, &combatB} {
			bonus, err := equipment.EquippedStats(ctx, cat.UserID, cat.Level)
			if err != nil {
				return FightOutcome{}, err
			}
			cat.Feline = cat.PhysicalStats()
			cat.Feline.Add(bonus)
		}
	}
	result, err := s.engine.Fight(ctx, gameengine.FightInput{
		RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion,
		Seed: seed, CatA: combatA, CatB: combatB,
	})
	if err != nil {
		return FightOutcome{}, err
	}
	if err := validateFightResult(result, catA.ID, catB.ID); err != nil {
		return FightOutcome{}, err
	}

	rivalryDelta := 1
	if winnerHP(result, catA.ID, catB.ID) <= 5 {
		rivalryDelta++
	}
	catAXPGain := arenaXPGain(catA.Level, result.WinnerCatID == catA.ID)
	catBXPGain := arenaXPGain(catB.Level, result.WinnerCatID == catB.ID)
	record := domain.FightRecord{
		CatAVersion: catA.StateVersion, CatBVersion: catB.StateVersion,
		YardID: yard.ID, CatAID: catA.ID, CatBID: catB.ID,
		WinnerCatID: result.WinnerCatID, LoserCatID: result.LoserCatID,
		Seed: int64(seed), Rounds: result.Rounds, FinalHPA: result.FinalHPA, FinalHPB: result.FinalHPB,
		RivalryDelta: rivalryDelta, CatAXPGain: catAXPGain, CatBXPGain: catBXPGain,
		RulesVersion:   gameengine.CurrentRulesVersion,
		ContentVersion: gameengine.CurrentContentVersion, CreatedAt: now,
		Kind: kind, ParentFightID: parentFightID,
	}
	saved, err := s.fights.SaveFight(ctx, record, result, catA.Name, catB.Name, fightLimits(now))
	if err != nil {
		return FightOutcome{}, err
	}
	outcome := FightOutcome{
		Status: domain.FightQueueMatched, Yard: yard, Cat: catB, Opponent: catA,
		CatAFacts: saved.CatAFacts, CatBFacts: saved.CatBFacts,
		Result: result, FightID: saved.FightID, Friendship: saved.Friendship, Rivalry: saved.Rivalry, Respect: saved.Respect, Seed: seed,
		Kind: kind, ParentID: parentFightID, Stats: saved.Stats,
	}
	if result.WinnerCatID == catA.ID {
		outcome.WinnerXPGain, outcome.LoserXPGain = catAXPGain, catBXPGain
	} else {
		outcome.WinnerXPGain, outcome.LoserXPGain = catBXPGain, catAXPGain
	}
	outcome.Banter = s.generateFightBanter(ctx, yard, catA, catB, result, saved, kind, now)

	if s.events != nil {
		notability := fightNotability(catA, catB, result, saved, kind)
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: fmt.Sprintf("yard:%d:fight:%d:finished", yard.ID, saved.FightID), Kind: GameEventFightFinished,
			UserID: initiatingUserID, CatID: result.WinnerCatID, YardID: yard.ID, OccurredAt: now,
			RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion,
			PayloadVersion: GameEventPayloadVersion, Notable: notability.Notable,
			Payload: FightFinishedPayload{
				FightID: saved.FightID, TelegramChatID: telegramChatID, CatAName: catA.Name, CatBName: catB.Name,
				Result: result, Friendship: saved.Friendship, Rivalry: saved.Rivalry, Respect: saved.Respect, Seed: seed, Kind: kind,
				ParentFightID: parentFightID, Stats: saved.Stats,
				WinnerXPGain: outcome.WinnerXPGain, LoserXPGain: outcome.LoserXPGain,
				Banter: fightBanterPayload(outcome.Banter),
			},
		})
	}
	return outcome, nil
}

func arenaXPGain(level int, won bool) int64 {
	if level < 1 {
		level = 1
	}
	// A level needs level*100 XP: participation is 3%, victory adds 2%.
	percent := 3
	if won {
		percent += 2
	}
	return int64(level * percent)
}

type fightNotableFacts struct {
	Notable       bool
	Close         bool
	Upset         bool
	FirstFight    bool
	DecidingFight bool
	Milestone     bool
}

func fightNotability(catA, catB *domain.Cat, result domain.FightResult, saved domain.FightSaveResult, kind domain.FightKind) fightNotableFacts {
	winner, loser := catA, catB
	if result.WinnerCatID == catB.ID {
		winner, loser = catB, catA
	}
	pairFights := saved.Stats.PairCatAWins + saved.Stats.PairCatBWins
	facts := fightNotableFacts{
		Close:         winnerHP(result, catA.ID, catB.ID) <= 5,
		Upset:         winner.Level < loser.Level,
		FirstFight:    pairFights == 1,
		DecidingFight: pairFights == FightPairFightLimit,
	}
	previousRivalry := saved.Rivalry - 1
	if facts.Close {
		previousRivalry--
	}
	facts.Milestone = previousRivalry/5 < saved.Rivalry/5
	facts.Notable = facts.Close || facts.Upset || facts.FirstFight || facts.DecidingFight || facts.Milestone || kind == domain.FightKindRevenge
	return facts
}

func (s *FightService) generateFightBanter(ctx context.Context, yard *domain.Yard, catA, catB *domain.Cat, result domain.FightResult, saved domain.FightSaveResult, kind domain.FightKind, now time.Time) *FightBanter {
	if s.voice == nil || s.personalities == nil || yard == nil || !yard.AutoMessagesEnabled || !yard.CatToCatBanter || yard.MaxAutoMessagesDay <= 0 || yard.QuietUntil.After(now) {
		return nil
	}
	facts := fightNotability(catA, catB, result, saved, kind)
	if !facts.Notable {
		return nil
	}
	personalityA, err := s.personalities.GetByCatID(ctx, catA.ID)
	if err != nil || personalityA == nil || !personalityA.AutoSpeakEnabled {
		return nil
	}
	personalityB, err := s.personalities.GetByCatID(ctx, catB.ID)
	if err != nil || personalityB == nil || !personalityB.AutoSpeakEnabled {
		return nil
	}
	allowed, err := s.yards.ClaimAutoMessageSlot(ctx, yard.ID, now, yard.MaxAutoMessagesDay, domain.AutoMessageBanter)
	if err != nil || !allowed {
		return nil
	}

	winner, loser := catA, catB
	winnerPersonality, loserPersonality := personalityA, personalityB
	winnerPairWins, loserPairWins := saved.Stats.PairCatAWins, saved.Stats.PairCatBWins
	if result.WinnerCatID == catB.ID {
		winner, loser = catB, catA
		winnerPersonality, loserPersonality = personalityB, personalityA
		winnerPairWins, loserPairWins = saved.Stats.PairCatBWins, saved.Stats.PairCatAWins
	}
	humor := domain.HumorNormal
	if yard.HumorMode == domain.HumorBold && winnerPersonality.HumorMode == domain.HumorBold && loserPersonality.HumorMode == domain.HumorBold {
		humor = domain.HumorBold
	}
	context := &ai.ArenaBanterContext{
		FightKind: string(kind), WinnerName: winner.Name, LoserName: loser.Name,
		WinnerHP: winnerHP(result, catA.ID, catB.ID), Rounds: result.Rounds,
		Friendship: saved.Friendship, Rivalry: saved.Rivalry, Respect: saved.Respect,
		LoserPairWins: loserPairWins, WinnerPairWins: winnerPairWins,
		WinnerStreak: saved.Stats.PairWinnerStreak,
		Close:        facts.Close, Upset: facts.Upset, FirstFight: facts.FirstFight, DecidingFight: facts.DecidingFight,
	}
	generation, err := s.voice.GenerateArenaBanter(ctx, ai.GenerationRequest{
		YardID: yard.ID, HumorMode: humor,
		Cat:         ai.CatContext{ID: loser.ID, Name: loser.Name, Breed: loser.Breed, Trait: loserPersonality.Trait, SpeechStyle: loserPersonality.SpeechStyle},
		OtherCat:    ai.CatContext{ID: winner.ID, Name: winner.Name, Breed: winner.Breed, Trait: winnerPersonality.Trait, SpeechStyle: winnerPersonality.SpeechStyle},
		ArenaBanter: context,
		Relationships: []ai.RelationshipContext{{
			CatAName: loser.Name, CatBName: winner.Name,
			Friendship: saved.Friendship, Rivalry: saved.Rivalry, Respect: saved.Respect,
		}},
		EventFacts: []string{
			"fight_kind=" + strconv.Quote(string(kind)), "winner=" + strconv.Quote(winner.Name), "loser=" + strconv.Quote(loser.Name),
			"winner_hp=" + strconv.Itoa(context.WinnerHP), "rounds=" + strconv.Itoa(result.Rounds),
			"friendship=" + strconv.Itoa(saved.Friendship), "rivalry=" + strconv.Itoa(saved.Rivalry), "respect=" + strconv.Itoa(saved.Respect),
			"pair_score_loser=" + strconv.Itoa(loserPairWins), "pair_score_winner=" + strconv.Itoa(winnerPairWins),
			"winner_streak=" + strconv.Itoa(saved.Stats.PairWinnerStreak),
			"close=" + strconv.FormatBool(facts.Close), "upset=" + strconv.FormatBool(facts.Upset),
		},
	})
	if err != nil {
		return nil
	}
	lines, ok := ai.ParseArenaBanterLines(generation.Text)
	if !ok {
		return nil
	}
	return &FightBanter{FirstCat: loser, SecondCat: winner, FirstLine: lines[0], SecondLine: lines[1], Generation: generation}
}

func fightBanterPayload(banter *FightBanter) *FightBanterPayload {
	if banter == nil || banter.FirstCat == nil || banter.SecondCat == nil {
		return nil
	}
	return &FightBanterPayload{
		FirstCatID: banter.FirstCat.ID, FirstCatName: banter.FirstCat.Name, FirstLine: banter.FirstLine,
		SecondCatID: banter.SecondCat.ID, SecondCatName: banter.SecondCat.Name, SecondLine: banter.SecondLine,
		Provider: banter.Generation.Provider, Model: banter.Generation.Model, Fallback: banter.Generation.Fallback,
	}
}

func (s *FightService) fightSeed() uint64 {
	return uint64(s.rng.Intn(1<<30))<<30 | uint64(s.rng.Intn(1<<30))
}

func winnerHP(result domain.FightResult, catAID, catBID int64) int {
	if result.WinnerCatID == catAID {
		return result.FinalHPA
	}
	if result.WinnerCatID == catBID {
		return result.FinalHPB
	}
	return 0
}

func validateFightResult(result domain.FightResult, catAID, catBID int64) error {
	validPair := (result.WinnerCatID == catAID && result.LoserCatID == catBID) ||
		(result.WinnerCatID == catBID && result.LoserCatID == catAID)
	if !validPair || result.Rounds < 1 || result.FinalHPA < 0 || result.FinalHPB < 0 || len(result.Turns) == 0 {
		return fmt.Errorf("game engine returned an invalid fight result")
	}
	return nil
}
