package postgres

import (
	"context"
	"fmt"
	"testing"

	"catforge/internal/ai"
)

func TestConversationMemoryIsolationRetentionAndRestart(t *testing.T) {
	pool := paymentTestPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO users(id,telegram_id) VALUES(1,101),(2,102);
 INSERT INTO cats(id,user_id,name,breed,trait,hp_base,atk_base,def_base,spd_base)
 SELECT id,id,'Cat','british','lazy',10,10,10,10 FROM users;`)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewConversationMemoryRepo(pool)
	for i := 0; i < 35; i++ {
		if err := repo.Remember(ctx, "yard:1", ai.SpeechMemory{CatID: 1, OtherCatID: 2, CatName: "One", OtherCatName: "Two", Text: fmt.Sprintf("line %d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM cat_speech_memory WHERE scope_key='yard:1'`).Scan(&count); err != nil || count != 30 {
		t.Fatalf("retention=%d %v", count, err)
	}
	restarted := NewConversationMemoryRepo(pool)
	for _, scope := range []string{"yard:2", "chat:101:cat:1", "chat:102:cat:1"} {
		got, err := restarted.Recent(ctx, scope, 1, 2)
		if err != nil || len(got) != 0 {
			t.Fatalf("scope leak %s: %+v %v", scope, got, err)
		}
	}
	got, err := restarted.Recent(ctx, "yard:1", 1, 2)
	if err != nil || len(got) != 5 || got[0].Text != "line 30" || got[4].Text != "line 34" {
		t.Fatalf("history: %+v %v", got, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE cat_speech_memory SET created_at=now()-interval '8 days'`); err != nil {
		t.Fatal(err)
	}
	got, err = restarted.Recent(ctx, "yard:1", 1, 2)
	if err != nil || len(got) != 0 {
		t.Fatalf("expired history: %+v %v", got, err)
	}
}
