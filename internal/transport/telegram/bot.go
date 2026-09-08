package telegram

import (
	"bytes"
	"catforge/internal/observability"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"catforge/internal/app"
)

type Bot struct {
	app        *app.App
	token      string
	webhookURL string
	secret     string
	apiBaseURL string
	httpClient *http.Client
}

func New(app *app.App, token, publicBaseURL, webhookPath, secret string) *Bot {
	return &Bot{
		app:        app,
		token:      token,
		webhookURL: publicBaseURL + webhookPath,
		secret:     secret,
		apiBaseURL: "https://api.telegram.org",
		httpClient: newTelegramHTTPClient(40 * time.Second),
	}
}

func (b *Bot) RegisterWebhook(ctx context.Context) error {
	reqBody := map[string]any{"url": b.webhookURL, "allowed_updates": []string{"message", "callback_query", "pre_checkout_query"}}
	if b.secret != "" {
		reqBody["secret_token"] = b.secret
	}
	data, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		b.methodURL("setWebhook"),
		bytes.NewReader(data),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return telegramRequestError("setWebhook", err)
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

// DeleteWebhook switches Telegram away from webhook delivery before long polling.
// Keeping pending updates lets the polling worker process messages sent during restart.
func (b *Bot) DeleteWebhook(ctx context.Context, dropPendingUpdates bool) error {
	return b.call(ctx, "deleteWebhook", map[string]any{"drop_pending_updates": dropPendingUpdates}, nil)
}

type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

func (b *Bot) RegisterCommands(ctx context.Context) error {
	privateCommands := []BotCommand{
		command("start"), command("profile"), command("train"), command("name"),
		command("reset"), command("askcat"), command("cat"), command("autospeak"),
		command("humor"), command("support"),
	}
	groupCommands := []BotCommand{
		command("start"), command("profile"), command("train"), command("name"), command("askcat"), command("cat"),
		command("yard"), command("event"), command("fight"), command("week"),
	}
	adminCommands := append(append([]BotCommand{}, groupCommands...), command("yardsettings"), command("quiet"))
	if err := b.setCommands(ctx, "all_private_chats", privateCommands); err != nil {
		return err
	}
	if err := b.setCommands(ctx, "all_group_chats", groupCommands); err != nil {
		return err
	}
	return b.setCommands(ctx, "all_chat_administrators", adminCommands)
}

func command(name string) BotCommand {
	return BotCommand{Command: name, Description: contentText("command." + name)}
}

func (b *Bot) setCommands(ctx context.Context, scopeType string, commands []BotCommand) error {
	return b.call(ctx, "setMyCommands", map[string]any{
		"commands": commands,
		"scope":    map[string]string{"type": scopeType},
	}, nil)
}

// Poll receives updates until the context is cancelled or Telegram/the handler
// returns an error. The caller may restart it with a backoff after transient errors.
func (b *Bot) Poll(ctx context.Context, handle func(context.Context, Update) error) error {
	offset := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		updates, err := b.getUpdates(ctx, offset)
		if err != nil {
			return err
		}
		// Checkout deadlines take priority over slower game/AI handlers in a batch.
		for _, update := range updates {
			if update.PreCheckoutQuery != nil {
				if err := handle(ctx, update); err != nil {
					return err
				}
			}
		}
		for _, update := range updates {
			if update.PreCheckoutQuery != nil {
				offset = update.UpdateID + 1
				continue
			}
			if err := handle(ctx, update); err != nil {
				return fmt.Errorf("handle telegram update %d: %w", update.UpdateID, err)
			}
			offset = update.UpdateID + 1
		}
	}
}

func (b *Bot) getUpdates(ctx context.Context, offset int) (updates []Update, resultErr error) {
	started := time.Now()
	defer func() { observability.Observe("telegram", "getUpdates", started, resultErr) }()
	values := url.Values{}
	values.Set("offset", strconv.Itoa(offset))
	values.Set("timeout", "30")
	values.Set("allowed_updates", `["message","callback_query","pre_checkout_query"]`)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.methodURL("getUpdates"), bytes.NewBufferString(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, telegramRequestError("getUpdates", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("getUpdates failed: status=%s body=%s", resp.Status, string(body))
	}
	var result struct {
		tgResp
		Result []Update `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("getUpdates bad json: %w", err)
	}
	if !result.Ok {
		return nil, fmt.Errorf("getUpdates error: code=%d desc=%s", result.ErrorCode, result.Description)
	}
	return result.Result, nil
}

func (b *Bot) call(ctx context.Context, method string, payload map[string]any, result any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%s marshal: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.methodURL(method), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return telegramRequestError(method, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s failed: status=%s body=%s", method, resp.Status, string(body))
	}
	var envelope tgResp
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("%s bad json: %w", method, err)
	}
	if !envelope.Ok {
		return fmt.Errorf("%s error: code=%d desc=%s", method, envelope.ErrorCode, envelope.Description)
	}
	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("%s result: %w", method, err)
		}
	}
	return nil
}

func (b *Bot) methodURL(method string) string {
	return fmt.Sprintf("%s/bot%s/%s", b.apiBaseURL, b.token, method)
}

type Update struct {
	PreCheckoutQuery *PreCheckoutQuery `json:"pre_checkout_query,omitempty"`
	UpdateID         int               `json:"update_id"`
	Message          *Message          `json:"message,omitempty"`
	CallbackQuery    *CallbackQuery    `json:"callback_query,omitempty"`
}

type Message struct {
	SuccessfulPayment *TelegramPayment `json:"successful_payment,omitempty"`
	RefundedPayment   *TelegramPayment `json:"refunded_payment,omitempty"`
	MessageID         int              `json:"message_id"`
	From              *User            `json:"from,omitempty"`
	Chat              *Chat            `json:"chat,omitempty"`
	Text              string           `json:"text,omitempty"`
	ReplyToMessage    *Message         `json:"reply_to_message,omitempty"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    *User    `json:"from,omitempty"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

type Chat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type,omitempty"` // private|group|supergroup|channel
	Title string `json:"title,omitempty"`
}

type User struct {
	ID    int64 `json:"id"`
	IsBot bool  `json:"is_bot,omitempty"`
}
