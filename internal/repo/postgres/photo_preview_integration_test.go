package postgres

import (
	"catforge/internal/app"
	"context"
	"testing"
	"time"
)

func TestPhotoPreviewPayloadSurvivesReloadAndIsClearedOnNewInput(t *testing.T) {
	pool := paymentTestPool(t)
	ctx := context.Background()
	repo := NewUserRepo(pool)
	uid, err := repo.EnsureUser(ctx, 97654321)
	if err != nil {
		t.Fatal(err)
	}
	p := app.PendingInput{UserID: uid, ChatID: 97654321, Kind: "await_cat_photo", PromptMessageID: 5, ExpiresAt: time.Now().Add(time.Minute), Payload: `{"CatID":3,"FileID":"abc","Token":"nonce"}`}
	if err = repo.SavePendingInput(ctx, p); err != nil {
		t.Fatal(err)
	}
	fresh := NewUserRepo(pool)
	got, err := fresh.GetPendingInput(ctx, uid, p.ChatID)
	if err != nil || got == nil || got.Payload != p.Payload {
		t.Fatal("draft not persisted", got, err)
	}
	if other, err := fresh.GetPendingInput(ctx, uid, p.ChatID+1); err != nil || other != nil {
		t.Fatal("draft leaked to another chat")
	}
	p.Payload = ""
	p.Kind = app.PendingAwaitCatName
	if err = repo.SavePendingInput(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, err = fresh.GetPendingInput(ctx, uid, p.ChatID)
	if err != nil || got == nil || got.Payload != "" {
		t.Fatal("old photo draft survived new input")
	}
}
