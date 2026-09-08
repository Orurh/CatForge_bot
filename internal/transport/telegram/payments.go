package telegram

import (
	"catforge/internal/app"
	"catforge/internal/logx"
	"catforge/internal/observability"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type PreCheckoutQuery struct {
	ID             string `json:"id"`
	From           *User  `json:"from"`
	Currency       string `json:"currency"`
	TotalAmount    int    `json:"total_amount"`
	InvoicePayload string `json:"invoice_payload"`
}
type TelegramPayment struct {
	Currency       string `json:"currency"`
	TotalAmount    int    `json:"total_amount"`
	InvoicePayload string `json:"invoice_payload"`
	ChargeID       string `json:"telegram_payment_charge_id"`
}

func (r *Router) preCheckout(ctx context.Context, q *PreCheckoutQuery) error {
	if q == nil || q.From == nil || q.From.IsBot {
		return nil
	}
	// Keep DB work bounded separately so a slow database still leaves time to reject.
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	ok, err := r.app.Payments.ApproveCheckout(checkCtx, q.ID, app.PaymentReceipt{TelegramID: q.From.ID, Currency: q.Currency, Stars: q.TotalAmount, Payload: q.InvoicePayload})
	cancel()
	if err != nil {
		r.log.Error("payment checkout validation failed", logx.Any("err", err))
	}
	replyCtx, replyCancel := context.WithTimeout(ctx, 3*time.Second)
	defer replyCancel()
	payload := map[string]any{"pre_checkout_query_id": q.ID, "ok": ok && err == nil}
	if !ok || err != nil {
		payload["error_message"] = "Не удалось подтвердить счёт. Открой /support и выбери сумму заново; по вопросам оплаты — /paysupport."
	}
	err = r.send.Call(replyCtx, "answerPreCheckoutQuery", payload)
	if err != nil {
		r.log.Error("payment checkout answer failed; user may retry with a new invoice", logx.Any("err", err))
	}
	// Expired checkout queries cannot succeed on redelivery. Never let one old
	// query block later successful-payment receipts in the polling batch.
	return nil
}
func (r *Router) paymentReceipt(ctx context.Context, m *Message) (resultErr error) {
	started := time.Now()
	defer func() { observability.Observe("payments", "receipt", started, resultErr) }()
	if m.Chat == nil || m.Chat.Type != "private" {
		return errors.New("payment receipt outside private chat")
	}
	p := m.SuccessfulPayment
	refunded := false
	if m.RefundedPayment != nil {
		p = m.RefundedPayment
		refunded = true
	}
	// Private-chat ID identifies the payer even when a refund service message
	// is sent by Telegram/the bot rather than by the original user.
	err := r.app.Payments.RecordReceipt(ctx, app.PaymentReceipt{TelegramID: m.Chat.ID, Currency: p.Currency, Stars: p.TotalAmount, Payload: p.InvoicePayload, ChargeID: p.ChargeID, Refunded: refunded})
	if errors.Is(err, app.ErrPaymentMismatch) {
		r.log.Error("payment receipt saved for review")
		observability.Observe("payments", "review", started, err)
		return nil // Durably quarantined; retrying the same receipt cannot repair it.
	}
	return err
}
func (r *Router) supportText() string {
	return "❤️ Поддержать CatForge\n\nCatForge — независимый проект. Если тебе нравятся коты, можно помочь с серверами, AI и разработкой Звёздами.\n\nПоддержка не даёт преимуществ в игре. Это разовый платёж, без подписки.\n\nВыбери сумму или введи свою — от 1 ⭐:"
}
func (r *Router) termsText() string {
	contact := "Контакт поддержки пока не настроен. Приём новых платежей недоступен."
	if r.app.Payments != nil && r.app.Payments.Contact != "" {
		contact = "Вопросы и запросы возврата: " + r.app.Payments.Contact + ". Укажи дату, сумму и идентификатор платежа из квитанции Telegram. Поддержка Telegram не решает вопросы покупок у этого бота."
	}
	return strings.Join([]string{
		"Условия поддержки CatForge · " + app.SupportTermsVersion,
		"Добровольная разовая поддержка разработки в Telegram Stars (XTR). Сумму выбираешь ты, от 1 Звезды; точная сумма показана перед оплатой. Подписки и автоматических списаний нет.",
		"Платёж не даёт XP, энергию, предметы, дополнительные бои, характеристики или другие игровые преимущества. Публичная отметка о поддержке не выставляется.",
		"Проект находится в бете: функции могут меняться, конкретные будущие возможности или сроки не обещаются.",
		contact,
		"Для учёта оплаты и возвратов сохраняются Telegram ID, сумма, время и идентификатор платежа. Удаление кота не удаляет платёжную запись.",
	}, "\n\n")
}
func (r *Router) showSupport(ctx context.Context, tgc *tgCtx) {
	if !r.app.Payments.Enabled() {
		r.sendText(ctx, tgc.chatID, "Поддержка Звёздами пока недоступна. Условия: /terms. Вопросы оплаты: /paysupport.")
		return
	}
	r.clearSupportInput(ctx, tgc)
	rows := [][]map[string]any{{}, {}}
	for i, n := range []int{50, 100, 250, 500, 1000, 2500} {
		rows[i/3] = append(rows[i/3], map[string]any{"text": fmt.Sprintf("⭐ %d", n), "callback_data": PersonalCallback(tgc.userID, "support:choose:"+strconv.Itoa(n))})
	}
	rows = append(rows, []map[string]any{{"text": "✏️ Своя сумма", "callback_data": PersonalCallback(tgc.userID, "support:custom")}})
	rows = append(rows, []map[string]any{{"text": "Условия", "callback_data": PersonalCallback(tgc.userID, "support:terms")}, {"text": "Помощь с оплатой", "callback_data": PersonalCallback(tgc.userID, "support:help")}})
	r.sendTextKB(ctx, tgc.chatID, r.supportText(), map[string]any{"inline_keyboard": rows})
}

func (r *Router) clearSupportInput(ctx context.Context, tgc *tgCtx) {
	if r.app.Users == nil {
		return
	}
	pending, err := r.app.Users.GetPendingInput(ctx, tgc.userID, tgc.chatID)
	if err == nil && pending != nil && pending.Kind == app.PendingAwaitSupportAmount {
		_ = r.app.Users.ClearPendingInput(ctx, tgc.userID, tgc.chatID)
	}
}

func (r *Router) promptSupportAmount(ctx context.Context, tgc *tgCtx) {
	if tgc.chatType != "private" || !r.app.Payments.Enabled() {
		return
	}
	promptID, err := r.send.ForceReply(ctx, tgc.chatID, "⭐ Сколько Звёзд хочешь отправить?\nОтветь на это сообщение целым числом от 1. Например: 150.\n\nДалее покажу условия и сумму для подтверждения. Отмена: /skip.", "Количество Звёзд")
	if err == nil && r.app.Users != nil {
		err = r.app.Users.SavePendingInput(ctx, app.PendingInput{
			UserID: tgc.userID, ChatID: tgc.chatID, Kind: app.PendingAwaitSupportAmount,
			PromptMessageID: int64(promptID), ExpiresAt: tgc.now.Add(15 * time.Minute),
		})
	}
	if err != nil || r.app.Users == nil {
		r.sendText(ctx, tgc.chatID, "Не удалось открыть ввод суммы. Можно написать /support 150, заменив 150 на свою сумму.")
	}
}

func (r *Router) tryConsumeSupportAmount(ctx context.Context, tgc *tgCtx, message *Message) bool {
	if tgc.chatType != "private" || message == nil || message.ReplyToMessage == nil || r.app.Users == nil {
		return false
	}
	pending, err := r.app.Users.GetPendingInput(ctx, tgc.userID, tgc.chatID)
	if err != nil || pending == nil || pending.Kind != app.PendingAwaitSupportAmount || pending.PromptMessageID != int64(message.ReplyToMessage.MessageID) || !pending.ExpiresAt.After(tgc.now) {
		return false
	}
	r.chooseSupportAmount(ctx, tgc, message.Text)
	return true
}

func (r *Router) chooseSupportAmount(ctx context.Context, tgc *tgCtx, text string) {
	if tgc.chatType != "private" {
		return
	}
	if !r.app.Payments.Enabled() {
		r.showSupport(ctx, tgc)
		return
	}
	text = strings.TrimSpace(text)
	stars, err := strconv.ParseInt(text, 10, 32)
	if err != nil || !app.ValidSupportAmount(int(stars)) || strings.IndexFunc(text, func(c rune) bool { return c < '0' || c > '9' }) >= 0 {
		r.sendText(ctx, tgc.chatID, "Введи сумму целым числом от 1 Звезды, без дробей и других символов. Например: /support 150. Если число слишком большое, выбери меньшую сумму.")
		return
	}
	r.clearSupportInput(ctx, tgc)
	r.showSupportTerms(ctx, tgc, int(stars))
}

func (r *Router) showSupportTerms(ctx context.Context, tgc *tgCtx, stars int) {
	r.sendTextKB(ctx, tgc.chatID, fmt.Sprintf("Выбрано: ⭐ %d\n\n", stars)+r.termsText(), map[string]any{"inline_keyboard": [][]map[string]any{
		{{"text": fmt.Sprintf("Согласен с условиями · ⭐ %d", stars), "callback_data": PersonalCallback(tgc.userID, "support:pay:"+app.SupportTermsVersion+":"+strconv.Itoa(stars))}},
		{{"text": "⬅️ Выбрать другую сумму", "callback_data": PersonalCallback(tgc.userID, "support:open")}},
	}})
}
func (r *Router) showPaySupport(ctx context.Context, tgc *tgCtx) {
	if r.app.Payments == nil || r.app.Payments.Contact == "" {
		r.sendText(ctx, tgc.chatID, "Контакт платёжной поддержки пока не настроен. Приём новых платежей недоступен.")
		return
	}
	r.sendText(ctx, tgc.chatID, "Вопросы оплаты и возврата: "+r.app.Payments.Contact+"\n\nУкажи дату, сумму и идентификатор платежа из квитанции Telegram. Не присылай пароли или коды входа. Поддержка Telegram не решает вопросы покупок у этого бота.")
}
func (r *Router) supportCallback(ctx context.Context, cbc *cbCtx, queryID string) {
	if cbc.chatType != "private" {
		return
	}
	switch cbc.data {
	case "support:open":
		r.showSupport(ctx, &cbc.tgCtx)
		return
	case "support:terms":
		r.sendText(ctx, cbc.chatID, r.termsText())
		return
	case "support:help":
		r.showPaySupport(ctx, &cbc.tgCtx)
		return
	case "support:custom":
		r.promptSupportAmount(ctx, &cbc.tgCtx)
		return
	}
	parts := strings.Split(cbc.data, ":")
	if len(parts) != 3 && len(parts) != 4 {
		return
	}
	stars, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || !app.ValidSupportAmount(stars) || !r.app.Payments.Enabled() {
		return
	}
	if parts[1] == "choose" && len(parts) == 3 {
		r.chooseSupportAmount(ctx, &cbc.tgCtx, parts[2])
		return
	}
	if parts[1] != "pay" {
		return
	}
	// Old cards must show the current terms before recording a new acceptance.
	if len(parts) != 4 || parts[2] != app.SupportTermsVersion {
		r.showSupportTerms(ctx, &cbc.tgCtx, stars)
		return
	}
	invoice, err := r.app.Payments.CreateInvoice(ctx, cbc.userID, cbc.tgID, stars, queryID)
	if err == nil {
		err = r.send.Call(ctx, "sendInvoice", map[string]any{
			"chat_id": cbc.tgID, "title": "Поддержать CatForge", "description": "Разовая поддержка серверов, AI и разработки. Без игровых преимуществ. Условия: /terms. Помощь: /paysupport.",
			"payload": invoice.Payload, "provider_token": "", "currency": "XTR", "start_parameter": "support",
			"prices": []map[string]any{{"label": "Поддержка CatForge", "amount": invoice.Stars}},
		})
	}
	if err != nil {
		r.log.Error("support invoice failed", logx.Any("err", err))
		r.sendText(ctx, cbc.chatID, "Не удалось открыть счёт. Попробуй снова через /support; если Telegram не принимает эту сумму, выбери меньшую. Помощь: /paysupport.")
	}
}
func (s *Sender) RefundStarPayment(ctx context.Context, userID int64, chargeID string) error {
	return s.Call(ctx, "refundStarPayment", map[string]any{"user_id": userID, "telegram_payment_charge_id": chargeID})
}

// Inspect transaction history to recover a refund whose API response was lost.
func (s *Sender) VerifyStarRefund(ctx context.Context, userID int64, chargeID string) (bool, error) {
	for offset := 0; offset < 10000; offset += 100 {
		var page struct {
			Transactions []struct {
				ID       string `json:"id"`
				Receiver *struct {
					Type string `json:"type"`
					User *User  `json:"user"`
				} `json:"receiver"`
			} `json:"transactions"`
		}
		if err := s.callResult(ctx, "getStarTransactions", map[string]any{"offset": offset, "limit": 100}, &page); err != nil {
			return false, err
		}
		for _, entry := range page.Transactions {
			if entry.ID == chargeID && entry.Receiver != nil && entry.Receiver.Type == "user" && entry.Receiver.User != nil && entry.Receiver.User.ID == userID {
				return true, nil
			}
		}
		if len(page.Transactions) < 100 {
			return false, nil
		}
	}
	return false, errors.New("refund history search limit reached")
}
