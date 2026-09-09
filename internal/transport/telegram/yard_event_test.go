package telegram

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"catforge/internal/app"
	"catforge/internal/domain"
)

func TestFormatYardEventHidesRoleCounts(t *testing.T) {
	t.Parallel()
	now := time.Unix(1000, 0)
	status := app.YardEventStatus{
		Event: &domain.YardEvent{ID: 17, Type: domain.YardEventFishTruck, Seed: 9, ResolvesAt: now.Add(2*time.Hour + 13*time.Minute)},
		Counts: map[domain.YardEventChoiceID]int{
			domain.YardChoiceSteal: 2, domain.YardChoiceDistract: 1, domain.YardChoiceScout: 4,
		},
	}

	text := formatYardEvent(status, now)
	if !strings.Contains(text, "В дело вписались: 7") || !strings.Contains(text, "2 ч 13 мин") {
		t.Fatalf("event card misses total or deadline: %q", text)
	}
	for _, leaked := range []string{"украсть 2", "отвлечь 1", "разведать 4"} {
		if strings.Contains(strings.ToLower(text), leaked) {
			t.Fatalf("event card leaked a role count %q: %q", leaked, text)
		}
	}

	keyboard, err := json.Marshal(YardEventKeyboard(status.Event.ID, status.Event.Type))
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{"· 2", "· 1", "· 4"} {
		if strings.Contains(string(keyboard), leaked) {
			t.Fatalf("event keyboard leaked a role count %q: %s", leaked, keyboard)
		}
	}
}

func TestFormatYardEventResultShowsSharedRewardsAndNames(t *testing.T) {
	t.Parallel()
	payload := app.YardEventResolvedPayload{
		EventID: 17, EventType: string(domain.YardEventFishTruck),
		Result: domain.YardEventResult{
			OutcomeTier: domain.YardOutcomeSuccess, TeamScore: 31, TargetScore: 25, YardScore: 14, XPGain: 30, StrategyBonus: 6, SecretFound: true,
			RelationshipEffects: []domain.YardRelationshipEffect{{FriendshipDelta: 1, RivalryDelta: 1, RespectDelta: 1}},
		},
		Participants: []app.YardEventParticipantResultPayload{
			{CatID: 41, CatName: "ХвостоЛап", Breed: domain.BreedSiamese, Level: 3, Choice: domain.YardChoiceScout, Contribution: 18, MVP: true},
			{CatID: 42, CatName: "Мурчалкин", Breed: domain.BreedBengal, Level: 2, Choice: domain.YardChoiceSteal, Contribution: 13},
		},
	}

	text := formatYardEventResult(payload)
	for _, wanted := range []string{"ХвостоЛап", "Мурчалкин", "Вклад: 18", "Очки Двора", "+14", "+30 XP", "MVP", "не валюты"} {
		if !strings.Contains(text, wanted) {
			t.Fatalf("event result misses %q: %q", wanted, text)
		}
	}
	if strings.Contains(text, "кот №") {
		t.Fatalf("event result exposed a database id instead of a cat name: %q", text)
	}
}

func TestYardEventTemplatesUseDistinctRolesAndRewards(t *testing.T) {
	t.Parallel()
	now := time.Unix(1000, 0)
	tests := []struct {
		eventType  domain.YardEventType
		cardWant   string
		buttonWant string
		resultWant string
	}{
		{domain.YardEventFishTruck, "Рыбовоз", "Украсть", "Рыбовоз"},
		{domain.YardEventBigDog, "Огромный пёс", "Драться", "Пёс"},
		{domain.YardEventBigBox, "Большая коробка", "Залезть", "Коробка"},
	}
	for _, test := range tests {
		test := test
		t.Run(string(test.eventType), func(t *testing.T) {
			t.Parallel()
			status := app.YardEventStatus{Event: &domain.YardEvent{ID: 17, Type: test.eventType, Seed: 9, ResolvesAt: now.Add(time.Hour)}, Counts: map[domain.YardEventChoiceID]int{}}
			card := formatYardEvent(status, now)
			keyboardJSON, err := json.Marshal(YardEventKeyboard(17, test.eventType))
			if err != nil {
				t.Fatal(err)
			}
			result := formatYardEventResult(app.YardEventResolvedPayload{
				EventID: 17, EventType: string(test.eventType),
				Result: domain.YardEventResult{OutcomeTier: domain.YardOutcomeSuccess, TeamScore: 20, TargetScore: 18, YardScore: 7, XPGain: 30},
			})
			if !strings.Contains(card, test.cardWant) || !strings.Contains(string(keyboardJSON), test.buttonWant) || !strings.Contains(result, test.resultWant) {
				t.Fatalf("type %s rendered card=%q keyboard=%s result=%q", test.eventType, card, keyboardJSON, result)
			}
		})
	}
}
