package telegram

import (
	"bytes"
	"catforge/internal/logx"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Sender struct {
	token string
	http  *http.Client
	log   logx.Logger
}

type tgResp struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code,omitempty"`
	Description string `json:"description,omitempty"`
}

func NewSender(token string, log logx.Logger) *Sender {
	if log == nil {
		log = logx.Nop()
	}
	return &Sender{
		token: token,
		http:  &http.Client{Timeout: 10 * time.Second},
		log:   log,
	}
}

func (s *Sender) Text(ctx context.Context, chatID int64, text string) error {
	return s.Call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
}

func (s *Sender) TextWithKeyboard(ctx context.Context, chatID int64, text string, kb any) error {
	return s.Call(ctx, "sendMessage", map[string]any{
		"chat_id":      chatID,
		"text":         text,
		"reply_markup": kb,
	})
}

func (s *Sender) EditTextWithKeyboard(ctx context.Context, chatID int64, messageID int, text string, kb any) error {
	return s.Call(ctx, "editMessageText", map[string]any{
		"chat_id":      chatID,
		"message_id":   messageID,
		"text":         text,
		"reply_markup": kb,
	})
}

func (s *Sender) AnswerCallback(ctx context.Context, callbackID string) error {
	return s.Call(ctx, "answerCallbackQuery", map[string]any{
		"callback_query_id": callbackID,
	})
}

func (s *Sender) Call(ctx context.Context, method string, payload map[string]any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram api marshal error: method=%s err=%w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://api.telegram.org/bot%s/%s", s.token, method),
		bytes.NewReader(data),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("telegram api http error: %s body=%s", resp.Status, string(b))
	}

	var r tgResp
	if err := json.Unmarshal(b, &r); err != nil {
		return fmt.Errorf("telegram api bad json: %w body=%s", err, string(b))
	}
	if !r.Ok {
		return fmt.Errorf("telegram api error: method=%s code=%d desc=%s", method, r.ErrorCode, r.Description)
	}
	return nil
}

func (s *Sender) PhotoWithKeyboard(ctx context.Context, chatID int64, photoURL, caption string, kb any) error {
	return s.Call(ctx, "sendPhoto", map[string]any{
		"chat_id":      chatID,
		"photo":        photoURL, // URL или file_id
		"caption":      caption,
		"reply_markup": kb,
	})
}

func (s *Sender) EditMediaWithKeyboard(ctx context.Context, chatID int64, messageID int, photoURL, caption string, kb any) error {
	media := map[string]any{
		"type":    "photo",
		"media":   photoURL,
		"caption": caption,
	}
	return s.Call(ctx, "editMessageMedia", map[string]any{
		"chat_id":      chatID,
		"message_id":   messageID,
		"media":        media,
		"reply_markup": kb,
	})
}
