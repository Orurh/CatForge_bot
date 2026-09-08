package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"catforge/internal/app"
	"catforge/internal/domain"
	"catforge/internal/logx"
)

type eventlogCatMessageRefs struct {
	ref domain.CatMessageReference
}

func TestCatBanterPublishesOneCompactTelegramMessage(t *testing.T) {
	t.Parallel()
	var sentText string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode Telegram payload: %v", err)
		}
		sentText = payload.Text
		return jsonResponse(`{"ok":true,"result":{"message_id":322,"chat":{"id":-100}}}`), nil
	})}
	sender := NewSender("test-token", logx.Nop())
	sender.apiBaseURL = "http://telegram.test"
	sender.http = client
	sink := NewTelegramGameEventSink(sender, nil, logx.Nop())

	err := sink.Publish(context.Background(), app.GameEvent{
		Kind: app.GameEventCatBanter,
		Payload: app.CatBanterPayload{
			TelegramChatID: -100,
			FirstCatName:   "Барсик", FirstCatBreed: domain.BreedBengal, FirstCatLevel: 4, FirstLine: "не привыкай",
			SecondCatName: "Батон", SecondCatBreed: domain.BreedBritish, SecondCatLevel: 5, SecondLine: "уже поздно",
		},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	for _, expected := range []string{"Барсик: не привыкай", "Батон: уже поздно"} {
		if !strings.Contains(sentText, expected) {
			t.Fatalf("Telegram text %q misses %q", sentText, expected)
		}
	}
	if strings.Count(sentText, "\n\n") != 1 {
		t.Fatalf("Telegram text is not one compact two-line message: %q", sentText)
	}
}

func (r *eventlogCatMessageRefs) Remember(_ context.Context, chatID int64, messageID int, catID int64, expiresAt time.Time) error {
	r.ref = domain.CatMessageReference{
		TelegramChatID: chatID, TelegramMessageID: messageID, CatID: catID, ExpiresAt: expiresAt,
	}
	return nil
}

func (*eventlogCatMessageRefs) Claim(context.Context, int64, int, time.Time) (*domain.CatMessageReference, bool, error) {
	return nil, false, nil
}

func TestAutonomousCatMessageStoresSpeakerReference(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"ok":true,"result":{"message_id":321,"chat":{"id":-100}}}`), nil
	})}
	sender := NewSender("test-token", logx.Nop())
	sender.apiBaseURL = "http://telegram.test"
	sender.http = client
	refs := &eventlogCatMessageRefs{}
	sink := NewTelegramGameEventSink(sender, refs, logx.Nop())
	now := time.Unix(1_000, 0)
	err := sink.Publish(context.Background(), app.GameEvent{
		Kind: app.GameEventAutonomousCatMessage, CatID: 42, OccurredAt: now,
		Payload: app.AutonomousCatMessagePayload{
			TelegramChatID: -100, CatName: "Барсик", Breed: domain.BreedBengal, Level: 4, Text: "всё видел",
		},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if refs.ref.TelegramMessageID != 321 || refs.ref.CatID != 42 || !refs.ref.ExpiresAt.Equal(now.Add(48*time.Hour)) {
		t.Fatalf("stored reference = %+v", refs.ref)
	}
}

func TestTrainingGroupExplainsBothEnergyDeadlines(t *testing.T) {
	now := time.Unix(10000, 0)
	for _, tt := range []struct {
		energy  int
		outcome domain.TrainingOutcome
		crit    bool
		updated time.Time
		want    []string
	}{
		{49, domain.TrainingNotEnoughEnergy, false, now.Add(-7 * time.Minute), []string{"Тренироваться можно с 50", "через 1м", "До полного запаса"}},
		{0, domain.TrainingOK, true, now, []string{"КРИТИЧЕСКАЯ ТРЕНИРОВКА", "+100% базового XP", "6ч 40м", "13ч 20м", "энергия больше не копится"}},
	} {
		var sent string
		sender := NewSender("test-token", logx.Nop())
		sender.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload struct {
				Text string `json:"text"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			sent = payload.Text
			return jsonResponse(`{"ok":true,"result":{"message_id":1}}`), nil
		})}
		sink := NewTelegramGameEventSink(sender, nil, logx.Nop())
		err := sink.Publish(context.Background(), app.GameEvent{Kind: app.GameEventCatTrained, OccurredAt: now, Payload: app.CatTrainedPayload{TargetChatID: -1, CatName: "Дрыстулька", Energy: tt.energy, EnergyUpdatedAt: tt.updated, Result: domain.TrainResult{Outcome: tt.outcome, Crit: tt.crit, EnergyCost: 100, XPGain: 300}}})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range tt.want {
			if !strings.Contains(sent, want) {
				t.Fatalf("missing %q: %s", want, sent)
			}
		}
		if strings.Contains(sent, "x2") {
			t.Fatal("obsolete critical multiplier")
		}
	}
}
