package postgres

import (
	"catforge/internal/app"
	"catforge/internal/domain"
	"context"
	"testing"
	"time"
)

func TestAsynchronousFightQueueIntegration(t *testing.T) {
	pool := paymentTestPool(t)
	ctx := context.Background()
	cats, users, yards := NewCatRepo(pool), NewUserRepo(pool), NewYardRepo(pool)
	now := time.Date(2026, 9, 7, 6, 0, 0, 0, time.UTC)
	var players []*domain.Cat
	var yardID int64
	for _, tgID := range []int64{100, 200} {
		uid, err := users.EnsureUser(ctx, tgID)
		if err != nil {
			t.Fatal(err)
		}
		cat, err := cats.Create(ctx, uid, "Arena", domain.BreedBritish, domain.TraitLazy, 30, 20, 20, 20)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, "UPDATE cats SET level=3 WHERE id=$1", cat.ID); err != nil {
			t.Fatal(err)
		}
		yard, _, _, err := yards.EnsureAndJoin(ctx, -999, "Test", uid, cat.ID, now)
		if err != nil {
			t.Fatal(err)
		}
		yardID = yard.ID
		players = append(players, cat)
	}
	repo := NewFightRepo(pool)
	toggle := func(cat *domain.Cat, at time.Time) domain.FightQueueToggle {
		out, err := repo.ToggleQueue(ctx, yardID, cat.UserID, cat.ID, at, at.Add(app.FightQueueTTL), domain.FightLimits{DailySince: at.Add(-24 * time.Hour), DailyLimit: 1, PairSince: at.Add(-30 * time.Minute), PairLimit: 3})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if out := toggle(players[0], now); out.Status != domain.FightQueueWaiting {
		t.Fatal(out)
	}
	if out := toggle(players[1], now.Add(3*time.Hour)); out.Status != domain.FightQueueMatched || out.Opponent.CatID != players[0].ID {
		t.Fatalf("3h asynchronous match: %+v", out)
	}
	if out := toggle(players[0], now.Add(4*time.Hour)); out.Status != domain.FightQueueWaiting {
		t.Fatal(out)
	}
	if out := toggle(players[0], now.Add(5*time.Hour)); out.Status != domain.FightQueueCanceled {
		t.Fatal("repeat fight did not cancel")
	}
	toggle(players[0], now.Add(6*time.Hour))
	if out := toggle(players[1], now.Add(19*time.Hour)); out.Status != domain.FightQueueWaiting {
		t.Fatal("expired opponent matched")
	}
}
