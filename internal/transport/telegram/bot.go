package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"catforge/internal/app"
)

type Bot struct {
	app        *app.App
	token      string
	webhookURL string
	httpClient *http.Client
}

func New(app *app.App, token, publicBaseURL, webhookPath string) *Bot {
	return &Bot{
		app:        app,
		token:      token,
		webhookURL: publicBaseURL + webhookPath,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *Bot) RegisterWebhook(ctx context.Context) error {
	reqBody := map[string]any{"url": b.webhookURL}
	data, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", b.token),
		bytes.NewReader(data),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("setWebhook failed: status=%s body=%s", resp.Status, string(body))
	}
	var r tgResp
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("setWebhook bad json: %w body=%s", err, string(body))
	}
	if !r.Ok {
		return fmt.Errorf("setWebhook error: code=%d desc=%s", r.ErrorCode, r.Description)
	}
	return nil
}

type Update struct {
	UpdateID      int            `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      *Chat  `json:"chat,omitempty"`
	Text      string `json:"text,omitempty"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    *User    `json:"from,omitempty"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type,omitempty"` // private|group|supergroup|channel
}

type User struct {
	ID int64 `json:"id"`
}
