package telegram

import (
	"catforge/internal/app"
	"catforge/internal/logx"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func stringKey(chat, actor int64, action string) string {
	return fmt.Sprintf("%d:%d:%s", chat, actor, action)
}

type fakeGroupCooldowns struct {
	next map[string]time.Time
	keys []string
}

func (f *fakeGroupCooldowns) Claim(_ context.Context, chat, actor int64, action string, now time.Time, interval time.Duration) (bool, time.Time, error) {
	key := stringKey(chat, actor, action)
	f.keys = append(f.keys, key)
	if next := f.next[key]; next.After(now) {
		return false, next, nil
	}
	next := now.Add(interval)
	f.next[key] = next
	return true, next, nil
}
func (f *fakeGroupCooldowns) Release(_ context.Context, chat, actor int64, action string, next time.Time) error {
	key := stringKey(chat, actor, action)
	if f.next[key].Equal(next) {
		delete(f.next, key)
	}
	return nil
}

func TestWeeklyCooldownIsSharedAndNoticesAreThrottled(t *testing.T) {
	sends := 0
	sender := NewSender("test", logx.Nop())
	sender.apiBaseURL = "http://telegram.test"
	sender.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var p map[string]any
		if err := json.NewDecoder(req.Body).Decode(&p); err != nil {
			t.Error(err)
		}
		sends++
		return jsonResponse(`{"ok":true,"result":{"message_id":1}}`), nil
	})}
	r := NewRouter(&app.App{}, sender, "", "", "", nil)
	store := &fakeGroupCooldowns{next: map[string]time.Time{}}
	r.SetGroupCooldownStore(store)
	now := time.Now()
	first := &tgCtx{chatID: -100, userID: 1, chatType: "supergroup", now: now}
	if ok, _ := r.claimGroupAction(context.Background(), first, 0, "week", 15*time.Minute); !ok {
		t.Fatal("first claim blocked")
	}
	// No WeeklySummary service: reaching generation would panic. All users must
	// share the already reserved cooldown and only one denial is posted.
	for i := int64(2); i < 12; i++ {
		r.showYardWeeklySummary(context.Background(), &tgCtx{chatID: -100, userID: i, chatType: "supergroup", now: now}, &Message{MessageID: int(i)})
	}
	if sends != 1 {
		t.Fatalf("denials spammed %d messages", sends)
	}
}

func TestGroupCooldownPoliciesAndPrivateExemption(t *testing.T) {
	store := &fakeGroupCooldowns{next: map[string]time.Time{}}
	r := NewRouter(&app.App{}, nil, "", "", "", nil)
	r.SetGroupCooldownStore(store)
	ctx := context.Background()
	now := time.Now()
	for i := 0; i < 3; i++ {
		if !r.allowGroupCommand(ctx, &tgCtx{chatID: 1, userID: 1, chatType: "private", now: now}, "/train") {
			t.Fatal("private blocked")
		}
	}
	if len(store.keys) != 0 {
		t.Fatal("private consumed group quota")
	}
	group := &tgCtx{chatID: -100, userID: 1, chatType: "group", now: now}
	if !r.allowGroupCommand(ctx, group, "/train") || r.allowGroupCommand(ctx, group, "/fight") {
		t.Fatal("rapid mixed commands bypassed input limit")
	}
	group.now = now.Add(3 * time.Second)
	if !r.allowGroupCommand(ctx, group, "/quiet") {
		t.Fatal("quiet switch blocked after input cooldown")
	}
	train, interval, actor := groupCommandCooldown("/train", 1)
	hunt, huntInterval, huntActor := groupCommandCooldown("/hunt", 1)
	if train != hunt || interval != huntInterval || actor != huntActor {
		t.Fatal("training alias bypass")
	}
	for _, cmd := range []string{"/yard", "/event"} {
		_, d, u := groupCommandCooldown(cmd, 999)
		if d != 30*time.Second || u != 0 {
			t.Fatal("public status is not shared")
		}
	}
}
