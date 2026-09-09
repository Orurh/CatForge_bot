package main

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
)

func TestReadiness(t *testing.T) {
	for _, fail := range []int{-1, 0, 1} {
		calls := 0
		check := func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Error("missing deadline")
			}
			index := calls
			calls++
			if index == fail {
				return errors.New("secret connection details")
			}
			return nil
		}
		w := httptest.NewRecorder()
		readinessHandler(check, check)(w, httptest.NewRequest("GET", "/readyz", nil))
		want := 200
		if fail >= 0 {
			want = 503
		}
		if w.Code != want {
			t.Fatalf("failure %d: status %d", fail, w.Code)
		}
		if fail >= 0 && w.Body.String() != "not ready\n" {
			t.Fatal("leaked dependency error")
		}
	}
}
