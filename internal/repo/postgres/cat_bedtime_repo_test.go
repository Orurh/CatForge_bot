package postgres

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBedtimeClaimSurvivesConcurrencyRestartAndMidnight(t *testing.T) {
	pool := paymentTestPool(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 11, 20, 55, 0, 0, time.UTC)
	sleepAt := now.Add(5 * time.Minute)
	_, err := pool.Exec(ctx, `INSERT INTO users(id,telegram_id) VALUES(1,101),(2,102),(3,103);
 INSERT INTO cats(id,user_id,name,breed,trait,hp_base,atk_base,def_base,spd_base)
 SELECT id,id,'Cat','british','lazy',10,10,10,10 FROM users;
 INSERT INTO cat_personality(cat_id,trait,speech_style,auto_speak_enabled)
 SELECT id,'lazy','lazy',id<>3 FROM cats ON CONFLICT(cat_id) DO UPDATE SET auto_speak_enabled=EXCLUDED.auto_speak_enabled;
 INSERT INTO yards(id,telegram_chat_id,name,auto_messages_enabled,cat_to_cat_banter,max_auto_messages_day)
 SELECT id,-100-id,'Yard',id<>2,id<>3,CASE WHEN id=4 THEN 0 ELSE 2 END FROM generate_series(1,8) AS id;`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `UPDATE yards SET quiet_until=$1 WHERE id=5;
 `, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO yard_members(yard_id,user_id,cat_id,last_active_at)
 SELECT y.id,c.id,c.id,CASE WHEN y.id=6 THEN $1::timestamptz - interval '8 days' ELSE $1 END
 FROM yards y JOIN cats c ON c.id=CASE WHEN y.id=6 THEN 2 WHEN y.id=7 THEN 3 ELSE 1 END WHERE y.id<>8`, now)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewYardRepo(pool)
	candidates, err := repo.ListBedtimeCandidates(ctx, now, sleepAt, now.Add(-7*24*time.Hour), 20)
	if err != nil || len(candidates) != 1 || candidates[0].ID != 1 {
		t.Fatalf("candidates: %+v %v", candidates, err)
	}
	for _, ids := range [][2]int64{{1, 999}, {2, 1}, {3, 1}, {4, 1}, {5, 1}, {7, 3}, {8, 1}} {
		if ok, err := repo.ClaimBedtimeFarewell(ctx, ids[0], ids[1], now, sleepAt); ok || err != nil {
			t.Fatalf("disabled claim %v: %v %v", ids, ok, err)
		}
	}
	var claims atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := NewYardRepo(pool).ClaimBedtimeFarewell(ctx, 1, 1, now, sleepAt)
			if err != nil {
				t.Error(err)
			}
			if ok {
				claims.Add(1)
			}
		}()
	}
	wg.Wait()
	if claims.Load() != 1 {
		t.Fatalf("concurrent claims: %d", claims.Load())
	}
	restarted := NewYardRepo(pool)
	if ok, err := restarted.ClaimBedtimeFarewell(ctx, 1, 1, now.Add(time.Minute), sleepAt); ok || err != nil {
		t.Fatalf("restart repeated farewell: %v %v", ok, err)
	}
	if got, err := restarted.ListBedtimeCandidates(ctx, now, sleepAt, now.Add(-7*24*time.Hour), 20); len(got) != 0 || err != nil {
		t.Fatalf("already claimed candidates: %+v %v", got, err)
	}
	if ok, err := restarted.ClaimBedtimeFarewell(ctx, 1, 1, sleepAt, sleepAt); ok || err != nil {
		t.Fatalf("midnight claim: %v %v", ok, err)
	}
	if ok, err := restarted.ClaimBedtimeFarewell(ctx, 1, 1, now.Add(24*time.Hour), sleepAt.Add(24*time.Hour)); !ok || err != nil {
		t.Fatalf("next evening: %v %v", ok, err)
	}
	if ok, err := restarted.ClaimBedtimeFarewell(ctx, 1, 1, now, sleepAt); ok || err != nil {
		t.Fatalf("clock rewind repeated farewell: %v %v", ok, err)
	}
}
