package postgres

import (
	"context"
	"testing"
	"time"
)

func TestBanterRestartWaitsFullInterval(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	repo := NewYardRepo(nil)
	repo.banterStarted = now
	yards, err := repo.ListIdleBanterCandidates(context.Background(), now.Add(2*time.Hour-time.Second), time.Time{}, 20)
	if err != nil || len(yards) != 0 {
		t.Fatalf("early run: %v %v", yards, err)
	}
}

func TestPeriodicBanterClaimsEachYardWithoutCatchup(t *testing.T) {
	pool := paymentTestPool(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `INSERT INTO users(id,telegram_id) VALUES(1,101),(2,102);
 INSERT INTO cats(id,user_id,name,breed,trait,hp_base,atk_base,def_base,spd_base)
 SELECT id,id,'Cat','british','lazy',10,10,10,10 FROM users;
 INSERT INTO cat_personality(cat_id,trait,speech_style,auto_speak_enabled)
 SELECT id,'lazy','lazy',true FROM cats ON CONFLICT (cat_id) DO UPDATE SET auto_speak_enabled=true;
 INSERT INTO yards(id,telegram_chat_id,name,auto_messages_enabled,cat_to_cat_banter) VALUES(1,-101,'One',true,true),(2,-102,'Two',true,true);
 INSERT INTO yard_members(yard_id,user_id,cat_id,last_active_at)
 SELECT y.id,c.id,c.id,'2020-01-01' FROM yards y CROSS JOIN cats c;`)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewYardRepo(pool)
	repo.banterStarted = now.Add(-2 * time.Hour)
	for _, tc := range []struct {
		at    time.Time
		count int
	}{
		{now, 2}, {now, 0}, {now.Add(time.Hour), 0}, {now.Add(2 * time.Hour), 2},
		{now.Add(72 * time.Hour), 2}, {now.Add(72 * time.Hour), 0},
	} {
		got, err := repo.ListIdleBanterCandidates(ctx, tc.at, now, 20)
		if err != nil || len(got) != tc.count {
			t.Fatalf("at %v: count=%d want=%d err=%v", tc.at, len(got), tc.count, err)
		}
	}
}
