package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestDeleteWebhookKeepsPendingUpdates(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/bottest-token/deleteWebhook" {
			t.Errorf("path = %q", req.URL.Path)
		}
		var payload map[string]bool
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if payload["drop_pending_updates"] {
			t.Error("drop_pending_updates must be false")
		}
		return jsonResponse(`{"ok":true,"result":true}`), nil
	})}

	bot := New(nil, "test-token", "", "", "")
	bot.apiBaseURL = "http://telegram.test"
	bot.httpClient = client
	if err := bot.DeleteWebhook(context.Background(), false); err != nil {
		t.Fatalf("DeleteWebhook() error = %v", err)
	}
}

func TestPollDeliversUpdatesInOrder(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/bottest-token/getUpdates" {
			t.Errorf("path = %q", req.URL.Path)
		}
		if calls.Add(1) == 1 {
			return jsonResponse(`{"ok":true,"result":[{"update_id":41},{"update_id":42}]}`), nil
		}
		return jsonResponse(`{"ok":true,"result":[]}`), nil
	})}

	bot := New(nil, "test-token", "", "", "")
	bot.apiBaseURL = "http://telegram.test"
	bot.httpClient = client
	ctx, cancel := context.WithCancel(context.Background())
	var got []int
	err := bot.Poll(ctx, func(_ context.Context, update Update) error {
		got = append(got, update.UpdateID)
		if len(got) == 2 {
			cancel()
		}
		return nil
	})
	if err == nil {
		t.Fatal("Poll() error = nil after context cancellation")
	}
	if len(got) != 2 || got[0] != 41 || got[1] != 42 {
		t.Fatalf("updates = %v, want [41 42]", got)
	}
}

func TestRegisterCommandsSetsPersonalGroupAndAdminScopes(t *testing.T) {
	t.Parallel()
	var scopes []string
	commandsByScope := make(map[string][]BotCommand)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/bottest-token/setMyCommands" {
			t.Errorf("path = %q", req.URL.Path)
		}
		var payload struct {
			Commands []BotCommand      `json:"commands"`
			Scope    map[string]string `json:"scope"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(payload.Commands) == 0 {
			t.Error("commands are empty")
		}
		scope := payload.Scope["type"]
		scopes = append(scopes, scope)
		commandsByScope[scope] = payload.Commands
		return jsonResponse(`{"ok":true,"result":true}`), nil
	})}

	bot := New(nil, "test-token", "", "", "")
	bot.apiBaseURL = "http://telegram.test"
	bot.httpClient = client
	if err := bot.RegisterCommands(context.Background()); err != nil {
		t.Fatalf("RegisterCommands() error = %v", err)
	}
	if len(scopes) != 3 || scopes[0] != "all_private_chats" || scopes[1] != "all_group_chats" || scopes[2] != "all_chat_administrators" {
		t.Fatalf("scopes = %v", scopes)
	}
	group := commandsByScope["all_group_chats"]
	for _, expected := range []string{"start", "profile", "train", "name", "askcat", "cat", "yard", "event", "fight"} {
		if !hasBotCommand(group, expected) {
			t.Errorf("group commands = %+v, missing /%s", group, expected)
		}
	}
	for _, hidden := range []string{"reset", "autospeak", "humor", "week", "expedition", "collection", "bestiary", "bind", "unbind", "yardsettings", "quiet"} {
		if hasBotCommand(group, hidden) {
			t.Errorf("group commands unexpectedly expose /%s", hidden)
		}
	}
	admins := commandsByScope["all_chat_administrators"]
	if !hasBotCommand(admins, "yardsettings") || !hasBotCommand(admins, "quiet") || !hasBotCommand(admins, "train") {
		t.Fatalf("admin commands are incomplete: %+v", admins)
	}
}

func hasBotCommand(commands []BotCommand, command string) bool {
	for _, candidate := range commands {
		if candidate.Command == command {
			return true
		}
	}
	return false
}
