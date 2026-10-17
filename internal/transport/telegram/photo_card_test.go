package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"catforge/internal/app"
	"catforge/internal/assets"
	"catforge/internal/domain"
	"catforge/internal/logx"
)

func TestPhotoCardKeepsFrameAndFitsPhoto(t *testing.T) {
	for _, breed := range []string{"siamese", "british", "bengal", "maine_coon"} {
		for _, size := range []image.Point{{160, 320}, {320, 160}} {
			t.Run(breed+size.String(), func(t *testing.T) {
				frame, _ := assets.FS.ReadFile("static/cats/" + breed + "_base.png")
				src := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
				for y := 0; y < size.Y; y++ {
					for x := 0; x < size.X; x++ {
						src.Set(x, y, color.RGBA{220, 20, 30, 255})
					}
				}
				var b bytes.Buffer
				_ = png.Encode(&b, src)
				data, err := renderPhotoCard(frame, b.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				card, _, err := image.Decode(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				if card.Bounds() != image.Rect(0, 0, 960, 1000) {
					t.Fatal(card.Bounds())
				}
				r, g, _, _ := card.At(480, 400).RGBA()
				if r < 50000 || g > 10000 {
					t.Fatal("photo missing from center")
				}
				// The label and outer frame must survive the replacement.
				base, _, _ := image.Decode(bytes.NewReader(frame))
				scaled := image.NewRGBA(card.Bounds())
				scalePhoto(scaled, scaled.Bounds(), base)
				for _, p := range []image.Point{{20, 500}, {480, 900}} {
					cr, cg, cb, _ := card.At(p.X, p.Y).RGBA()
					br, bg, bb, _ := scaled.At(p.X, p.Y).RGBA()
					for _, d := range []int{int(cr) - int(br), int(cg) - int(bg), int(cb) - int(bb)} {
						if d > 9000 || d < -9000 {
							t.Fatal("frame/nameplate changed", p)
						}
					}
				}
			})
		}
	}
}

func TestPhotoPreviewDraftRejectsStaleAndReplaced(t *testing.T) {
	now := time.Now()
	d := catPhotoDraft{CatID: 3, FileID: "photo-file", Token: "latest"}
	b, _ := json.Marshal(d)
	p := &app.PendingInput{Kind: catPhotoPendingKind, Payload: string(b), ExpiresAt: now.Add(time.Minute)}
	if _, ok := validPhotoDraft(p, "latest", now); !ok {
		t.Fatal("valid draft rejected")
	}
	if _, ok := validPhotoDraft(p, "old", now); ok {
		t.Fatal("replaced draft accepted")
	}
	if _, ok := validPhotoDraft(p, "latest", now.Add(time.Minute)); ok {
		t.Fatal("expired draft accepted")
	}
	p.Kind = app.PendingAwaitCatName
	if _, ok := validPhotoDraft(p, "latest", now); ok {
		t.Fatal("other pending input accepted")
	}
}

func TestPhotoCardDownloadsOnceAndUploadsForSendAndEdit(t *testing.T) {
	frame, _ := assets.FS.ReadFile("static/cats/siamese_base.png")
	downloads, uploads := 0, 0
	s := NewSender("test-token", logx.Nop())
	s.apiBaseURL = "https://telegram.test"
	s.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/getFile"):
			return jsonResponse(`{"ok":true,"result":{"file_path":"photos/cat.png"}}`), nil
		case strings.HasPrefix(req.URL.Path, "/file/"):
			downloads++
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(frame)), Header: make(http.Header)}, nil
		default:
			uploads++
			if err := req.ParseMultipartForm(4 << 20); err != nil {
				t.Fatal(err)
			}
			defer req.MultipartForm.RemoveAll()
			f, _, err := req.FormFile("photo")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			cfg, _, err := image.DecodeConfig(f)
			if err != nil || cfg.Width != 960 || cfg.Height != 1000 {
				t.Fatal("not a rendered card", cfg, err)
			}
			if strings.HasSuffix(req.URL.Path, "sendPhoto") && req.FormValue("caption") != "profile" {
				t.Fatal("caption lost")
			}
			return jsonResponse(`{"ok":true}`), nil
		}
	})}
	ref := catCardURL(domain.Breed("siamese"), "file-id")
	if err := s.PhotoWithKeyboard(context.Background(), 1, ref, "profile", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.EditMediaWithKeyboard(context.Background(), 1, 2, ref, "profile", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if downloads != 1 || uploads != 2 {
		t.Fatal(downloads, uploads)
	}
}

func TestPhotoCardPreviewArtifact(t *testing.T) {
	path := os.Getenv("CATFORGE_CARD_PREVIEW")
	if path == "" {
		t.Skip("optional visual inspection artifact")
	}
	frame, _ := assets.FS.ReadFile("static/cats/siamese_base.png")
	src, _, _ := image.Decode(bytes.NewReader(frame))
	b := src.Bounds()
	crop := src.(interface {
		SubImage(image.Rectangle) image.Image
	}).SubImage(image.Rect(b.Dx()/5, b.Dy()/10, b.Dx()*4/5, b.Dy()*3/4))
	var p bytes.Buffer
	_ = png.Encode(&p, crop)
	data, err := renderPhotoCard(frame, p.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

type previewUsers struct {
	renameUsers
	pending *app.PendingInput
}

func (u *previewUsers) GetPendingInput(_ context.Context, user, chat int64) (*app.PendingInput, error) {
	if u.pending != nil && u.pending.UserID == user && u.pending.ChatID == chat {
		return u.pending, nil
	}
	return nil, nil
}
func (u *previewUsers) SavePendingInput(_ context.Context, p app.PendingInput) error {
	u.pending = &p
	return nil
}
func (u *previewUsers) ClearPendingInput(context.Context, int64, int64) error {
	u.pending = nil
	return nil
}

type previewPhotos struct {
	app.CatPhotoRepository
	submits int
}

func (p *previewPhotos) Submit(_ context.Context, user, cat int64, file string) (app.CatPhotoRequest, error) {
	p.submits++
	return app.CatPhotoRequest{ID: 1, UserID: user, CatID: cat, FileID: file}, nil
}

func TestPhotoRequiresOwnerConfirmationBeforeSubmission(t *testing.T) {
	now := time.Now()
	users := &previewUsers{pending: &app.PendingInput{UserID: 1, ChatID: 10, Kind: catPhotoPendingKind, PromptMessageID: 2, ExpiresAt: now.Add(time.Minute)}}
	photos := &previewPhotos{}
	cats := &renameCats{cat: &domain.Cat{ID: 3, UserID: 1, Breed: domain.BreedSiamese}}
	s := NewSender("test-token", logx.Nop())
	ref := catCardURL(domain.BreedSiamese, "photo-id")
	s.photoCards = []photoCardCache{{ref, []byte("cached-card")}}
	s.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) { return jsonResponse(`{"ok":true}`), nil })}
	r := NewRouter(&app.App{Users: users, Profile: app.NewProfileService(cats), CatPhotos: app.NewCatPhotoService(photos, app.SystemClock{})}, s, "", "", "", logx.Nop())
	r.ConfigureCatPhotos(true, 99, 0)
	ctx := &tgCtx{userID: 1, tgID: 10, chatID: 10, chatType: "private", now: now}
	m := &Message{Photo: []TelegramPhoto{{FileID: "photo-id", Width: 600, Height: 800}}}
	if !r.tryConsumeCatPhoto(context.Background(), ctx, m) {
		t.Fatal("photo not consumed")
	}
	if photos.submits != 0 {
		t.Fatal("submitted without confirmation")
	}
	var d catPhotoDraft
	if err := json.Unmarshal([]byte(users.pending.Payload), &d); err != nil {
		t.Fatal(err)
	}
	cb := &cbCtx{tgCtx: *ctx, data: "photo:preview:" + d.Token + ":send"}
	other := *cb
	other.userID = 2
	other.tgID = 20
	other.chatID = 20
	r.handleCatPhotoPreviewCallback(context.Background(), &other, "other")
	if photos.submits != 0 {
		t.Fatal("another user confirmed photo")
	}
	// Do not send a reviewer notification from this unit test.
	r.catPhotoReviewerID = 0
	r.handleCatPhotoPreviewCallback(context.Background(), cb, "confirm")
	r.handleCatPhotoPreviewCallback(context.Background(), cb, "duplicate")
	if photos.submits != 1 || users.pending != nil {
		t.Fatal("confirmation not idempotent", photos.submits)
	}
}
