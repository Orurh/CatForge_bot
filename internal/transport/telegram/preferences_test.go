package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"catforge/internal/app"
	"catforge/internal/domain"
	"catforge/internal/logx"
)

type preferenceRepo struct {
	app.PersonalityRepository
	value                   domain.CatPersonality
	autoWrites, humorWrites int
}

func (p *preferenceRepo) GetByCatID(context.Context, int64) (*domain.CatPersonality, error) {
	v := p.value
	return &v, nil
}
func (p *preferenceRepo) SetAutoSpeak(_ context.Context, _ int64, on bool) error {
	p.autoWrites++
	p.value.AutoSpeakEnabled = on
	return nil
}
func (p *preferenceRepo) SetHumorMode(_ context.Context, _ int64, mode domain.HumorMode) error {
	p.humorWrites++
	p.value.HumorMode = mode
	return nil
}

type preferenceItems struct{ app.ItemRepository }

func (preferenceItems) ListOwned(context.Context, int64) ([]domain.OwnedItem, error) { return nil, nil }

func TestPreferencesEditOriginalCardAndAreIdempotent(t *testing.T) {
	repo := &preferenceRepo{value: domain.CatPersonality{CatID: 10, HumorMode: domain.HumorNormal}}
	sender := NewSender("test", logx.Nop())
	edits, sends := 0, 0
	var keyboard string
	sender.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "editMessageMedia") {
			edits++
			if err := req.ParseMultipartForm(20 << 20); err != nil {
				t.Fatal(err)
			}
			defer req.MultipartForm.RemoveAll()
			if req.FormValue("message_id") != "77" {
				t.Fatal("edited another message")
			}
			keyboard = req.FormValue("reply_markup")
		} else if strings.HasSuffix(req.URL.Path, "sendMessage") || strings.HasSuffix(req.URL.Path, "sendPhoto") {
			sends++
		}
		return jsonResponse(`{"ok":true,"result":{"message_id":77}}`), nil
	})}
	cats := &renameCats{cat: &domain.Cat{ID: 10, Name: "Мур", Breed: domain.BreedBengal, Level: 1}}
	a := &app.App{Users: &renameUsers{}, Profile: app.NewProfileService(cats), Collection: app.NewCollectionService(preferenceItems{}), Personality: app.NewPersonalityService(repo, nil, nil, app.SystemClock{}), Clock: app.SystemClock{}}
	r := NewRouter(a, sender, "", "TryToGreat_bot", "", logx.Nop())
	callback := func(owner int64, chatType, action string) {
		r.onCallback(context.Background(), &CallbackQuery{ID: "cb", From: &User{ID: 100}, Message: &Message{MessageID: 77, Chat: &Chat{ID: 100, Type: chatType}}, Data: PersonalCallback(owner, action)})
	}
	callback(1, "private", preferenceAction("a", 10, "on"))
	callback(1, "private", preferenceAction("a", 10, "on"))
	if repo.autoWrites != 1 || !repo.value.AutoSpeakEnabled || edits != 2 || sends != 0 || !strings.Contains(keyboard, "Реплики: ON") {
		t.Fatalf("writes=%d edits=%d sends=%d keyboard=%s", repo.autoWrites, edits, sends, keyboard)
	}
	callback(1, "private", preferenceAction("h", 10, "bold"))
	if repo.humorWrites != 1 || repo.value.HumorMode != domain.HumorBold || !strings.Contains(keyboard, "Дерзкий юмор: ON") {
		t.Fatal(keyboard)
	}
	callback(2, "private", preferenceAction("a", 10, "off"))
	callback(1, "supergroup", preferenceAction("a", 10, "off"))
	callback(1, "private", preferenceAction("a", 11, "off"))
	if repo.autoWrites != 1 || !repo.value.AutoSpeakEnabled {
		t.Fatal("foreign/group/stale card changed preference")
	}
}

func TestPreferenceButtonsDescribeCurrentStateAndFitCallbackLimit(t *testing.T) {
	p := &domain.CatPersonality{CatID: 9223372036854775807, HumorMode: domain.HumorNormal}
	kb := ProfileKeyboard(9223372036854775807, p)
	raw, _ := json.Marshal(kb)
	if !strings.Contains(string(raw), "Реплики: OFF") || !strings.Contains(string(raw), "Дерзкий юмор: OFF") {
		t.Fatal(string(raw))
	}
	for _, row := range kb["inline_keyboard"].([][]map[string]any) {
		for _, button := range row {
			if len(button["callback_data"].(string)) > 64 {
				t.Fatal("callback exceeds Telegram limit")
			}
		}
	}
}
