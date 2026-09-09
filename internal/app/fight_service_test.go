package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

type fightCats map[int64]*domain.Cat

func (f fightCats) GetByUserID(_ context.Context, userID int64) (*domain.Cat, error) {
	cat, ok := f[userID]
	if !ok {
		return nil, domain.ErrNoCat
	}
	return cat, nil
}
func (fightCats) Create(context.Context, int64, string, domain.Breed, domain.Trait, int, int, int, int) (*domain.Cat, error) {
	return nil, nil
}
func (fightCats) DeleteByUserID(context.Context, int64) error { return nil }
func (fightCats) SaveProgress(context.Context, int64, int64, domain.Cat) (bool, error) {
	return true, nil
}
func (fightCats) SetName(context.Context, int64, string) (*domain.Cat, error) { return nil, nil }

type fightPersonalities map[int64]*domain.CatPersonality

type recordingFightProvider struct {
	prompt ai.Prompt
}

func (*recordingFightProvider) Name() string { return "recording" }
func (p *recordingFightProvider) Generate(_ context.Context, prompt ai.Prompt) (ai.ProviderResult, error) {
	p.prompt = prompt
	return ai.ProviderResult{Text: "Я ещё вернусь.\nВозьми карту, а то опять не найдёшь арену.", Model: "test"}, nil
}

func (f fightPersonalities) GetByCatID(_ context.Context, catID int64) (*domain.CatPersonality, error) {
	personality, ok := f[catID]
	if !ok {
		return nil, domain.ErrNoCat
	}
	return personality, nil
}
func (fightPersonalities) SetTrait(context.Context, int64, domain.Trait, string) (*domain.CatPersonality, error) {
	return nil, nil
}
func (fightPersonalities) SetHumorMode(context.Context, int64, domain.HumorMode) error {
	return nil
}
func (fightPersonalities) SetAutoSpeak(context.Context, int64, bool) error { return nil }

type fakeFightRepo struct {
	toggle      domain.FightQueueToggle
	toggleNow   time.Time
	toggleUntil time.Time
	saved       bool
	record      domain.FightRecord
	result      domain.FightResult
	catAName    string
	catBName    string
	revenge     domain.FightRevenge
	limits      domain.FightLimits
	saveLimits  domain.FightLimits
	saveResult  domain.FightSaveResult
}

func (f *fakeFightRepo) ToggleQueue(_ context.Context, _, _, _ int64, now, expiresAt time.Time, limits domain.FightLimits) (domain.FightQueueToggle, error) {
	f.toggleNow, f.toggleUntil, f.limits = now, expiresAt, limits
	return f.toggle, nil
}
func (f *fakeFightRepo) GetRevenge(_ context.Context, _, _, _ int64, _ time.Time, limits domain.FightLimits) (domain.FightRevenge, error) {
	f.limits = limits
	return f.revenge, nil
}
func (f *fakeFightRepo) SaveFight(_ context.Context, record domain.FightRecord, result domain.FightResult, catAName, catBName string, limits domain.FightLimits) (domain.FightSaveResult, error) {
	f.saved, f.record, f.result, f.catAName, f.catBName = true, record, result, catAName, catBName
	f.saveLimits = limits
	if f.saveResult.FightID == 0 {
		f.saveResult = domain.FightSaveResult{FightID: 55, Rivalry: 9}
	}
	return f.saveResult, nil
}
func (f *fakeFightRepo) CatStats(context.Context, int64) (domain.ArenaStats, error) {
	return domain.ArenaStats{}, nil
}

type fightEngine struct {
	input  gameengine.FightInput
	result domain.FightResult
	calls  int
}

func (e *fightEngine) Train(_ context.Context, input gameengine.TrainingInput) (gameengine.TrainingOutput, error) {
	return gameengine.TrainingOutput{Cat: input.Cat}, nil
}
func (e *fightEngine) Expedition(_ context.Context, input gameengine.ExpeditionInput) (gameengine.ExpeditionOutput, error) {
	return gameengine.ExpeditionOutput{Cat: input.Cat}, nil
}
func (e *fightEngine) ResolveYardEvent(context.Context, gameengine.YardEventInput) (domain.YardEventResult, error) {
	return domain.YardEventResult{}, nil
}
func (e *fightEngine) Fight(_ context.Context, input gameengine.FightInput) (domain.FightResult, error) {
	e.calls++
	e.input = input
	return e.result, nil
}

type sequenceRNG struct {
	values []int
	index  int
}

func (s *sequenceRNG) Intn(int) int {
	value := s.values[s.index]
	s.index++
	return value
}

func TestFightServiceWaitsAndCancelsWithoutRunningEngine(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	cat := &domain.Cat{ID: 10, UserID: 1, Name: "Барсик", Level: 3}
	for _, status := range []domain.FightQueueStatus{domain.FightQueueWaiting, domain.FightQueueCanceled} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()
			repo := &fakeFightRepo{toggle: domain.FightQueueToggle{Status: status}}
			engine := &fightEngine{}
			svc := NewFightService(fightCats{1: cat}, nil, stubYards{yard: &domain.Yard{ID: 7, FightsEnabled: true}}, repo, engine, nil, fakeClock{t: now}, zeroRNG{}, nil)

			outcome, err := svc.Toggle(context.Background(), -100, 1)
			if err != nil {
				t.Fatalf("Toggle() error = %v", err)
			}
			if outcome.Status != status || outcome.Cat != cat || engine.calls != 0 || repo.saved {
				t.Fatalf("unexpected outcome: %+v, engine calls=%d saved=%v", outcome, engine.calls, repo.saved)
			}
			if !repo.toggleNow.Equal(now) || !repo.toggleUntil.Equal(now.Add(FightQueueTTL)) {
				t.Fatalf("queue interval = %v..%v", repo.toggleNow, repo.toggleUntil)
			}
			if repo.limits.DailyLimit != FightDailyLimit || repo.limits.PairLimit != FightPairFightLimit {
				t.Fatalf("fight limits = %+v", repo.limits)
			}
		})
	}
}

func TestFightDailyLimitIsOnePerRollingDay(t *testing.T) {
	t.Parallel()
	if FightDailyLimit != 1 {
		t.Fatalf("FightDailyLimit = %d, want 1", FightDailyLimit)
	}
	now := time.Unix(10_000, 0)
	limits := fightLimits(now)
	if limits.DailyLimit != 1 || !limits.DailySince.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("fightLimits() = %+v", limits)
	}
}

func TestFightServiceHonorsYardFightSwitch(t *testing.T) {
	t.Parallel()
	repo := &fakeFightRepo{}
	engine := &fightEngine{}
	svc := NewFightService(fightCats{}, nil, stubYards{yard: &domain.Yard{ID: 7, FightsEnabled: false}}, repo, engine, nil, fakeClock{}, zeroRNG{}, nil)

	_, err := svc.Toggle(context.Background(), -100, 1)
	if err != domain.ErrFightsDisabled {
		t.Fatalf("Toggle() error = %v, want ErrFightsDisabled", err)
	}
	if !repo.toggleNow.IsZero() || engine.calls != 0 {
		t.Fatalf("disabled fight reached repository or engine: repo=%+v engine_calls=%d", repo, engine.calls)
	}
}

func TestFightServiceRunsLoserOwnedRevenge(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	winner := &domain.Cat{ID: 10, UserID: 1, Name: "Барсик", Level: 3, HPBase: 50, ATKBase: 20, DEFBase: 15, SPDBase: 18}
	loser := &domain.Cat{ID: 20, UserID: 2, Name: "Батон", Level: 3, HPBase: 40, ATKBase: 24, DEFBase: 12, SPDBase: 22}
	result := domain.FightResult{
		WinnerCatID: loser.ID, LoserCatID: winner.ID, Rounds: 3, FinalHPA: 0, FinalHPB: 7,
		Turns: []domain.FightTurn{{Round: 3, AttackerCatID: loser.ID, DefenderCatID: winner.ID, Damage: 8}},
	}
	repo := &fakeFightRepo{
		revenge: domain.FightRevenge{
			SourceFightID: 44, YardID: 7, LoserUserID: loser.UserID, LoserCatID: loser.ID,
			WinnerUserID: winner.UserID, WinnerCatID: winner.ID,
		},
		saveResult: domain.FightSaveResult{FightID: 56, Rivalry: 10, Stats: domain.FightStats{PairCatAWins: 1, PairCatBWins: 1}},
	}
	engine := &fightEngine{result: result}
	svc := NewFightService(
		fightCats{winner.UserID: winner, loser.UserID: loser}, nil, stubYards{yard: &domain.Yard{ID: 7, FightsEnabled: true}},
		repo, engine, nil, fakeClock{t: now}, &sequenceRNG{values: []int{5, 6}}, nil,
	)

	outcome, err := svc.Revenge(context.Background(), -100, 44, loser.UserID)
	if err != nil {
		t.Fatalf("Revenge() error = %v", err)
	}
	if engine.input.CatA.ID != winner.ID || engine.input.CatB.ID != loser.ID {
		t.Fatalf("revenge engine input = %+v", engine.input)
	}
	if repo.record.Kind != domain.FightKindRevenge || repo.record.ParentFightID != 44 {
		t.Fatalf("revenge record = %+v", repo.record)
	}
	if outcome.Kind != domain.FightKindRevenge || outcome.ParentID != 44 || outcome.FightID != 56 || outcome.Cat != loser {
		t.Fatalf("revenge outcome = %+v", outcome)
	}
}

func TestFightServiceMatchesWaitingOpponentAndPersistsResult(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	waiter := &domain.Cat{ID: 10, UserID: 1, Name: "Барсик", Level: 3, HPBase: 50, ATKBase: 20, DEFBase: 15, SPDBase: 18}
	newcomer := &domain.Cat{ID: 20, UserID: 2, Name: "Батон", Level: 3, HPBase: 40, ATKBase: 24, DEFBase: 12, SPDBase: 22}
	result := domain.FightResult{
		WinnerCatID: 20, LoserCatID: 10, Rounds: 4, FinalHPA: 0, FinalHPB: 4,
		Turns: []domain.FightTurn{{Round: 4, AttackerCatID: 20, DefenderCatID: 10, Damage: 8}},
	}
	repo := &fakeFightRepo{toggle: domain.FightQueueToggle{
		Status: domain.FightQueueMatched, Opponent: domain.FightQueueEntry{UserID: 1, CatID: 10},
	}}
	engine := &fightEngine{result: result}
	events := &fakeEvents{}
	rng := &sequenceRNG{values: []int{3, 4}}
	svc := NewFightService(fightCats{1: waiter, 2: newcomer}, nil, stubYards{yard: &domain.Yard{ID: 7, FightsEnabled: true}}, repo, engine, nil, fakeClock{t: now}, rng, events)

	outcome, err := svc.Toggle(context.Background(), -100, 2)
	if err != nil {
		t.Fatalf("Toggle() error = %v", err)
	}
	wantSeed := uint64(3)<<30 | 4
	if engine.calls != 1 || engine.input.CatA.ID != waiter.ID || engine.input.CatB.ID != newcomer.ID || engine.input.Seed != wantSeed {
		t.Fatalf("unexpected engine input: %+v", engine.input)
	}
	if !repo.saved || repo.record.RivalryDelta != 2 || repo.catAName != waiter.Name || repo.catBName != newcomer.Name {
		t.Fatalf("fight was not persisted correctly: %+v", repo)
	}
	if repo.record.CatAXPGain != 9 || repo.record.CatBXPGain != 15 || outcome.WinnerXPGain != 15 || outcome.LoserXPGain != 9 {
		t.Fatalf("fight XP = record %d/%d outcome %d/%d", repo.record.CatAXPGain, repo.record.CatBXPGain, outcome.WinnerXPGain, outcome.LoserXPGain)
	}
	if repo.saveLimits.DailyLimit != 1 || !repo.saveLimits.DailySince.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("save limits = %+v", repo.saveLimits)
	}
	if outcome.FightID != 55 || outcome.Rivalry != 9 || outcome.Opponent != waiter || outcome.Seed != wantSeed {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
	if len(events.events) != 1 || events.events[0].Kind != GameEventFightFinished || events.events[0].YardID != 7 {
		t.Fatalf("unexpected events: %+v", events.events)
	}
}

func TestArenaXPGainUsesLevelProgressPercentages(t *testing.T) {
	t.Parallel()
	if got := arenaXPGain(10, false); got != 30 {
		t.Fatalf("participation XP = %d, want 30", got)
	}
	if got := arenaXPGain(10, true); got != 50 {
		t.Fatalf("winner XP = %d, want 50", got)
	}
}

func TestFightServiceGeneratesOneBanterForNotableFight(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	loser := &domain.Cat{ID: 10, UserID: 1, Name: "Барсик", Breed: domain.BreedBengal, Level: 3, HPBase: 50, ATKBase: 20, DEFBase: 15, SPDBase: 18}
	winner := &domain.Cat{ID: 20, UserID: 2, Name: "Батон", Breed: domain.BreedBritish, Level: 3, HPBase: 40, ATKBase: 24, DEFBase: 12, SPDBase: 22}
	result := domain.FightResult{
		WinnerCatID: winner.ID, LoserCatID: loser.ID, Rounds: 4, FinalHPA: 0, FinalHPB: 4,
		Turns: []domain.FightTurn{{Round: 4, AttackerCatID: winner.ID, DefenderCatID: loser.ID, Damage: 8, DefenderHPAfter: 0}},
	}
	repo := &fakeFightRepo{
		toggle: domain.FightQueueToggle{Status: domain.FightQueueMatched, Opponent: domain.FightQueueEntry{UserID: loser.UserID, CatID: loser.ID}},
		saveResult: domain.FightSaveResult{
			FightID: 55, Friendship: 2, Rivalry: 9, Respect: 4,
			Stats: domain.FightStats{PairCatBWins: 3, PairWinnerStreak: 3},
		},
	}
	personalities := fightPersonalities{
		loser.ID:  {CatID: loser.ID, Trait: domain.TraitBully, SpeechStyle: domain.DefaultSpeechStyle(domain.TraitBully), HumorMode: domain.HumorBold, AutoSpeakEnabled: true},
		winner.ID: {CatID: winner.ID, Trait: domain.TraitLazy, SpeechStyle: domain.DefaultSpeechStyle(domain.TraitLazy), HumorMode: domain.HumorBold, AutoSpeakEnabled: true},
	}
	claimed := []domain.AutoMessageKind{}
	yard := &domain.Yard{ID: 7, TelegramChatID: -100, HumorMode: domain.HumorBold, AutoMessagesEnabled: true, MaxAutoMessagesDay: 2, CatToCatBanter: true, FightsEnabled: true}
	provider := &recordingFightProvider{}
	voice := ai.NewGateway(provider, ai.NewFallbackProvider(), nil, time.Second)
	events := &fakeEvents{}
	svc := NewFightService(
		fightCats{loser.UserID: loser, winner.UserID: winner}, personalities,
		stubYards{yard: yard, claimedKinds: &claimed}, repo, &fightEngine{result: result}, voice,
		fakeClock{t: now}, &sequenceRNG{values: []int{3, 4}}, events,
	)

	outcome, err := svc.Toggle(context.Background(), -100, winner.UserID)
	if err != nil {
		t.Fatalf("Toggle() error = %v", err)
	}
	if outcome.Banter == nil || outcome.Banter.FirstCat.ID != loser.ID || outcome.Banter.SecondCat.ID != winner.ID || outcome.Banter.FirstLine == "" || outcome.Banter.SecondLine == "" {
		t.Fatalf("fight banter = %+v", outcome.Banter)
	}
	if len(claimed) != 1 || claimed[0] != domain.AutoMessageBanter {
		t.Fatalf("claimed auto-message kinds = %v", claimed)
	}
	request := provider.prompt.Request
	if request.ArenaBanter == nil || request.ArenaBanter.Friendship != 2 || request.ArenaBanter.Rivalry != 9 ||
		request.ArenaBanter.Respect != 4 || request.ArenaBanter.WinnerStreak != 3 || len(request.Relationships) != 1 {
		t.Fatalf("relationship-aware banter request = %+v", request)
	}
	if !strings.Contains(provider.prompt.User, "winner_streak=3") || !strings.Contains(provider.prompt.User, "friendship=2 rivalry=9 respect=4") {
		t.Fatalf("relationship facts missing from prompt: %q", provider.prompt.User)
	}
	payload, ok := events.events[0].Payload.(FightFinishedPayload)
	if !ok || payload.Banter == nil || !events.events[0].Notable {
		t.Fatalf("fight event payload = %+v", events.events[0])
	}
}

func TestFightNotabilityDoesNotTurnEveryFightIntoBanter(t *testing.T) {
	t.Parallel()
	catA := &domain.Cat{ID: 10, Level: 3}
	catB := &domain.Cat{ID: 20, Level: 3}
	ordinary := fightNotability(catA, catB, domain.FightResult{
		WinnerCatID: catA.ID, LoserCatID: catB.ID, FinalHPA: 12,
	}, domain.FightSaveResult{
		Rivalry: 2, Stats: domain.FightStats{PairCatAWins: 1, PairCatBWins: 1},
	}, domain.FightKindRegular)
	if ordinary.Notable {
		t.Fatalf("ordinary fight marked notable: %+v", ordinary)
	}

	deciding := fightNotability(catA, catB, domain.FightResult{
		WinnerCatID: catA.ID, LoserCatID: catB.ID, FinalHPA: 12,
	}, domain.FightSaveResult{
		Rivalry: 3, Stats: domain.FightStats{PairCatAWins: 2, PairCatBWins: 1},
	}, domain.FightKindRegular)
	if !deciding.Notable || !deciding.DecidingFight {
		t.Fatalf("deciding fight not notable: %+v", deciding)
	}
}

func TestFightLockedBeforeLevelThree(t *testing.T) {
	cat := &domain.Cat{ID: 10, UserID: 1, Level: 2}
	repo := &fakeFightRepo{}
	engine := &fightEngine{}
	service := NewFightService(fightCats{1: cat}, nil, stubYards{yard: &domain.Yard{ID: 7, FightsEnabled: true}}, repo, engine, nil, fakeClock{t: time.Unix(123, 0)}, zeroRNG{}, nil)
	if _, err := service.Toggle(context.Background(), -100, 1); !errors.Is(err, domain.ErrFeatureLocked) {
		t.Fatalf("low-level fight: %v", err)
	}
	if engine.calls != 0 || repo.saved {
		t.Fatal("locked arena ran game logic")
	}
}

func TestFightQueueExpiresAfterHoursOrAtGameMidnight(t *testing.T) {
	for _, tc := range []struct{ now, until string }{
		{"2026-09-07T06:00:00Z", "2026-09-07T18:00:00Z"},
		{"2026-09-07T17:00:00Z", "2026-09-07T21:00:00Z"},
		{"2026-09-07T21:00:00Z", "2026-09-08T09:00:00Z"},
		{"2026-09-07T20:59:00Z", "2026-09-07T21:00:00Z"},
	} {
		now, _ := time.Parse(time.RFC3339, tc.now)
		want, _ := time.Parse(time.RFC3339, tc.until)
		if got := fightQueueExpiresAt(now); !got.Equal(want) {
			t.Errorf("%s: got %v want %v", tc.now, got, want)
		}
	}
}
