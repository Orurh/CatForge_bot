package app

// import (
// 	"context"
// 	"testing"
// 	"time"

// 	"catforge/internal/domain"
// )

// type fakeClock struct{ t time.Time }
// func (f fakeClock) Now() time.Time { return f.t }

// type fakeLog struct {
// 	calls int
// 	last  TrainingEvent
// }
// func (l *fakeLog) Training(_ context.Context, e TrainingEvent) error {
// 	l.calls++
// 	l.last = e
// 	return nil
// }

// type stubCats struct {
// 	cat *domain.Cat
// 	res domain.TrainResult
// 	err error
// }
// func (s stubCats) GetByUserID(context.Context, int64) (*domain.Cat, error) { return nil, nil }
// func (s stubCats) Create(context.Context, int64, string, domain.Breed, domain.Trait, int, int, int, int) (*domain.Cat, error) {
// 	return nil, nil
// }
// func (s stubCats) DeleteByUserID(context.Context, int64) error { return nil }
// func (s stubCats) SetName(context.Context, int64, string) (*domain.Cat, error) { return nil, nil }
// func (s stubCats) Train(ctx context.Context, userID int64, now time.Time) (*domain.Cat, domain.TrainResult, error) {
// 	return s.cat, s.res, s.err
// }

// func TestTrainingService_PublishesLogInGroup(t *testing.T) {
// 	t.Parallel()

// 	clk := fakeClock{t: time.Unix(123, 0)}
// 	cats := stubCats{
// 		cat: &domain.Cat{Name: "Тест", Breed: domain.BreedBengal, Level: 2, Energy: 80, EnergyUpdatedAt: clk.t},
// 		res: domain.TrainResult{Outcome: domain.TrainingOK, XPGain: 10, EnergyCost: 20, EffPercent: 100},
// 	}
// 	log := &fakeLog{}

// 	svc := NewTrainingService(cats, clk, log)
// 	_, _, err := svc.Train(context.Background(), 1, 777, -100, "group")
// 	if err != nil {
// 		t.Fatalf("unexpected err: %v", err)
// 	}
// 	if log.calls != 1 {
// 		t.Fatalf("log.calls=%d want 1", log.calls)
// 	}
// 	if log.last.CatName != "Тест" || log.last.Breed != domain.BreedBengal {
// 		t.Fatalf("bad event payload: %v", log.last)
// 	}
// }

// func TestTrainingService_DoesNotPublishInPrivate(t *testing.T) {
// 	t.Parallel()

// 	clk := fakeClock{t: time.Unix(123, 0)}
// 	cats := stubCats{
// 		cat: &domain.Cat{Name: "Тест", Level: 1, Energy: 50, EnergyUpdatedAt: clk.t},
// 		res: domain.TrainResult{Outcome: domain.TrainingOK, XPGain: 8, EnergyCost: 18, EffPercent: 100},
// 	}
// 	log := &fakeLog{}

// 	svc := NewTrainingService(cats, clk, log)
// 	_, _, err := svc.Train(context.Background(), 1, 777, 10, "private")
// 	if err != nil {
// 		t.Fatalf("unexpected err: %v", err)
// 	}
// 	if log.calls != 0 {
// 		t.Fatalf("log.calls=%d want 0", log.calls)
// 	}
// }
