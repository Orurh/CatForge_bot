package postgres

import (
	"catforge/internal/domain"
	"catforge/internal/gameengine"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"os"
	"testing"
	"time"
)

// Uses its own schema; no bot credentials or external Telegram/AI calls.
func TestFelineProgressionIntegration(t *testing.T) {
	dsn, address := os.Getenv("CATFORGE_TEST_DATABASE_URL"), os.Getenv("CATFORGE_TEST_ENGINE")
	if dsn == "" || address == "" {
		t.Skip("set CATFORGE_TEST_DATABASE_URL and CATFORGE_TEST_ENGINE for integration test")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("feline_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE")
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg.ConnConfig)
	defer db.Close()
	if err = goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpTo(db, "../../../migrations", 202609050030); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO users(telegram_id) VALUES(999);
 INSERT INTO cats(user_id,name,breed,trait,level,xp,coins,hp_base,atk_base,def_base,spd_base) SELECT id,'Legacy','british','lazy',7,47,999,60,40,30,25 FROM users WHERE telegram_id=999;
 INSERT INTO cat_items(user_id,item_id,item_level) SELECT id,'rat_tooth',4 FROM users WHERE telegram_id=999;`); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(db, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var legacyLevel, legacyXP, legacyCoins, legacyItemLevel, claws int
	if err = pool.QueryRow(ctx, `SELECT c.level,c.xp,c.coins,i.item_level,c.claws_tenth_mm FROM cats c JOIN users u ON u.id=c.user_id JOIN cat_items i ON i.user_id=u.id WHERE u.telegram_id=999`).Scan(&legacyLevel, &legacyXP, &legacyCoins, &legacyItemLevel, &claws); err != nil {
		t.Fatal(err)
	}
	if legacyLevel != 7 || legacyXP != 47 || legacyCoins != 999 || legacyItemLevel != 4 || claws != 200 {
		t.Fatalf("migration lost progress: %d/%d/%d/%d/%d", legacyLevel, legacyXP, legacyCoins, legacyItemLevel, claws)
	}
	engine, err := gameengine.NewRemoteEngine(address)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	if err := engine.Check(ctx); err != nil {
		t.Fatalf("engine readiness: %v", err)
	}
	cats, items, yards := NewCatRepo(pool), NewItemRepo(pool), NewYardRepo(pool)
	userID, err := NewUserRepo(pool).EnsureUser(ctx, 123456)
	if err != nil {
		t.Fatal(err)
	}
	hp, atk, def, spd := domain.BaseStatsByBreed(domain.BreedBritish)
	cat, err := cats.Create(ctx, userID, "Test", domain.BreedBritish, domain.TraitLazy, hp, atk, def, spd)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Feline != domain.BaseFelineStats(cat.Breed) {
		t.Fatalf("starter stats: %+v", cat)
	}
	now := time.Now()
	trained, err := engine.Train(ctx, gameengine.TrainingInput{RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion, Cat: *cat, Now: now, Random: gameengine.TrainingRandom{EncounterRoll: 50}})
	if err != nil {
		t.Fatal(err)
	}
	if trained.Cat.Level != 2 || trained.Cat.LootItemID != "rat_tooth" {
		t.Fatalf("first reward: %+v", trained)
	}
	saved, err := cats.SaveProgress(ctx, userID, cat.StateVersion, trained.Cat)
	if err != nil || !saved {
		t.Fatalf("save: %v %v", saved, err)
	}
	saved, err = cats.SaveProgress(ctx, userID, cat.StateVersion, trained.Cat)
	if err != nil || saved {
		t.Fatalf("stale save: %v %v", saved, err)
	}
	owned, err := items.ListOwned(ctx, userID)
	if err != nil || len(owned) != 1 || !owned[0].Equipped || owned[0].Level != 1 {
		t.Fatalf("first equipment: %+v %v", owned, err)
	}
	cat, err = cats.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if cat.TrainingCritBonusPercent != 2 {
		t.Fatalf("equipped tooth crit bonus: %d", cat.TrainingCritBonusPercent)
	}
	probeCat := *cat
	probeCat.Energy = 50
	probeCat.EnergyUpdatedAt = now
	critProbe, critErr := engine.Train(ctx, gameengine.TrainingInput{RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion, Cat: probeCat, Now: now, Random: gameengine.TrainingRandom{TrainingCritRoll: 6, XPGainRoll: 25, EncounterRoll: 50}, TrainingCritBonusPercent: cat.TrainingCritBonusPercent})
	if critErr != nil || !critProbe.Result.Crit || critProbe.Result.XPGain != 150 {
		t.Fatalf("equipped training crit: %+v %v", critProbe.Result, critErr)
	}

	for _, tc := range []struct {
		energy, roll int
		drop         bool
	}{{50, 99, true}, {50, 100, false}, {75, 149, true}, {75, 150, false}, {100, 199, true}, {100, 200, false}} {
		probe := *cat
		probe.Energy = tc.energy
		probe.EnergyUpdatedAt = now
		probe.FirstItemGranted = true
		probe.Coins = 777
		result, err := engine.Train(ctx, gameengine.TrainingInput{RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion, Cat: probe, Now: now, Random: gameengine.TrainingRandom{TrainingCritRoll: 99, TrainingLootRoll: tc.roll}})
		if err != nil || (result.Cat.LootItemID != "") != tc.drop || result.Cat.Coins != 777 || result.Result.CoinsGain != 0 {
			t.Fatalf("loot RPC %v: %+v %v", tc, result, err)
		}
	}
	cat.LootItemID = "rat_tooth"
	cat.ProgressionFacts = []string{"new_item=rat_tooth"}
	if saved, err = cats.SaveProgress(ctx, userID, cat.StateVersion, *cat); err != nil || !saved {
		t.Fatalf("duplicate: %v %v", saved, err)
	}
	owned, err = items.ListOwned(ctx, userID)
	if err != nil || len(owned) != 1 || owned[0].Level != 2 || owned[0].Fragments != 0 {
		t.Fatalf("auto upgrade: %+v %v", owned, err)
	}
	yard, _, _, err := yards.EnsureAndJoin(ctx, -123456, "Test yard", userID, cat.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	events := NewYardEventRepo(pool, engine)
	event, _, err := events.StartOrGet(ctx, yard.ID, domain.YardEventFishTruck, 111, now, now.Add(time.Hour), gameengine.CurrentContentVersion)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = events.SubmitChoice(ctx, -123456, event.ID, userID, "service_entry", now); !errors.Is(err, domain.ErrFeatureLocked) {
		t.Fatalf("missing capability: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO cat_items(user_id,item_id) VALUES($1,'janitor_glove')`, userID); err != nil {
		t.Fatal(err)
	}
	if err = items.Equip(ctx, userID, domain.ItemSlotClaws, "janitor_glove"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = events.SubmitChoice(ctx, -123456, event.ID, userID, "service_entry", now); err != nil {
		t.Fatal(err)
	}
	if err = items.Equip(ctx, userID, domain.ItemSlotClaws, "rat_tooth"); err != nil {
		t.Fatal(err)
	}
	input, err := events.GetResolutionInput(ctx, event.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Participants) != 1 || input.Participants[0].SpecialAction != "service_entry" {
		t.Fatalf("snapshot: %+v", input)
	}
	result, err := engine.ResolveYardEvent(ctx, gameengine.YardEventInput{RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion, EventType: event.Type, Seed: uint64(event.Seed), Participants: input.Participants})
	if err != nil {
		t.Fatal(err)
	}
	if saved, err = events.SaveResolution(ctx, event.ID, result, gameengine.CurrentRulesVersion, gameengine.CurrentContentVersion, now.Add(time.Hour)); err != nil || !saved {
		t.Fatalf("event save: %v %v", saved, err)
	}
	before, err := cats.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if saved, err = events.SaveResolution(ctx, event.ID, result, gameengine.CurrentRulesVersion, gameengine.CurrentContentVersion, now.Add(time.Hour)); err != nil || saved {
		t.Fatalf("event replay: %v %v", saved, err)
	}
	after, err := cats.GetByUserID(ctx, userID)
	if err != nil || before.XP != after.XP || before.StateVersion != after.StateVersion {
		t.Fatalf("replay changed progress: %v", err)
	}
	if _, err = events.WeeklySummary(ctx, yard.ID, now.Add(-time.Hour), now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// Arena saves XP, unlock facts and the global daily limit in one transaction.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	catA, err := awardCatXP(ctx, tx, engine, cat.ID, 200, "arena", 0, "", false)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	otherUser, err := NewUserRepo(pool).EnsureUser(ctx, 123457)
	if err != nil {
		t.Fatal(err)
	}
	other, err := cats.Create(ctx, otherUser, "Other", domain.BreedBritish, domain.TraitLazy, hp, atk, def, spd)
	if err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	catB, err := awardCatXP(ctx, tx, engine, other.ID, 300, "arena", 0, "", false)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = yards.EnsureAndJoin(ctx, -123456, "Test yard", otherUser, other.ID, now); err != nil {
		t.Fatal(err)
	}
	fight, err := engine.Fight(ctx, gameengine.FightInput{RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion, Seed: 7, CatA: catA, CatB: catB})
	if err != nil {
		t.Fatal(err)
	}
	gainA, gainB := int64(catA.Level*3), int64(catB.Level*3)
	if fight.WinnerCatID == catA.ID {
		gainA = int64(catA.Level * 5)
	} else {
		gainB = int64(catB.Level * 5)
	}
	record := domain.FightRecord{RivalryDelta: 1, CatAVersion: catA.StateVersion, CatBVersion: catB.StateVersion, YardID: yard.ID, CatAID: catA.ID, CatBID: catB.ID, WinnerCatID: fight.WinnerCatID, LoserCatID: fight.LoserCatID, Seed: 7, CatAXPGain: gainA, CatBXPGain: gainB, RulesVersion: gameengine.CurrentRulesVersion, ContentVersion: gameengine.CurrentContentVersion, CreatedAt: now}
	fights := NewFightRepo(pool, engine)
	limits := domain.FightLimits{DailySince: now.Add(-24 * time.Hour), DailyLimit: 1, PairSince: now.Add(-time.Hour), PairLimit: 3}
	if _, err = fights.SaveFight(ctx, record, fight, catA.Name, catB.Name, limits); err != nil {
		t.Fatal(err)
	}
	if _, err = fights.SaveFight(ctx, record, fight, catA.Name, catB.Name, limits); err == nil {
		t.Fatal("fight replay succeeded")
	}
	var fightCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM yard_fights`).Scan(&fightCount); err != nil || fightCount != 1 {
		t.Fatalf("fight count %d: %v", fightCount, err)
	}
	otherOwned, err := items.ListOwned(ctx, otherUser)
	if err != nil || len(otherOwned) != 0 {
		t.Fatalf("arena dropped power loot: %+v %v", otherOwned, err)
	}

}
