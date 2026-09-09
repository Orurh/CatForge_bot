package telegram

import (
	"catforge/internal/app"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

type paymentRepoFake struct {
	app.PaymentRepository
	receipts int
	err      error
	approve  bool
	approved int
	creates  int
	invoice  app.SupportInvoice
}

func (p *paymentRepoFake) RecordReceipt(context.Context, app.PaymentReceipt, time.Time) error {
	p.receipts++
	return p.err
}
func (p *paymentRepoFake) ApproveCheckout(context.Context, string, app.PaymentReceipt, time.Time) (bool, error) {
	p.approved++
	return p.approve, p.err
}
func (p *paymentRepoFake) CreateInvoice(_ context.Context, i app.SupportInvoice) (app.SupportInvoice, error) {
	p.creates++
	p.invoice = i
	return i, p.err
}

type failIfClaimed struct{ app.UpdateRepository }

func (failIfClaimed) Claim(context.Context, int64) (bool, error) {
	return false, errors.New("payment used early update claim")
}
func TestPaymentReceiptRetriesBypassUpdateClaim(t *testing.T) {
	repo := &paymentRepoFake{err: errors.New("database offline")}
	a := &app.App{Payments: app.NewPaymentService(repo, app.SystemClock{}, "@support_test"), Updates: failIfClaimed{}}
	r := NewRouter(a, nil, "", "bot", "", nil)
	u := Update{UpdateID: 42, Message: &Message{Chat: &Chat{ID: 123, Type: "private"}, SuccessfulPayment: &TelegramPayment{Currency: "XTR", TotalAmount: 100, InvoicePayload: "support:test", ChargeID: "charge"}}}
	if err := r.HandleUpdate(context.Background(), u); err == nil {
		t.Fatal("DB error swallowed")
	}
	repo.err = nil
	if err := r.HandleUpdate(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if repo.receipts != 2 {
		t.Fatalf("receipt never retried: %d", repo.receipts)
	}
}
func TestStarsCheckoutAndInvoiceContract(t *testing.T) {
	repo := &paymentRepoFake{approve: true}
	a := &app.App{Payments: app.NewPaymentService(repo, app.SystemClock{}, "@support_test"), Updates: failIfClaimed{}}
	var methods []string
	var payloads []map[string]any
	send := NewSender("test", nil)
	send.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, req.URL.Path)
		payloads = append(payloads, payload)
		if strings.HasSuffix(req.URL.Path, "answerPreCheckoutQuery") {
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) > 3*time.Second {
				t.Fatal("checkout answer not bounded")
			}
		}
		return jsonResponse(`{"ok":true,"result":true}`), nil
	})}
	r := NewRouter(a, send, "", "bot", "", nil)
	ctx := context.Background()
	u := Update{PreCheckoutQuery: &PreCheckoutQuery{ID: "q", From: &User{ID: 123}, Currency: "XTR", TotalAmount: 100, InvoicePayload: "support:test"}}
	if err := r.HandleUpdate(ctx, u); err != nil {
		t.Fatal(err)
	}
	if payloads[0]["ok"] != true {
		t.Fatal("valid checkout rejected")
	}
	repo.err = errors.New("database offline")
	if err := r.HandleUpdate(ctx, u); err != nil {
		t.Fatal(err)
	}
	if payloads[1]["ok"] != false || payloads[1]["error_message"] == nil {
		t.Fatal("DB failure must reject checkout")
	}
	repo.err = nil
	u.PreCheckoutQuery.Currency = "USD"
	r.HandleUpdate(ctx, u)
	if repo.approved != 2 || payloads[2]["ok"] != false {
		t.Fatal("foreign currency accepted")
	}
	c := &cbCtx{tgCtx: tgCtx{chatType: "private", chatID: 123, tgID: 123, userID: 1}, data: "support:choose:100"}
	r.supportCallback(ctx, c, "choose")
	if repo.creates != 0 || !strings.Contains(payloads[3]["text"].(string), "Условия") {
		t.Fatal("invoice before terms confirmation")
	}
	c.data = "support:pay:" + app.SupportTermsVersion + ":100"
	r.supportCallback(ctx, c, "accept")
	last := payloads[len(payloads)-1]
	if !strings.HasSuffix(methods[len(methods)-1], "sendInvoice") || last["currency"] != "XTR" || last["provider_token"] != "" || last["start_parameter"] == "" {
		t.Fatalf("bad Stars invoice: %v", last)
	}
	prices := last["prices"].([]any)
	if len(prices) != 1 || prices[0].(map[string]any)["amount"] != float64(100) {
		t.Fatal("wrong Stars amount")
	}
	if repo.creates != 1 {
		t.Fatal("invoice missing")
	}
	c.chatType = "supergroup"
	r.supportCallback(ctx, c, "group")
	if repo.creates != 1 {
		t.Fatal("invoice in group")
	}
}

type supportUsersFake struct {
	app.UserRepository
	pending *app.PendingInput
}

func (p *supportUsersFake) EnsureUser(context.Context, int64) (int64, error) { return 1, nil }
func (p *supportUsersFake) GetPendingInput(_ context.Context, userID, chatID int64) (*app.PendingInput, error) {
	if p.pending != nil && p.pending.UserID == userID && p.pending.ChatID == chatID {
		return p.pending, nil
	}
	return nil, nil
}
func (p *supportUsersFake) SavePendingInput(_ context.Context, input app.PendingInput) error {
	p.pending = &input
	return nil
}
func (p *supportUsersFake) ClearPendingInput(context.Context, int64, int64) error {
	p.pending = nil
	return nil
}

func TestCustomSupportAmountFlow(t *testing.T) {
	repo := &paymentRepoFake{approve: true}
	users := &supportUsersFake{}
	a := &app.App{Users: users, Clock: app.SystemClock{}, Payments: app.NewPaymentService(repo, app.SystemClock{}, "@support_test")}
	var last map[string]any
	var method string
	send := NewSender("test", nil)
	send.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		last = nil
		if err := json.NewDecoder(req.Body).Decode(&last); err != nil {
			t.Fatal(err)
		}
		method = req.URL.Path
		return jsonResponse(`{"ok":true,"result":{"message_id":700}}`), nil
	})}
	r := NewRouter(a, send, "", "bot", "", nil)
	ctx := context.Background()
	c := &cbCtx{tgCtx: tgCtx{chatType: "private", chatID: 123, tgID: 123, userID: 1, now: time.Now()}, data: "support:custom"}
	r.supportCallback(ctx, c, "custom")
	if users.pending == nil || users.pending.Kind != app.PendingAwaitSupportAmount || last["reply_markup"].(map[string]any)["force_reply"] != true {
		t.Fatal("custom amount prompt not saved")
	}
	m := &Message{Chat: &Chat{ID: 123, Type: "private"}, From: &User{ID: 123}, Text: "137", ReplyToMessage: &Message{MessageID: 699}}
	if r.tryConsumeSupportAmount(ctx, &c.tgCtx, m) {
		t.Fatal("unrelated reply consumed")
	}
	m.ReplyToMessage.MessageID = 700
	users.pending.ExpiresAt = c.now.Add(-time.Second)
	if r.tryConsumeSupportAmount(ctx, &c.tgCtx, m) {
		t.Fatal("expired input consumed")
	}
	users.pending.ExpiresAt = c.now.Add(time.Minute)
	for _, text := range []string{"0", "-5", "1.5", "1,5", "+10", "9999999999999999999999999", "abc", ""} {
		m.Text = text
		r.onMessage(ctx, m)
		if users.pending == nil || repo.creates != 0 || !strings.Contains(last["text"].(string), "целым числом") {
			t.Fatalf("invalid amount %q advanced payment", text)
		}
	}
	m.Text = "137"
	r.onMessage(ctx, m)
	if users.pending != nil || repo.creates != 0 {
		t.Fatal("amount input must only show terms, without issuing an invoice")
	}
	terms := last["text"].(string)
	if !strings.Contains(terms, "⭐ 137\n\n") || !strings.Contains(terms, "@support_test") || strings.Contains(terms, `\`) {
		t.Fatalf("incorrect terms formatting: %q", terms)
	}
	markup := last["reply_markup"].(map[string]any)["inline_keyboard"].([]any)
	callback := markup[0].([]any)[0].(map[string]any)["callback_data"].(string)
	if !strings.Contains(callback, "support:pay:"+app.SupportTermsVersion+":137") || len(callback) > 64 {
		t.Fatal("invalid terms confirmation button", callback)
	}
	c.data = "support:pay:" + app.SupportTermsVersion + ":137"
	r.supportCallback(ctx, c, "confirmed")
	if !strings.HasSuffix(method, "sendInvoice") || repo.creates != 1 || repo.invoice.Stars != 137 || repo.invoice.TermsVersion != app.SupportTermsVersion {
		t.Fatal("confirmed custom amount not invoiced")
	}
	prices := last["prices"].([]any)
	if prices[0].(map[string]any)["amount"] != float64(137) {
		t.Fatal("invoice changed custom amount")
	}
	// Old confirmations must not silently accept a newer terms version.
	for _, data := range []string{"support:pay:100", "support:pay:2026-09-07-v1:100"} {
		c.data = data
		r.supportCallback(ctx, c, "old")
		if repo.creates != 1 || !strings.Contains(last["text"].(string), app.SupportTermsVersion) {
			t.Fatal("old terms accepted")
		}
	}
	// Direct commands also stop at terms; group commands never create invoices.
	r.handlePersonalCommand(ctx, &c.tgCtx, "/support", "150", m)
	if !strings.Contains(last["text"].(string), "⭐ 150") || repo.creates != 1 {
		t.Fatal("direct custom amount command failed")
	}
	c.chatType = "supergroup"
	r.handlePersonalCommand(ctx, &c.tgCtx, "/support", "150", m)
	if repo.creates != 1 || strings.Contains(last["text"].(string), "Условия поддержки") {
		t.Fatal("support amount accepted in a group")
	}
	for _, stars := range []int{1, 137, 150, 10000, app.MaxSupportStars} {
		if !app.ValidSupportAmount(stars) {
			t.Fatal("custom amount rejected", stars)
		}
		button := PersonalCallback(9223372036854775807, "support:pay:"+app.SupportTermsVersion+":"+strconv.Itoa(stars))
		if len(button) > 64 {
			t.Fatal("callback exceeds Telegram limit", button)
		}
	}
	for _, stars := range []int{-1, 0, app.MaxSupportStars + 1} {
		if _, err := a.Payments.CreateInvoice(ctx, 1, 123, stars, "invalid"); err == nil {
			t.Fatal("invalid amount reached invoice repository", stars)
		}
		if ok, _ := a.Payments.ApproveCheckout(ctx, "invalid", app.PaymentReceipt{Currency: "XTR", Stars: stars}); ok {
			t.Fatal("invalid amount approved", stars)
		}
	}
	a.Payments.Contact = ""
	c.chatType, c.data = "private", "support:pay:"+app.SupportTermsVersion+":137"
	r.supportCallback(ctx, c, "disabled")
	if repo.creates != 1 {
		t.Fatal("disabled payments created invoice")
	}
}

func TestExpiredCheckoutDoesNotBlockFollowingReceipts(t *testing.T) {
	repo := &paymentRepoFake{approve: true}
	a := &app.App{Payments: app.NewPaymentService(repo, app.SystemClock{}, "@support_test")}
	send := NewSender("test", nil)
	send.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"ok":false,"error_code":400,"description":"query is too old"}`), nil
	})}
	r := NewRouter(a, send, "", "bot", "", nil)
	if err := r.HandleUpdate(context.Background(), Update{PreCheckoutQuery: &PreCheckoutQuery{ID: "expired", From: &User{ID: 123}, Currency: "XTR", TotalAmount: 100}}); err != nil {
		t.Fatal("poison checkout blocks polling", err)
	}
}
func TestRefundHistoryMatchesOutgoingRecipient(t *testing.T) {
	send := NewSender("test", nil)
	send.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{"ok":true,"result":{"transactions":[{"id":"charge","source":{"type":"user","user":{"id":123}}},{"id":"charge","receiver":{"type":"user","user":{"id":456}}}]}}`), nil
	})}
	if ok, err := send.VerifyStarRefund(context.Background(), 123, "charge"); err != nil || ok {
		t.Fatal("incoming payment mistaken for refund")
	}
	if ok, err := send.VerifyStarRefund(context.Background(), 456, "charge"); err != nil || !ok {
		t.Fatal("refund recipient not recognized")
	}
}
