package app

import (
	"context"
	"testing"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
)

type bedtimeTestYards struct {
	stubYards
	lastSleep time.Time
	lists     int
	claims    int
}

func (r *bedtimeTestYards) ListBedtimeCandidates(_ context.Context, _, sleepAt, _ time.Time, _ int) ([]domain.Yard, error) {
	r.lists++
	if !r.lastSleep.Before(sleepAt) {
		return nil, nil
	}
	return []domain.Yard{*r.yard}, nil
}

func (r *bedtimeTestYards) ClaimBedtimeFarewell(_ context.Context, _, _ int64, _, sleepAt time.Time) (bool, error) {
	r.claims++
	if !r.lastSleep.Before(sleepAt) {
		return false, nil
	}
	r.lastSleep = sleepAt
	return true, nil
}

type bedtimeTestProvider struct {
	requests []ai.GenerationRequest
	after    func()
}

func (*bedtimeTestProvider) Name() string { return "bedtime-test" }
func (p *bedtimeTestProvider) Generate(_ context.Context, prompt ai.Prompt) (ai.ProviderResult, error) {
	p.requests = append(p.requests, prompt.Request)
	if p.after != nil {
		p.after()
	}
	return ai.ProviderResult{Text: "Счастливо оставаться, ночные гении. А я спать!", Model: "test"}, nil
}

func newBedtimeTestService() (*CatBanterService, *bedtimeTestYards, *bedtimeTestProvider, *fakeEvents) {
	now := time.Date(2026, 9, 11, 23, 55, 0, 0, time.FixedZone("MSK", 3*60*60))
	yards := &bedtimeTestYards{stubYards: stubYards{
		yard: &domain.Yard{ID: 1, TelegramChatID: -100, HumorMode: domain.HumorBold,
			AutoMessagesEnabled: true, CatToCatBanter: true, MaxAutoMessagesDay: 2},
		members: []domain.YardMember{{CatID: 42, CatName: "Барсик", Breed: domain.BreedBengal, Level: 3, LastActiveAt: now}},
	}}
	personalities := banterPersonalities{42: {CatID: 42, Trait: domain.TraitBully, HumorMode: domain.HumorBold, AutoSpeakEnabled: true}}
	provider, events := &bedtimeTestProvider{}, &fakeEvents{}
	service := NewCatBanterService(yards, nil, personalities, ai.NewGateway(provider, nil, nil, time.Second), fakeClock{t: now}, events)
	return service, yards, provider, events
}

func TestBedtimeWindowUsesConfiguredTimezoneAndDoesNotCatchUp(t *testing.T) {
	s := NewCatBanterService(nil, nil, nil, nil, nil, nil)
	for _, tc := range []struct {
		hour, minute, second int
		due                  bool
	}{
		{23, 54, 59, false}, {23, 55, 0, true}, {23, 59, 59, true},
		{0, 0, 0, false}, {9, 59, 0, false}, {10, 0, 0, false},
	} {
		now := time.Date(2026, 9, 11, tc.hour, tc.minute, tc.second, 0, s.location)
		sleepAt, due := s.bedtimeWindow(now.UTC())
		if due != tc.due {
			t.Fatalf("%v: due=%v", now, due)
		}
		if due && (sleepAt.Hour() != 0 || sleepAt.Day() != 12 || !sleepAt.After(now)) {
			t.Fatalf("incorrect next midnight: %v", sleepAt)
		}
	}
	s.SetQuietHours(time.FixedZone("UTC+5", 5*60*60), 22, 8)
	if at, due := s.bedtimeWindow(time.Date(2026, 9, 11, 16, 55, 0, 0, time.UTC)); !due || at.Hour() != 22 {
		t.Fatalf("custom timezone/overnight interval: %v %v", at, due)
	}
	s.SetQuietHours(s.location, 10, 12)
	if _, due := s.bedtimeWindow(time.Date(2026, 9, 11, 9, 55, 0, 0, s.location)); !due {
		t.Fatal("daytime quiet interval ignored")
	}
	s.SetQuietHours(s.location, 0, 0)
	if _, due := s.bedtimeWindow(time.Date(2026, 9, 11, 23, 55, 0, 0, s.location)); due {
		t.Fatal("disabled quiet hours still produced bedtime")
	}
}

func TestBedtimeUsesAIOncePerYardAcrossRestart(t *testing.T) {
	s, yards, provider, events := newBedtimeTestService()
	for attempt := 0; attempt < 3; attempt++ {
		if attempt == 2 {
			s = NewCatBanterService(yards, nil, s.personalities, s.voice, s.clock, events)
		}
		n, err := s.RunDue(context.Background(), 20)
		if err != nil || (attempt == 0 && n != 1) || (attempt > 0 && n != 0) {
			t.Fatalf("attempt %d: %d %v", attempt, n, err)
		}
	}
	if len(provider.requests) != 1 || len(events.events) != 1 || yards.claims != 1 {
		t.Fatalf("requests=%d events=%d claims=%d", len(provider.requests), len(events.events), yards.claims)
	}
	request := provider.requests[0]
	if request.Type != ai.GenerationBedtimeFarewell || request.HumorMode != domain.HumorBold || request.YardID != 1 || request.Cat.ID != 42 {
		t.Fatalf("request: %+v", request)
	}
	payload := events.events[0].Payload.(AutonomousCatMessagePayload)
	if payload.Trigger != "bedtime" || payload.Fallback || payload.Text != "Счастливо оставаться, ночные гении. А я спать!" {
		t.Fatalf("not the AI farewell: %+v", payload)
	}
}

func TestBedtimeRespectsDisabledSpeechAndInactiveCats(t *testing.T) {
	for _, scenario := range []string{"auto_off", "banter_off", "limit_zero", "quiet", "owner_off", "inactive"} {
		t.Run(scenario, func(t *testing.T) {
			s, yards, provider, events := newBedtimeTestService()
			switch scenario {
			case "auto_off":
				yards.yard.AutoMessagesEnabled = false
			case "banter_off":
				yards.yard.CatToCatBanter = false
			case "limit_zero":
				yards.yard.MaxAutoMessagesDay = 0
			case "quiet":
				yards.yard.QuietUntil = s.clock.Now().Add(time.Hour)
			case "owner_off":
				s.personalities.(banterPersonalities)[42].AutoSpeakEnabled = false
			case "inactive":
				yards.members[0].LastActiveAt = s.clock.Now().Add(-8 * 24 * time.Hour)
			}
			if n, err := s.RunDue(context.Background(), 20); n != 0 || err != nil || len(provider.requests) != 0 || len(events.events) != 0 || yards.claims != 0 {
				t.Fatalf("disabled speech: %d %v calls=%d events=%d", n, err, len(provider.requests), len(events.events))
			}
		})
	}
}

func TestBedtimeRechecksNightAndSettingsAfterGeneration(t *testing.T) {
	for _, scenario := range []string{"late_generation", "quiet", "owner_off", "humor_changed"} {
		t.Run(scenario, func(t *testing.T) {
			s, yards, provider, events := newBedtimeTestService()
			provider.after = func() {
				switch scenario {
				case "late_generation":
					s.clock = fakeClock{t: s.clock.Now().Add(5 * time.Minute)}
				case "quiet":
					yards.yard.QuietUntil = s.clock.Now().Add(time.Hour)
				case "owner_off":
					s.personalities.(banterPersonalities)[42].AutoSpeakEnabled = false
				case "humor_changed":
					yards.yard.HumorMode = domain.HumorNormal
				}
			}
			if n, err := s.RunDue(context.Background(), 20); n != 0 || err != nil || len(events.events) != 0 {
				t.Fatalf("late farewell: %d %v events=%d", n, err, len(events.events))
			}
			if len(provider.requests) != 1 {
				t.Fatal("test did not generate a farewell")
			}
		})
	}
}

func TestBedtimeNormalModeAndNoQueriesOutsideWindow(t *testing.T) {
	for _, normalYard := range []bool{false, true} {
		s, _, provider, _ := newBedtimeTestService()
		if normalYard {
			s.yards.(*bedtimeTestYards).yard.HumorMode = domain.HumorNormal
		} else {
			s.personalities.(banterPersonalities)[42].HumorMode = domain.HumorNormal
		}
		if n, err := s.RunDue(context.Background(), 20); n != 1 || err != nil || provider.requests[0].HumorMode != domain.HumorNormal {
			t.Fatalf("normal mode ignored: %d %v", n, err)
		}
	}
	for _, hours := range []int{0, 10, 22} {
		s, yards, provider, _ := newBedtimeTestService()
		s.clock = fakeClock{t: time.Date(2026, 9, 12, hours, 0, 0, 0, s.location)}
		if _, err := s.RunDue(context.Background(), 20); err != nil || yards.lists != 0 || len(provider.requests) != 0 {
			t.Fatalf("outside window: %v lists=%d", err, yards.lists)
		}
	}
}
