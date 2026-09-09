package postgres

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroupCooldownConcurrencyAndPersistence(t *testing.T) {
	pool := paymentTestPool(t)
	ctx := context.Background()
	repo := NewGroupCooldownRepo(pool)
	now := time.Date(2026, 9, 9, 17, 0, 0, 0, time.UTC)
	var successes atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _, err := repo.Claim(ctx, -100, 0, "week", now, 15*time.Minute)
			if err != nil {
				t.Error(err)
			}
			if ok {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("concurrent claims admitted %d", successes.Load())
	}
	restarted := NewGroupCooldownRepo(pool)
	ok, deadline, err := restarted.Claim(ctx, -100, 0, "week", now.Add(time.Minute), 15*time.Minute)
	if err != nil || ok || !deadline.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("restart/denial extended cooldown: %v %v %v", ok, deadline, err)
	}
	ok, _, err = repo.Claim(ctx, -200, 0, "week", now, 15*time.Minute)
	if err != nil || !ok {
		t.Fatalf("other chat blocked: %v %v", ok, err)
	}
	for _, uid := range []int64{1, 2} {
		ok, _, err = repo.Claim(ctx, -100, uid, "training", now, 10*time.Second)
		if err != nil || !ok {
			t.Fatalf("other user blocked: %v %v", ok, err)
		}
	}
	ok, newDeadline, err := repo.Claim(ctx, -100, 0, "week", deadline, 15*time.Minute)
	if err != nil || !ok {
		t.Fatalf("expiry boundary: %v %v", ok, err)
	}
	if err = repo.Release(ctx, -100, 0, "week", deadline); err != nil {
		t.Fatal(err)
	}
	ok, _, err = repo.Claim(ctx, -100, 0, "week", deadline.Add(time.Second), 15*time.Minute)
	if err != nil || ok {
		t.Fatalf("stale release removed newer claim: %v %v", ok, err)
	}
	if err = repo.Release(ctx, -100, 0, "week", newDeadline); err != nil {
		t.Fatal(err)
	}
	ok, _, err = repo.Claim(ctx, -100, 0, "week", deadline.Add(time.Second), 15*time.Minute)
	if err != nil || !ok {
		t.Fatalf("failed action was not released: %v %v", ok, err)
	}
}
