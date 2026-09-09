package telegram

import (
	"bytes"
	"catforge/internal/observability"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"catforge/internal/assets"
	"catforge/internal/logx"
)

type Sender struct {
	token      string
	http       *http.Client
	apiBaseURL string
	log        logx.Logger
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
		token:      token,
		http:       newTelegramHTTPClient(10 * time.Second),
		apiBaseURL: "https://api.telegram.org",
		log:        log,
	}
}

func (s *Sender) Text(ctx context.Context, chatID int64, text string) error {
	_, err := s.TextResult(ctx, chatID, text)
	return err
}

func (s *Sender) TextResult(ctx context.Context, chatID int64, text string) (int, error) {
	var message Message
	err := s.callResult(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	}, &message)
	return message.MessageID, err
}

func (s *Sender) TextReply(ctx context.Context, chatID int64, replyToMessageID int, text string) error {
	_, err := s.TextReplyResult(ctx, chatID, replyToMessageID, text)
	return err
}

func (s *Sender) TextReplyResult(ctx context.Context, chatID int64, replyToMessageID int, text string) (int, error) {
	var message Message
	err := s.callResult(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
		"reply_parameters": map[string]any{
			"message_id":                  replyToMessageID,
			"allow_sending_without_reply": true,
		},
	}, &message)
	return message.MessageID, err
}

func (s *Sender) TextWithKeyboard(ctx context.Context, chatID int64, text string, kb any) error {
	return s.Call(ctx, "sendMessage", map[string]any{
		"chat_id":      chatID,
		"text":         text,
		"reply_markup": kb,
	})
}

func (s *Sender) ForceReply(ctx context.Context, chatID int64, text, placeholder string) (int, error) {
	var message Message
	err := s.callResult(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]any{
			"force_reply":             true,
			"selective":               true,
			"input_field_placeholder": placeholder,
		},
	}, &message)
	return message.MessageID, err
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

func (s *Sender) AnswerCallbackText(ctx context.Context, callbackID, text string) error {
	return s.Call(ctx, "answerCallbackQuery", map[string]any{
		"callback_query_id": callbackID,
		"text":              text,
	})
}

func (s *Sender) Call(ctx context.Context, method string, payload map[string]any) error {
	return s.callResult(ctx, method, payload, nil)
}

func (s *Sender) IsChatAdmin(ctx context.Context, chatID, telegramUserID int64) (bool, error) {
	var member struct {
		Status string `json:"status"`
	}
	if err := s.callResult(ctx, "getChatMember", map[string]any{
		"chat_id": chatID, "user_id": telegramUserID,
	}, &member); err != nil {
		return false, err
	}
	return member.Status == "creator" || member.Status == "administrator", nil
}

func (s *Sender) callResult(ctx context.Context, method string, payload map[string]any, result any) (resultErr error) {
	started := time.Now()
	defer func() { observability.Observe("telegram", "send", started, resultErr) }()
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram api marshal error: method=%s err=%w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.methodURL(method),
		bytes.NewReader(data),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return telegramRequestError(method, err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("telegram api http error: %s body=%s", resp.Status, string(b))
	}

	var r struct {
		tgResp
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return fmt.Errorf("telegram api bad json: %w body=%s", err, string(b))
	}
	if !r.Ok {
		return fmt.Errorf("telegram api error: method=%s code=%d desc=%s", method, r.ErrorCode, r.Description)
	}
	if result != nil {
		if len(r.Result) == 0 {
			return fmt.Errorf("telegram api missing result: method=%s", method)
		}
		if err := json.Unmarshal(r.Result, result); err != nil {
			return fmt.Errorf("telegram api bad result: method=%s err=%w", method, err)
		}
	}
	return nil
}

func (s *Sender) PhotoWithKeyboard(ctx context.Context, chatID int64, photoURL, caption string, kb any) error {
	// Telegram expects an object when reply_markup is present. In multipart,
	// marshaling nil sends the literal string "null", which rejects the photo.
	markup := marshalString(kb)
	if isEmbeddedPhoto(photoURL) {
		fields := map[string]string{"chat_id": strconv.FormatInt(chatID, 10), "caption": caption}
		if markup != "null" {
			fields["reply_markup"] = markup
		}
		return s.callMultipart(ctx, "sendPhoto", photoURL, fields)
	}
	payload := map[string]any{"chat_id": chatID, "photo": photoURL, "caption": caption}
	if markup != "null" {
		payload["reply_markup"] = kb
	}
	return s.Call(ctx, "sendPhoto", payload)
}

func (s *Sender) EditMediaWithKeyboard(ctx context.Context, chatID int64, messageID int, photoURL, caption string, kb any) error {
	if isEmbeddedPhoto(photoURL) {
		media := map[string]any{"type": "photo", "media": "attach://photo", "caption": caption}
		return s.callMultipart(ctx, "editMessageMedia", photoURL, map[string]string{
			"chat_id":      strconv.FormatInt(chatID, 10),
			"message_id":   strconv.Itoa(messageID),
			"media":        marshalString(media),
			"reply_markup": marshalString(kb),
		})
	}
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

func (s *Sender) callMultipart(ctx context.Context, method, photoURL string, fields map[string]string) (resultErr error) {
	started := time.Now()
	defer func() { observability.Observe("telegram", "send", started, resultErr) }()
	parsed, err := url.Parse(photoURL)
	if err != nil {
		return fmt.Errorf("parse embedded photo path: %w", err)
	}
	assetPath := strings.TrimPrefix(parsed.Path, "/")
	photo, err := assets.FS.ReadFile(assetPath)
	if err != nil {
		return fmt.Errorf("read embedded photo %q: %w", assetPath, err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return fmt.Errorf("telegram multipart field %s: %w", key, err)
		}
	}
	part, err := writer.CreateFormFile("photo", path.Base(assetPath))
	if err != nil {
		return fmt.Errorf("telegram multipart photo: %w", err)
	}
	if _, err := part.Write(photo); err != nil {
		return fmt.Errorf("telegram multipart photo write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("telegram multipart close: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.methodURL(method), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := s.http.Do(req)
	if err != nil {
		return telegramRequestError(method, err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("telegram api http error: %s body=%s", resp.Status, string(responseBody))
	}
	var result tgResp
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return fmt.Errorf("telegram api bad json: %w body=%s", err, string(responseBody))
	}
	if !result.Ok {
		return fmt.Errorf("telegram api error: method=%s code=%d desc=%s", method, result.ErrorCode, result.Description)
	}
	return nil
}

func (s *Sender) methodURL(method string) string {
	return fmt.Sprintf("%s/bot%s/%s", s.apiBaseURL, s.token, method)
}

func isEmbeddedPhoto(photoURL string) bool {
	return strings.HasPrefix(photoURL, "/static/")
}

func marshalString(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
