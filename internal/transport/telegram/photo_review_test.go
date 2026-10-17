package telegram

import (
	"catforge/internal/app"
	"catforge/internal/logx"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

type reviewPhotos struct {
	app.CatPhotoRepository
	status       app.CatPhotoStatus
	failure      error
	decisions    int
	ownerLookups int
}

func (p *reviewPhotos) Review(_ context.Context, id, reviewer int64, approve bool) (app.CatPhotoReviewResult, error) {
	if p.failure != nil {
		return app.CatPhotoReviewResult{}, p.failure
	}
	r := app.CatPhotoReviewResult{Request: app.CatPhotoRequest{ID: id, UserID: 1, Status: p.status}}
	if p.status != app.CatPhotoPending {
		return r, app.ErrCatPhotoConflict
	}
	p.decisions++
	p.status = app.CatPhotoApplied
	r.Request.Status = p.status
	r.Applied = true
	return r, nil
}
func (p *reviewPhotos) TelegramUserID(context.Context, int64) (int64, error) {
	p.ownerLookups++
	return 123, nil
}

func TestPhotoReviewRepeatedClickShowsSavedDecisionWithoutResending(t *testing.T) {
	for _, status := range []app.CatPhotoStatus{app.CatPhotoPending, app.CatPhotoAwaitingPayment, app.CatPhotoRejected, app.CatPhotoSuperseded, app.CatPhotoRefunded} {
		t.Run(string(status), func(t *testing.T) {
			repo := &reviewPhotos{status: status}
			s := NewSender("test", logx.Nop())
			edits, notifications := 0, 0
			s.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var p map[string]json.RawMessage
				if err := json.NewDecoder(req.Body).Decode(&p); err != nil {
					t.Fatal(err)
				}
				switch {
				case strings.HasSuffix(req.URL.Path, "/editMessageCaption"):
					edits++
					var markup struct {
						Buttons [][]map[string]any `json:"inline_keyboard"`
					}
					if err := json.Unmarshal(p["reply_markup"], &markup); err != nil || len(markup.Buttons) != 0 {
						t.Fatal("buttons not cleared")
					}
					var caption string
					_ = json.Unmarshal(p["caption"], &caption)
					if !strings.Contains(caption, photoReviewStatusText(repo.status)) {
						t.Fatal("wrong saved status", caption)
					}
					if edits == 2 {
						return jsonResponse(`{"ok":false,"error_code":400,"description":"Bad Request: message is not modified"}`), nil
					}
				case strings.HasSuffix(req.URL.Path, "/answerCallbackQuery"):
					var text string
					_ = json.Unmarshal(p["text"], &text)
					if text != photoReviewStatusText(repo.status) {
						t.Fatal(text)
					}
				case strings.HasSuffix(req.URL.Path, "/sendMessage"):
					notifications++
				default:
					t.Fatal("unexpected method", req.URL.Path)
				}
				return jsonResponse(`{"ok":true}`), nil
			})}
			r := NewRouter(&app.App{CatPhotos: app.NewCatPhotoService(repo, app.SystemClock{})}, s, "", "", "", logx.Nop())
			r.ConfigureCatPhotos(true, 99, 0)
			cb := &cbCtx{tgCtx: tgCtx{tgID: 99, chatID: 99, chatType: "private"}, msgID: 5, data: "photo:admin:3:yes"}
			r.handleCatPhotoAdminCallback(context.Background(), cb, "first")
			cb.data = "photo:admin:3:no" // Even the opposite stale action cannot undo a decision.
			r.handleCatPhotoAdminCallback(context.Background(), cb, "second")
			want := 0
			if status == app.CatPhotoPending {
				want = 1
			}
			if edits != 2 || notifications != want || repo.ownerLookups != want || repo.decisions != want {
				t.Fatal(edits, notifications, repo.ownerLookups, repo.decisions)
			}
		})
	}
}

func TestPhotoReviewFailureDoesNotRemoveButtonsOrClaimSuccess(t *testing.T) {
	repo := &reviewPhotos{failure: errors.New("database unavailable")}
	s := NewSender("test", logx.Nop())
	s.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(req.URL.Path, "/answerCallbackQuery") {
			t.Fatal("mutated moderator card after database error")
		}
		var p struct {
			Text string `json:"text"`
		}
		_ = json.NewDecoder(req.Body).Decode(&p)
		if !strings.Contains(p.Text, "Попробуй ещё раз") {
			t.Fatal(p.Text)
		}
		return jsonResponse(`{"ok":true}`), nil
	})}
	r := NewRouter(&app.App{CatPhotos: app.NewCatPhotoService(repo, app.SystemClock{})}, s, "", "", "", logx.Nop())
	r.ConfigureCatPhotos(true, 99, 0)
	r.handleCatPhotoAdminCallback(context.Background(), &cbCtx{tgCtx: tgCtx{tgID: 99, chatID: 99, chatType: "private"}, msgID: 5, data: "photo:admin:3:yes"}, "first")
}
