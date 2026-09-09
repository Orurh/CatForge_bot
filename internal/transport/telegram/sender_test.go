package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"catforge/internal/logx"
)

func TestForceReplyReturnsPromptMessageID(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload struct {
			ChatID      int64          `json:"chat_id"`
			Text        string         `json:"text"`
			ReplyMarkup map[string]any `json:"reply_markup"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.ChatID != -100 || payload.Text != "Как зовут кота?" || payload.ReplyMarkup["force_reply"] != true || payload.ReplyMarkup["selective"] != true {
			t.Errorf("unexpected ForceReply payload: %+v", payload)
		}
		return jsonResponse(`{"ok":true,"result":{"message_id":321,"chat":{"id":-100}}}`), nil
	})}
	sender := NewSender("test-token", logx.Nop())
	sender.apiBaseURL = "http://telegram.test"
	sender.http = client
	promptID, err := sender.ForceReply(context.Background(), -100, "Как зовут кота?", "Имя кота")
	if err != nil || promptID != 321 {
		t.Fatalf("ForceReply() = %d, %v", promptID, err)
	}
}

func TestTextReplyResultReturnsMessageID(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload struct {
			ChatID          int64          `json:"chat_id"`
			Text            string         `json:"text"`
			ReplyParameters map[string]any `json:"reply_parameters"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.ChatID != -100 || payload.Text != "Барсик ответил" || payload.ReplyParameters["message_id"] != float64(77) || payload.ReplyParameters["allow_sending_without_reply"] != true {
			t.Errorf("unexpected reply payload: %+v", payload)
		}
		return jsonResponse(`{"ok":true,"result":{"message_id":88,"chat":{"id":-100}}}`), nil
	})}
	sender := NewSender("test-token", logx.Nop())
	sender.apiBaseURL = "http://telegram.test"
	sender.http = client
	messageID, err := sender.TextReplyResult(context.Background(), -100, 77, "Барсик ответил")
	if err != nil || messageID != 88 {
		t.Fatalf("TextReplyResult() = %d, %v", messageID, err)
	}
}

func TestPhotoWithKeyboardUploadsEmbeddedAsset(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/bottest-token/sendPhoto" {
			t.Errorf("path = %q", req.URL.Path)
		}
		if err := req.ParseMultipartForm(2 << 20); err != nil {
			t.Errorf("ParseMultipartForm() error = %v", err)
			return jsonResponse(`{"ok":false}`), nil
		}
		if got := req.FormValue("chat_id"); got != "123" {
			t.Errorf("chat_id = %q", got)
		}
		if got := req.FormValue("caption"); got != "caption" {
			t.Errorf("caption = %q", got)
		}
		file, _, err := req.FormFile("photo")
		if err != nil {
			t.Errorf("photo form file: %v", err)
		} else {
			defer file.Close()
			prefix := make([]byte, 8)
			if _, err := io.ReadFull(file, prefix); err != nil {
				t.Errorf("read photo: %v", err)
			}
			if string(prefix) != "\x89PNG\r\n\x1a\n" {
				t.Errorf("photo is not PNG: %x", prefix)
			}
		}
		return jsonResponse(`{"ok":true,"result":{}}`), nil
	})}

	sender := NewSender("test-token", logx.Nop())
	sender.apiBaseURL = "http://telegram.test"
	sender.http = client
	err := sender.PhotoWithKeyboard(context.Background(), 123, "/static/ui/starter.png", "caption", map[string]any{
		"inline_keyboard": [][]any{},
	})
	if err != nil {
		t.Fatalf("PhotoWithKeyboard() error = %v", err)
	}
}

func TestRemotePhotoStillUsesJSON(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if contentType := req.Header.Get("Content-Type"); contentType != "application/json" {
			t.Errorf("Content-Type = %q", contentType)
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"photo":"https://example.test/cat.png"`) {
			t.Errorf("body = %s", body)
		}
		return jsonResponse(`{"ok":true,"result":{}}`), nil
	})}

	sender := NewSender("test-token", logx.Nop())
	sender.apiBaseURL = "http://telegram.test"
	sender.http = client
	if err := sender.PhotoWithKeyboard(context.Background(), 123, "https://example.test/cat.png", "caption", nil); err != nil {
		t.Fatalf("PhotoWithKeyboard() error = %v", err)
	}
}

func TestIsChatAdmin(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/bottest-token/getChatMember" {
			t.Errorf("path = %q", req.URL.Path)
		}
		return jsonResponse(`{"ok":true,"result":{"status":"administrator"}}`), nil
	})}
	sender := NewSender("test-token", logx.Nop())
	sender.apiBaseURL = "http://telegram.test"
	sender.http = client
	admin, err := sender.IsChatAdmin(context.Background(), -100, 42)
	if err != nil || !admin {
		t.Fatalf("IsChatAdmin() = %v, %v", admin, err)
	}
}

func TestPhotoOmitsNullReplyMarkup(t *testing.T) {
	for _, photo := range []string{"/static/ui/starter.png", "https://example.test/cat.png"} {
		for _, kb := range []any{nil, map[string]any(nil), map[string]any{"inline_keyboard": [][]any{}}} {
			wantMarkup := marshalString(kb) != "null"
			sender := NewSender("test-token", logx.Nop())
			sender.apiBaseURL = "http://telegram.test"
			sender.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var present bool
				var raw string
				if strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/") {
					if err := req.ParseMultipartForm(2 << 20); err != nil {
						t.Fatal(err)
					}
					defer req.MultipartForm.RemoveAll()
					_, present = req.MultipartForm.Value["reply_markup"]
					raw = req.FormValue("reply_markup")
				} else {
					var payload map[string]json.RawMessage
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatal(err)
					}
					v, ok := payload["reply_markup"]
					present = ok
					raw = string(v)
				}
				if present != wantMarkup || raw == "null" {
					t.Errorf("photo=%s keyboard=%v present=%v raw=%q", photo, kb, present, raw)
				}
				if present {
					var object map[string]any
					if err := json.Unmarshal([]byte(raw), &object); err != nil || object == nil {
						t.Errorf("invalid keyboard object: %s", raw)
					}
				}
				return jsonResponse(`{"ok":true,"result":{}}`), nil
			})}
			if err := sender.PhotoWithKeyboard(context.Background(), 123, photo, "caption", kb); err != nil {
				t.Fatal(err)
			}
		}
	}
}
