package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"catforge/internal/app"
	"catforge/internal/domain"
	"catforge/internal/logx"
)

type fakeUpdateRepo struct {
	claimed bool
	err     error
	calls   int
}

func (f *fakeUpdateRepo) Claim(context.Context, int64) (bool, error) {
	f.calls++
	return f.claimed, f.err
}

func webhookRequest(method, secret string) *http.Request {
	req := httptest.NewRequest(method, "/tg/webhook", strings.NewReader(`{"update_id":42}`))
	if secret != "" {
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
	}
	return req
}

func TestWebhookRejectsWrongSecret(t *testing.T) {
	t.Parallel()

	updates := &fakeUpdateRepo{claimed: true}
	router := NewRouter(&app.App{Updates: updates}, nil, "", "", "expected", logx.Nop())
	recorder := httptest.NewRecorder()

	router.WebhookHandler(recorder, webhookRequest(http.MethodPost, "wrong"))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if updates.calls != 0 {
		t.Fatalf("update repository calls = %d, want 0", updates.calls)
	}
}

func TestWebhookSkipsDuplicateUpdate(t *testing.T) {
	t.Parallel()

	updates := &fakeUpdateRepo{claimed: false}
	router := NewRouter(&app.App{Updates: updates}, nil, "", "", "expected", logx.Nop())
	recorder := httptest.NewRecorder()

	router.WebhookHandler(recorder, webhookRequest(http.MethodPost, "expected"))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if updates.calls != 1 {
		t.Fatalf("update repository calls = %d, want 1", updates.calls)
	}
}

func TestWebhookRetriesWhenClaimFails(t *testing.T) {
	t.Parallel()

	updates := &fakeUpdateRepo{err: errors.New("database unavailable")}
	router := NewRouter(&app.App{Updates: updates}, nil, "", "", "expected", logx.Nop())
	recorder := httptest.NewRecorder()

	router.WebhookHandler(recorder, webhookRequest(http.MethodPost, "expected"))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestWebhookRejectsNonPostRequest(t *testing.T) {
	t.Parallel()

	router := NewRouter(&app.App{}, nil, "", "", "", logx.Nop())
	recorder := httptest.NewRecorder()

	router.WebhookHandler(recorder, webhookRequest(http.MethodGet, ""))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}

func TestParseCommandRemovesTelegramBotSuffix(t *testing.T) {
	t.Parallel()
	command, args := parseCommandAndArgs("/askcat@TryToGreat_bot стоит ли работать?")
	if command != "/askcat" || args != "стоит ли работать?" {
		t.Fatalf("command/args = %q/%q", command, args)
	}
}

func TestParseTelegramCommandsForCurrentBot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, text, command, args string
	}{
		{name: "private short command", text: "/askcat вопрос", command: "/askcat", args: "вопрос"},
		{name: "group command", text: "/askcat@TryToGreat_bot вопрос", command: "/askcat", args: "вопрос"},
		{name: "group rename", text: "/name@TryToGreat_bot Пикачпук!", command: "/name", args: "Пикачпук!"},
		{name: "telegram usernames are case insensitive", text: "/yard@trytogreat_BOT", command: "/yard"},
		{name: "another bot", text: "/askcat@AnotherBot вопрос"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			command, args := parseCommandForBot(test.text, "@TryToGreat_bot")
			if command != test.command || args != test.args {
				t.Fatalf("command/args = %q/%q, want %q/%q", command, args, test.command, test.args)
			}
		})
	}
}

func TestPersonalCreationAndSettingsStayPrivateInGroups(t *testing.T) {
	t.Parallel()
	privateCommands := []string{"/start", "/menu", "/home", "/reset", "/autospeak", "/humor", "/skip"}
	for _, command := range privateCommands {
		if !personalCommandIsPrivate(command) {
			t.Errorf("%s must be redirected to the private chat", command)
		}
	}
	publicActions := []string{CBTrainDo, CBMenuYard, CBMenuFight, CBMenuAskCat, CBNoop}
	for _, action := range publicActions {
		if personalCallbackIsPrivate(action) {
			t.Errorf("%s must remain usable in a group", action)
		}
	}
	privateActions := []string{CBNameAsk, CBResetAsk, CBProfileAutoSpeak, CBProfileHumor, CBStarterPrefix + "bengal"}
	for _, action := range privateActions {
		if !personalCallbackIsPrivate(action) {
			t.Errorf("%s must be rejected in a group", action)
		}
	}
}

type renameCats struct {
	cat     *domain.Cat
	newName string
}

func (r *renameCats) GetByUserID(context.Context, int64) (*domain.Cat, error) { return r.cat, nil }
func (*renameCats) Create(context.Context, int64, string, domain.Breed, domain.Trait, int, int, int, int) (*domain.Cat, error) {
	return nil, nil
}
func (*renameCats) DeleteByUserID(context.Context, int64) error { return nil }
func (*renameCats) SaveProgress(context.Context, int64, int64, domain.Cat) (bool, error) {
	return true, nil
}
func (r *renameCats) SetName(_ context.Context, _ int64, name string) (*domain.Cat, error) {
	r.newName = name
	updated := *r.cat
	updated.Name = name
	return &updated, nil
}

type renameUsers struct{ saved app.PendingInput }

func (r *renameUsers) EnsureUser(context.Context, int64) (int64, error) { return 1, nil }
func (r *renameUsers) GetPendingAction(context.Context, int64) (string, error) {
	return "", nil
}
func (r *renameUsers) SetPendingAction(context.Context, int64, string) error { return nil }
func (r *renameUsers) GetPendingInput(context.Context, int64, int64) (*app.PendingInput, error) {
	return nil, nil
}
func (r *renameUsers) SavePendingInput(_ context.Context, input app.PendingInput) error {
	r.saved = input
	return nil
}
func (r *renameUsers) ClearPendingInput(context.Context, int64, int64) error { return nil }

func TestGroupNameCommandRenamesCatDirectly(t *testing.T) {
	t.Parallel()
	var chatIDs []int64
	var texts []string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload struct {
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Errorf("decode Telegram payload: %v", err)
		}
		chatIDs = append(chatIDs, payload.ChatID)
		texts = append(texts, payload.Text)
		return jsonResponse(`{"ok":true,"result":{"message_id":700}}`), nil
	})}

	users := &renameUsers{}
	cats := &renameCats{cat: &domain.Cat{ID: 10, Name: "ХвостоЛап"}}
	sender := NewSender("test", logx.Nop())
	sender.apiBaseURL = "http://telegram.test"
	sender.http = client
	router := NewRouter(&app.App{
		Profile: app.NewProfileService(cats),
		Users:   users,
	}, sender, "", "TryToGreat_bot", "", logx.Nop())
	router.handlePersonalCommand(context.Background(), &tgCtx{
		userID: 1, tgID: 99, chatID: -100, chatType: "supergroup",
	}, "/name", "Пикачпук", nil)

	if len(chatIDs) != 1 || chatIDs[0] != -100 {
		t.Fatalf("messages were sent to chats %v, want [-100]", chatIDs)
	}
	if cats.newName != "Пикачпук" || !strings.Contains(texts[0], "Пикачпук") {
		t.Fatalf("new name = %q, response = %q", cats.newName, texts[0])
	}
	if users.saved != (app.PendingInput{}) {
		t.Fatalf("group rename unexpectedly created pending input: %+v", users.saved)
	}
}
