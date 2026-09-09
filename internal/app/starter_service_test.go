package app

import (
	"context"
	"testing"
	"time"

	"catforge/internal/domain"
)

func TestStarterServicePublishesCatCreated(t *testing.T) {
	t.Parallel()
	now := time.Unix(123, 0)
	cat := &domain.Cat{ID: 42, UserID: 7, Name: "Мурчалкин100", Breed: domain.BreedBengal, Trait: "lazy", Level: 1}
	events := &fakeEvents{}
	svc := NewStarterService(stubCats{cat: cat}, fakeClock{t: now}, zeroRNG{}, events)

	got, err := svc.ChooseStarterCat(context.Background(), 7, domain.BreedBengal)
	if err != nil {
		t.Fatalf("ChooseStarterCat() error = %v", err)
	}
	if got != cat || len(events.events) != 1 {
		t.Fatalf("cat/events = %+v/%+v", got, events.events)
	}
	event := events.events[0]
	if event.Kind != GameEventCatCreated || event.CatID != 42 || event.DedupeKey != "user:7:cat:42:created" {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestAppRecordsOnlyDedupeableStartFact(t *testing.T) {
	t.Parallel()
	events := &fakeEvents{}
	a := &App{Clock: fakeClock{t: time.Unix(456, 0)}, Events: events}

	a.RecordUserStarted(context.Background(), 7, 777, "private")

	if len(events.events) != 1 {
		t.Fatalf("events = %d, want 1", len(events.events))
	}
	event := events.events[0]
	if event.Kind != GameEventUserStarted || event.DedupeKey != "user:7:started" {
		t.Fatalf("unexpected event: %+v", event)
	}
}
