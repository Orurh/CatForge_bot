package app

import (
	"context"
	"errors"
	"testing"
)

type recordingEventSink struct {
	calls int
	err   error
}

func (s *recordingEventSink) Publish(context.Context, GameEvent) error {
	s.calls++
	return s.err
}

func TestGameEventFanoutPublishesInOrder(t *testing.T) {
	t.Parallel()
	store := &recordingEventSink{}
	presenter := &recordingEventSink{}

	if err := NewGameEventFanout(store, presenter).Publish(context.Background(), GameEvent{}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if store.calls != 1 || presenter.calls != 1 {
		t.Fatalf("calls store/presenter = %d/%d, want 1/1", store.calls, presenter.calls)
	}
}

func TestGameEventFanoutStopsDuplicateBeforePresentation(t *testing.T) {
	t.Parallel()
	store := &recordingEventSink{err: ErrDuplicateGameEvent}
	presenter := &recordingEventSink{}

	if err := NewGameEventFanout(store, presenter).Publish(context.Background(), GameEvent{}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if presenter.calls != 0 {
		t.Fatalf("presenter calls = %d, want 0", presenter.calls)
	}
}

func TestGameEventFanoutStopsOnStoreFailure(t *testing.T) {
	t.Parallel()
	store := &recordingEventSink{err: errors.New("database unavailable")}
	presenter := &recordingEventSink{}

	err := NewGameEventFanout(store, presenter).Publish(context.Background(), GameEvent{})
	if err == nil {
		t.Fatal("Publish() error = nil, want failure")
	}
	if presenter.calls != 0 {
		t.Fatalf("presenter calls = %d, want 0", presenter.calls)
	}
}
