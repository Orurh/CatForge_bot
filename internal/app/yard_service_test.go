package app

import (
	"context"
	"testing"
	"time"

	"catforge/internal/domain"
)

type stubYards struct {
	yard          *domain.Yard
	members       []domain.YardMember
	relationships []domain.CatRelationship
	created       bool
	joined        bool
	err           error
	claimAllowed  *bool
	claimedKinds  *[]domain.AutoMessageKind
}

func (s stubYards) EnsureAndJoin(context.Context, int64, string, int64, int64, time.Time) (*domain.Yard, bool, bool, error) {
	return s.yard, s.created, s.joined, s.err
}
func (s stubYards) GetByTelegramChatID(context.Context, int64) (*domain.Yard, error) {
	return s.yard, s.err
}
func (s stubYards) GetByID(context.Context, int64) (*domain.Yard, error) { return s.yard, s.err }
func (s stubYards) ListMembers(context.Context, int64) ([]domain.YardMember, error) {
	return s.members, s.err
}
func (s stubYards) ListRelationships(context.Context, int64) ([]domain.CatRelationship, error) {
	return s.relationships, s.err
}
func (s stubYards) SaveSettings(_ context.Context, _ int64, settings domain.YardSettings, now time.Time) (*domain.Yard, error) {
	if s.err != nil {
		return nil, s.err
	}
	yard := *s.yard
	yard.HumorMode = settings.HumorMode
	yard.AutoMessagesEnabled = settings.AutoMessagesEnabled
	yard.MaxAutoMessagesDay = settings.MaxAutoMessagesDay
	yard.CatToCatBanter = settings.CatToCatBanter
	yard.FightsEnabled = settings.FightsEnabled
	yard.QuietUntil = settings.QuietUntil
	yard.UpdatedAt = now
	return &yard, nil
}
func (s stubYards) ClaimAutoMessageSlot(_ context.Context, _ int64, _ time.Time, _ int, kind domain.AutoMessageKind) (bool, error) {
	if s.claimedKinds != nil {
		*s.claimedKinds = append(*s.claimedKinds, kind)
	}
	if s.claimAllowed != nil {
		return *s.claimAllowed, s.err
	}
	return true, s.err
}

func TestYardServiceCreatesYardAndMembershipEvents(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	cat := &domain.Cat{ID: 42, UserID: 7, Name: "Барсик", Breed: domain.BreedBengal, Trait: domain.TraitBully}
	yard := &domain.Yard{ID: 9, TelegramChatID: -100, Name: "Друзья"}
	members := []domain.YardMember{{YardID: 9, UserID: 7, CatID: 42, CatName: "Барсик"}}
	events := &fakeEvents{}
	svc := NewYardService(stubYards{yard: yard, members: members, created: true, joined: true}, stubCats{cat: cat}, fakeClock{t: now}, events)

	snapshot, err := svc.Enter(context.Background(), -100, "Друзья", 7)
	if err != nil {
		t.Fatalf("Enter() error = %v", err)
	}
	if !snapshot.Created || !snapshot.Joined || len(snapshot.Members) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if len(events.events) != 2 || events.events[0].Kind != GameEventYardCreated || events.events[1].Kind != GameEventYardMemberJoined {
		t.Fatalf("unexpected events: %+v", events.events)
	}
	for _, event := range events.events {
		if event.YardID != 9 || event.CatID != 42 || !event.OccurredAt.Equal(now) {
			t.Fatalf("event context missing: %+v", event)
		}
	}
}

func TestYardServiceDoesNotRepublishExistingMembership(t *testing.T) {
	t.Parallel()
	yard := &domain.Yard{ID: 9, TelegramChatID: -100, Name: "Друзья"}
	events := &fakeEvents{}
	svc := NewYardService(stubYards{yard: yard}, stubCats{cat: &domain.Cat{ID: 42}}, fakeClock{}, events)

	if _, err := svc.Enter(context.Background(), -100, "Друзья", 7); err != nil {
		t.Fatalf("Enter() error = %v", err)
	}
	if len(events.events) != 0 {
		t.Fatalf("events = %+v, want none", events.events)
	}
}

func TestYardServicePublishesSettingsChange(t *testing.T) {
	t.Parallel()
	now := time.Unix(456, 0)
	yard := &domain.Yard{ID: 9, TelegramChatID: -100, Name: "Друзья"}
	events := &fakeEvents{}
	svc := NewYardService(stubYards{yard: yard}, stubCats{}, fakeClock{t: now}, events)
	settings := domain.YardSettings{
		AutoMessagesEnabled: true,
		MaxAutoMessagesDay:  2,
		CatToCatBanter:      true,
		FightsEnabled:       false,
		HumorMode:           domain.HumorBold,
		QuietUntil:          now.Add(24 * time.Hour),
	}

	updated, err := svc.SaveSettings(context.Background(), -100, settings)
	if err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}
	if updated.FightsEnabled || updated.HumorMode != domain.HumorBold || updated.MaxAutoMessagesDay != 2 {
		t.Fatalf("updated yard = %+v", updated)
	}
	if len(events.events) != 1 {
		t.Fatalf("events = %d, want 1", len(events.events))
	}
	event := events.events[0]
	payload, ok := event.Payload.(YardSettingsChangedPayload)
	if !ok || payload.FightsEnabled || !payload.CatToCatBanter || event.Kind != GameEventYardSettingsChanged || event.YardID != 9 || !event.OccurredAt.Equal(now) {
		t.Fatalf("unexpected event: %+v", event)
	}
}
