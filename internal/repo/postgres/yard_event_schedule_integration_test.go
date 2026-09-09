package postgres

import (
	"context"
	"testing"
	"time"
)

func TestYardEventScheduleMatchesScheduler(t *testing.T) {
	pool := paymentTestPool(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `INSERT INTO users(id,telegram_id) VALUES(1,101),(2,102);
 INSERT INTO cats(id,user_id,name,breed,trait,hp_base,atk_base,def_base,spd_base)
 SELECT id,id,'Cat','british','lazy',10,10,10,10 FROM users;
 INSERT INTO yards(id,telegram_chat_id,name,created_at) VALUES(1,-101,'Yard','2026-09-09T12:00:00Z');
 INSERT INTO yard_members(yard_id,user_id,cat_id,last_active_at) SELECT 1,id,id,'2026-09-09T12:00:00Z' FROM cats;`)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewYardEventRepo(pool)
	since := now.Add(-7 * 24 * time.Hour)
	status, err := repo.GetSchedule(ctx, 1, since)
	if err != nil || status.ActiveCats != 2 || status.NextAt.Before(now.Add(24*time.Hour)) || status.NextAt.After(now.Add(48*time.Hour)) {
		t.Fatalf("schedule: %+v %v", status, err)
	}
	before, err := repo.ListStartCandidates(ctx, status.NextAt.Add(-time.Second), since, 20)
	if err != nil || len(before) != 0 {
		t.Fatalf("started early: %v %v", before, err)
	}
	due, err := repo.ListStartCandidates(ctx, status.NextAt, since, 20)
	if err != nil || len(due) != 1 {
		t.Fatalf("not due at displayed time: %v %v", due, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE yard_members SET last_active_at=$1 WHERE cat_id=2`, since.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	status, err = repo.GetSchedule(ctx, 1, since)
	if err != nil || status.ActiveCats != 1 {
		t.Fatalf("inactive count: %+v %v", status, err)
	}
	due, err = repo.ListStartCandidates(ctx, status.NextAt, since, 20)
	if err != nil || len(due) != 0 {
		t.Fatalf("inactive yard scheduled: %v %v", due, err)
	}
}
