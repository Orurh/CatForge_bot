package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"catforge/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestTraitMigrationPreservesCatsAndSynchronizesPersonalities(t *testing.T) {
	dsn := os.Getenv("CATFORGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CATFORGE_TEST_DATABASE_URL")
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
	schema := pgx.Identifier{fmt.Sprintf("traits_test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg.ConnConfig)
	defer db.Close()
	if err = goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpTo(db, "../../../migrations", 202609130010); err != nil {
		t.Fatal(err)
	}

	legacy := []string{"lazy", "bully", "philosopher", "neat", "sleepy", "sleepy", "unknown", "veteran", "cunning", "arrogant", "angry"}
	for i, trait := range legacy {
		id := i + 1
		if _, err = db.Exec(`INSERT INTO users(id,telegram_id) VALUES($1,$1)`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO cats(id,user_id,name,breed,trait,level,xp,coins,energy,hp_base,atk_base,def_base,spd_base,state_version)
   VALUES($1,$1,'Legacy','british',$2,7,47,999,73,60,40,30,25,19)`, id, trait); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`UPDATE cat_personality SET speech_style='old custom style', auto_speak_enabled=(cat_id % 2 = 0), humor_mode='bold';
  UPDATE cat_personality SET trait='neat' WHERE cat_id=1;
  DELETE FROM cat_personality WHERE cat_id=7;`); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(db, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	expected := []domain.Trait{domain.TraitLazy, domain.TraitBully, domain.TraitVeteran, domain.TraitCunning, domain.TraitAngry, domain.TraitArrogant, domain.TraitBully, domain.TraitVeteran, domain.TraitCunning, domain.TraitArrogant, domain.TraitAngry}
	for i, want := range expected {
		var trait, personality, style, mode string
		var enabled bool
		var level, xp, coins, energy, hp, atk, def, spd, version int
		if err = db.QueryRow(`SELECT c.trait,p.trait,p.speech_style,p.humor_mode,p.auto_speak_enabled,c.level,c.xp,c.coins,c.energy,c.hp_base,c.atk_base,c.def_base,c.spd_base,c.state_version
   FROM cats c JOIN cat_personality p ON p.cat_id=c.id WHERE c.id=$1`, i+1).Scan(&trait, &personality, &style, &mode, &enabled, &level, &xp, &coins, &energy, &hp, &atk, &def, &spd, &version); err != nil {
			t.Fatal(err)
		}
		if trait != string(want) || personality != trait || style != domain.DefaultSpeechStyle(want) {
			t.Fatalf("cat %d: %s/%s/%q; want %s", i+1, trait, personality, style, want)
		}
		if level != 7 || xp != 47 || coins != 999 || energy != 73 || hp != 60 || atk != 40 || def != 30 || spd != 25 || version != 20 {
			t.Fatalf("cat %d lost progress or state version", i+1)
		}
		if i != 6 && (mode != "bold" || enabled != ((i+1)%2 == 0)) {
			t.Fatalf("cat %d lost speech settings", i+1)
		}
	}
	// The insert trigger must give every new trait the same speech style as Go.
	for i, trait := range domain.AllTraits {
		id := 100 + i
		if _, err = db.Exec(`INSERT INTO users(id,telegram_id) VALUES($1,$1)`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO cats(id,user_id,name,breed,trait,hp_base,atk_base,def_base,spd_base) VALUES($1,$1,'New','british',$2,10,10,10,10)`, id, string(trait)); err != nil {
			t.Fatal(err)
		}
		var style, stored string
		if err = db.QueryRow(`SELECT trait,speech_style FROM cat_personality WHERE cat_id=$1`, id).Scan(&stored, &style); err != nil {
			t.Fatal(err)
		}
		if stored != string(trait) || style != domain.DefaultSpeechStyle(trait) {
			t.Fatalf("trigger mismatch for %s: %s %q", trait, stored, style)
		}
	}
	for _, table := range []string{"cats", "cat_personality"} {
		if _, err = db.Exec("UPDATE " + table + " SET trait='philosopher'"); err == nil {
			t.Fatalf("%s accepted retired trait", table)
		}
	}
	if err = Migrate(db, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	var version int
	if err = db.QueryRow(`SELECT state_version FROM cats WHERE id=1`).Scan(&version); err != nil || version != 20 {
		t.Fatalf("migration reran: %d %v", version, err)
	}
}
