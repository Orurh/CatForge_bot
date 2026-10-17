package telegram

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strings"

	"catforge/internal/assets"
	"catforge/internal/domain"
)

const photoCardPrefix = "catcard:"
const photoCardWidth, photoCardHeight = 960, 1000

func catCardURL(breed domain.Breed, fileID string) string {
	return photoCardPrefix + string(breed) + ":" + base64.RawURLEncoding.EncodeToString([]byte(fileID))
}

// Resolve only Telegram's authenticated getFile result, never a user-supplied URL.
func (s *Sender) photoCard(ctx context.Context, reference string) ([]byte, error) {
	s.photoMu.Lock()
	defer s.photoMu.Unlock()
	for _, entry := range s.photoCards {
		if entry.key == reference {
			return entry.data, nil
		}
	}
	parts := strings.Split(strings.TrimPrefix(reference, photoCardPrefix), ":")
	if len(parts) != 2 {
		return nil, errors.New("invalid photo card")
	}
	frame, err := assets.FS.ReadFile("static/cats/" + parts[0] + "_base.png")
	if err != nil {
		return nil, errors.New("unknown photo card breed")
	}
	id, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid photo ID")
	}
	var file struct {
		Path string `json:"file_path"`
		Size int64  `json:"file_size"`
	}
	if err := s.callResult(ctx, "getFile", map[string]any{"file_id": string(id)}, &file); err != nil {
		return nil, err
	}
	if file.Size > 10<<20 || file.Path == "" || strings.Contains(file.Path, "..") || strings.ContainsAny(file.Path, "\\?#:") || strings.HasPrefix(file.Path, "/") {
		return nil, errors.New("invalid Telegram photo file")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiBaseURL+"/file/bot"+s.token+"/"+strings.ReplaceAll(url.PathEscape(file.Path), "%2F", "/"), nil)
	if err != nil {
		return nil, errors.New("photo download request failed")
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, errors.New("photo download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("photo download status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (10<<20)+1))
	if err != nil || len(data) > 10<<20 {
		return nil, errors.New("photo download exceeds limit or failed")
	}
	card, err := renderPhotoCard(frame, data)
	if err != nil {
		return nil, err
	}
	if len(s.photoCards) >= 16 {
		s.photoCards = s.photoCards[1:]
	}
	s.photoCards = append(s.photoCards, photoCardCache{reference, card})
	return card, nil
}

type photoCardCache struct {
	key  string
	data []byte
}

func renderPhotoCard(frameData, photoData []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(photoData))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 16000000 {
		return nil, errors.New("invalid or oversized photo")
	}
	src, _, err := image.Decode(bytes.NewReader(photoData))
	if err != nil {
		return nil, errors.New("cannot decode photo")
	}
	frame, _, err := image.Decode(bytes.NewReader(frameData))
	if err != nil {
		return nil, err
	}
	out := image.NewRGBA(image.Rect(0, 0, photoCardWidth, photoCardHeight))
	scalePhoto(out, out.Bounds(), frame)
	// Preserve the decorative outer border and original breed nameplate. A dark
	// inner mat covers the old illustration, with the whole photo contained inside.
	window := image.Rect(48, 16, 912, 778)
	draw.Draw(out, window, &image.Uniform{color.RGBA{20, 29, 36, 255}}, image.Point{}, draw.Src)
	// Reuse the four ornamental corners from the template, inside small triangles.
	for y := window.Min.Y; y < window.Max.Y; y++ {
		for x := window.Min.X; x < window.Max.X; x++ {
			dx := min(x-window.Min.X, window.Max.X-1-x)
			dy := min(y-window.Min.Y, window.Max.Y-1-y)
			if dx+dy < 55 {
				out.Set(x, y, frame.At(frame.Bounds().Min.X+x*frame.Bounds().Dx()/photoCardWidth, frame.Bounds().Min.Y+y*frame.Bounds().Dy()/photoCardHeight))
			}
		}
	}
	inner := image.Rect(65, 76, 895, 750)
	w, h := inner.Dx(), inner.Dy()
	if int64(cfg.Width)*int64(h) > int64(cfg.Height)*int64(w) {
		h = max(1, cfg.Height*w/cfg.Width)
	} else {
		w = max(1, cfg.Width*h/cfg.Height)
	}
	target := image.Rect(inner.Min.X+(inner.Dx()-w)/2, inner.Min.Y+(inner.Dy()-h)/2, inner.Min.X+(inner.Dx()-w)/2+w, inner.Min.Y+(inner.Dy()-h)/2+h)
	scalePhoto(out, target, src)
	var b bytes.Buffer
	err = jpeg.Encode(&b, out, &jpeg.Options{Quality: 90})
	return b.Bytes(), err
}

// Bilinear scaling keeps portrait and landscape photos proportional.
func scalePhoto(dst *image.RGBA, target image.Rectangle, src image.Image) {
	b := src.Bounds()
	for y := target.Min.Y; y < target.Max.Y; y++ {
		for x := target.Min.X; x < target.Max.X; x++ {
			fx := float64(x-target.Min.X) * float64(b.Dx()-1) / float64(max(1, target.Dx()-1))
			fy := float64(y-target.Min.Y) * float64(b.Dy()-1) / float64(max(1, target.Dy()-1))
			sx, sy := int(fx), int(fy)
			ax, ay := fx-float64(sx), fy-float64(sy)
			var channels [3]float64
			for j := 0; j < 2; j++ {
				for i := 0; i < 2; i++ {
					wx, wy := 1-ax, 1-ay
					if i == 1 {
						wx = ax
					}
					if j == 1 {
						wy = ay
					}
					r, g, bl, _ := src.At(b.Min.X+min(sx+i, b.Dx()-1), b.Min.Y+min(sy+j, b.Dy()-1)).RGBA()
					channels[0] += float64(r>>8) * wx * wy
					channels[1] += float64(g>>8) * wx * wy
					channels[2] += float64(bl>>8) * wx * wy
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(channels[0] + .5), uint8(channels[1] + .5), uint8(channels[2] + .5), 255})
		}
	}
}
