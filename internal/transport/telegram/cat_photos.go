package telegram

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"catforge/internal/app"
	"catforge/internal/domain"
	"catforge/internal/transport/telegram/views"
)

const catPhotoPendingKind = "await_cat_photo"

func (r *Router) ConfigureCatPhotos(enabled bool, reviewerID, canaryID int64) {
	r.catPhotoEnabled = enabled && reviewerID > 0
	r.catPhotoReviewerID = reviewerID
	r.catPhotoCanaryID = canaryID
}

// A nonzero canary ID restricts access to one Telegram user in production.
func catPhotoCanaryAllows(canaryID, telegramID int64) bool {
	return canaryID == 0 || telegramID == canaryID
}
func (r *Router) catPhotoAllowedForUser(ctx context.Context, userID int64) bool {
	if !r.catPhotoEnabled || r.app.CatPhotos == nil {
		return false
	}
	if r.catPhotoCanaryID == 0 {
		return true
	}
	telegramID, err := r.app.CatPhotos.TelegramUserID(ctx, userID)
	return err == nil && catPhotoCanaryAllows(r.catPhotoCanaryID, telegramID)
}
func (r *Router) catPhotoURL(ctx context.Context, cat *domain.Cat) string {
	original := views.CatAvatarURL(r.publicBaseURL, cat)
	if !r.catPhotoEnabled || r.app.CatPhotos == nil {
		return original
	}
	fileID, err := r.app.CatPhotos.Current(ctx, cat.UserID, cat.ID)
	if err != nil {
		r.log.Warn("cat photo lookup failed")
		return original
	}
	if fileID != "" {
		return catCardURL(cat.Breed, fileID)
	}
	return original
}
func (r *Router) requestCatPhoto(ctx context.Context, tgc *tgCtx) {
	if tgc.chatType != "private" {
		r.openPrivatePersonalUI(ctx, tgc)
		return
	}
	if !r.catPhotoEnabled || r.app.CatPhotos == nil || !catPhotoCanaryAllows(r.catPhotoCanaryID, tgc.tgID) {
		r.sendText(ctx, tgc.chatID, "Смена фотографии временно недоступна.")
		return
	}
	cat, err := r.app.Profile.GetCat(ctx, tgc.userID)
	if err != nil || cat == nil {
		r.sendText(ctx, tgc.chatID, "Сначала создай кота через /start.")
		return
	}
	open, err := r.app.CatPhotos.OpenForUser(ctx, tgc.userID)
	if err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось проверить заявку. Попробуй позже.")
		return
	}
	if open != nil {
		if open.Status == app.CatPhotoAwaitingPayment {
			r.sendCatPhotoInvoice(ctx, tgc.tgID, open.ID)
			return
		}
		r.sendText(ctx, tgc.chatID, "Твоя фотография уже ожидает проверки. Отменить отправленную заявку пока нельзя.")
		return
	}
	promptMessageID, err := r.send.ForceReply(ctx, tgc.chatID,
		"📷 Пришли фото кота ответом на это сообщение. Сначала покажу предпросмотр карточки 960 × 1000 пикс. Фото вписывается целиком; перед модерацией ты подтвердишь результат.\n\nПервое одобренное изменение бесплатно. Следующие — по 10 ⭐ после модерации.\n\nНе присылай фотографии людей, чужие приватные данные или запрещённые изображения. Модератор увидит фото до публикации.\n\nОтмена: /skip.", "Фото кота")
	if err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось запросить фото.")
		return
	}
	if err = r.app.Users.SavePendingInput(ctx, app.PendingInput{UserID: tgc.userID, ChatID: tgc.chatID, Kind: catPhotoPendingKind, PromptMessageID: int64(promptMessageID), ExpiresAt: tgc.now.Add(15 * time.Minute)}); err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось сохранить запрос. Попробуй /photo позже.")
	}
}
func (r *Router) tryConsumeCatPhoto(ctx context.Context, tgc *tgCtx, m *Message) bool {
	if !r.catPhotoEnabled || r.app.CatPhotos == nil || tgc.chatType != "private" || !catPhotoCanaryAllows(r.catPhotoCanaryID, tgc.tgID) || m == nil || len(m.Photo) == 0 {
		return false
	}
	pending, err := r.app.Users.GetPendingInput(ctx, tgc.userID, tgc.chatID)
	if err != nil || pending == nil || pending.Kind != catPhotoPendingKind {
		return false
	}
	photo := m.Photo[len(m.Photo)-1]
	if !app.ValidCatPhotoFileID(photo.FileID) || (photo.FileSize > 10<<20) || (photo.Width > 10000) || (photo.Height > 10000) {
		r.sendText(ctx, tgc.chatID, "Фото слишком большое или некорректное. Пришли другое изображение.")
		return true
	}
	cat, err := r.app.Profile.GetCat(ctx, tgc.userID)
	if err != nil || cat == nil {
		r.sendText(ctx, tgc.chatID, "Кот не найден.")
		return true
	}
	var nonce [12]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return true
	}
	draft := catPhotoDraft{CatID: cat.ID, FileID: photo.FileID, Token: hex.EncodeToString(nonce[:])}
	payload, _ := json.Marshal(draft)
	pending.Payload = string(payload)
	pending.ExpiresAt = tgc.now.Add(15 * time.Minute)
	if err = r.app.Users.SavePendingInput(ctx, *pending); err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось сохранить предпросмотр. Попробуй ещё раз.")
		return true
	}
	caption := fmt.Sprintf("📷 Предпросмотр\n\nФото: %d × %d пикс.\nКарточка: %d × %d пикс.\nФото целиком, без обрезки и растягивания. Рамка и подпись породы сохраняются.\n\nПервое одобренное изменение бесплатно, последующие — 10 ⭐ после модерации.\n\nОтправить на проверку? Можно прислать другое фото или отменить. Предпросмотр действует 15 минут.", photo.Width, photo.Height, photoCardWidth, photoCardHeight)
	kb := map[string]any{"inline_keyboard": [][]map[string]any{
		{{"text": "✅ На проверку", "callback_data": "photo:preview:" + draft.Token + ":send"}},
		{{"text": "📷 Другое фото", "callback_data": "photo:preview:" + draft.Token + ":change"}, {"text": "Отмена", "callback_data": "photo:preview:" + draft.Token + ":cancel"}},
	}}
	if err = r.send.PhotoWithKeyboard(ctx, tgc.chatID, catCardURL(cat.Breed, photo.FileID), caption, kb); err != nil {
		_ = r.app.Users.ClearPendingInput(ctx, tgc.userID, tgc.chatID)
		r.log.Warn("cat photo preview failed")
		r.sendText(ctx, tgc.chatID, "Не удалось подготовить предпросмотр. Фото не отправлено модератору. Повтори /photo.")
	}
	return true
}

type catPhotoDraft struct {
	CatID  int64
	FileID string
	Token  string
}

func validPhotoDraft(p *app.PendingInput, token string, now time.Time) (catPhotoDraft, bool) {
	var d catPhotoDraft
	if p == nil || p.Kind != catPhotoPendingKind || !p.ExpiresAt.After(now) || json.Unmarshal([]byte(p.Payload), &d) != nil {
		return d, false
	}
	return d, d.Token != "" && d.Token == token && d.CatID > 0 && app.ValidCatPhotoFileID(d.FileID)
}

func (r *Router) handleCatPhotoPreviewCallback(ctx context.Context, cbc *cbCtx, callbackID string) bool {
	if !strings.HasPrefix(cbc.data, "photo:preview:") {
		return false
	}
	if !r.catPhotoEnabled || r.app.CatPhotos == nil || cbc.chatType != "private" || !catPhotoCanaryAllows(r.catPhotoCanaryID, cbc.tgID) {
		_ = r.send.AnswerCallbackText(ctx, callbackID, "Смена фото недоступна.")
		return true
	}
	parts := strings.Split(cbc.data, ":")
	if len(parts) != 4 {
		return true
	}
	pending, err := r.app.Users.GetPendingInput(ctx, cbc.userID, cbc.chatID)
	draft, ok := validPhotoDraft(pending, parts[2], cbc.now)
	if err != nil || !ok {
		_ = r.send.AnswerCallbackText(ctx, callbackID, "Предпросмотр устарел. Повтори /photo.")
		return true
	}
	switch parts[3] {
	case "cancel":
		if err = r.app.Users.ClearPendingInput(ctx, cbc.userID, cbc.chatID); err != nil {
			_ = r.send.AnswerCallbackText(ctx, callbackID, "Не удалось отменить. Повтори.")
			return true
		}
		_ = r.send.AnswerCallbackText(ctx, callbackID, "Отменено.")
		r.sendText(ctx, cbc.chatID, "Смена фото отменена. Аватар остался прежним.")
	case "change":
		_ = r.send.AnswerCallbackText(ctx, callbackID, "Выбери другое фото.")
		r.requestCatPhoto(ctx, &cbc.tgCtx)
	case "send":
		cat, err := r.app.Profile.GetCat(ctx, cbc.userID)
		if err != nil || cat == nil || cat.ID != draft.CatID {
			_ = r.send.AnswerCallbackText(ctx, callbackID, "Кот изменился. Повтори /photo.")
			return true
		}
		req, err := r.app.CatPhotos.Submit(ctx, cbc.userID, cat.ID, draft.FileID)
		if err != nil {
			text := "Не удалось отправить заявку. Попробуй ещё раз."
			if errors.Is(err, app.ErrCatPhotoPending) {
				text = "Заявка уже на проверке."
			}
			_ = r.send.AnswerCallbackText(ctx, callbackID, text)
			return true
		}
		_ = r.app.Users.ClearPendingInput(ctx, cbc.userID, cbc.chatID)
		_ = r.send.AnswerCallbackText(ctx, callbackID, "Отправлено на проверку.")
		r.sendText(ctx, cbc.chatID, "📨 Фото отправлено на проверку. До одобрения останется прежний аватар.")
		r.notifyCatPhotoReviewer(ctx, req)
	}
	return true
}

func catPhotoAdminKeyboard(requestID int64) map[string]any {
	n := strconv.FormatInt(requestID, 10)
	return map[string]any{"inline_keyboard": [][]map[string]any{
		{{"text": "✅ Одобрить", "callback_data": "photo:admin:" + n + ":yes"},
			{"text": "❌ Отклонить", "callback_data": "photo:admin:" + n + ":no"}},
	}}
}
func (r *Router) notifyCatPhotoReviewer(ctx context.Context, req app.CatPhotoRequest) {
	if r.catPhotoReviewerID <= 0 {
		return
	}
	caption := fmt.Sprintf("📷 Заявка #%d\nПользователь: %d\nКот: %d\nФото не опубликовано. Проверь содержимое.", req.ID, req.UserID, req.CatID)
	if err := r.send.PhotoWithKeyboard(ctx, r.catPhotoReviewerID, req.FileID, caption, catPhotoAdminKeyboard(req.ID)); err != nil {
		r.log.Warn("photo review notification failed; use /photoqueue to recover")
	}
}
func (r *Router) photoQueue(ctx context.Context, tgc *tgCtx) {
	if !r.catPhotoEnabled || tgc.chatType != "private" || tgc.tgID != r.catPhotoReviewerID {
		return
	}
	pending, err := r.app.CatPhotos.AwaitingReview(ctx)
	if err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось получить очередь.")
		return
	}
	if len(pending) == 0 {
		r.sendText(ctx, tgc.chatID, "Нет заявок на рассмотрении.")
		return
	}
	for _, req := range pending {
		r.notifyCatPhotoReviewer(ctx, req)
	}
}
func (r *Router) handleCatPhotoAdminCallback(ctx context.Context, cbc *cbCtx, callbackID string) bool {
	if !strings.HasPrefix(cbc.data, "photo:admin:") {
		return false
	}
	if !r.catPhotoEnabled || cbc.chatType != "private" || cbc.tgID != r.catPhotoReviewerID {
		_ = r.send.AnswerCallbackText(ctx, callbackID, "Нет доступа к модерации.")
		return true
	}
	parts := strings.Split(cbc.data, ":")
	if len(parts) != 4 || (parts[3] != "yes" && parts[3] != "no") {
		_ = r.send.AnswerCallbackText(ctx, callbackID, "Неверное действие.")
		return true
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || id <= 0 {
		_ = r.send.AnswerCallbackText(ctx, callbackID, "Неверная заявка.")
		return true
	}
	result, err := r.app.CatPhotos.Review(ctx, id, r.catPhotoReviewerID, parts[3] == "yes")
	if err != nil {
		if errors.Is(err, app.ErrCatPhotoConflict) && photoReviewStatusText(result.Request.Status) != "" {
			// The repository returns the persisted state while holding the row lock.
			// Never repeat player notifications or invoices for a closed request.
			r.showPhotoReviewDecision(ctx, cbc, callbackID, result.Request)
		} else if errors.Is(err, app.ErrCatPhotoNotFound) {
			_ = r.send.AnswerCallbackText(ctx, callbackID, "Заявка не найдена.")
		} else {
			r.log.Warn("cat photo review failed")
			_ = r.send.AnswerCallbackText(ctx, callbackID, "Не удалось проверить решение. Попробуй ещё раз.")
		}
		return true
	}
	r.showPhotoReviewDecision(ctx, cbc, callbackID, result.Request)
	userID, err := r.app.CatPhotos.TelegramUserID(ctx, result.Request.UserID)
	if err != nil {
		r.log.Warn("photo review owner lookup failed")
		return true
	}
	switch {
	case result.Applied:
		r.sendText(ctx, userID, "✅ Фото кота одобрено и установлено. Первая смена была бесплатной.")
	case result.NeedsPayment:
		r.sendText(ctx, userID, "✅ Фото одобрено. Для установки повторной фотографии потребуется 10 ⭐.")
		r.sendCatPhotoInvoice(ctx, userID, result.Request.ID)
	case result.Request.Status == app.CatPhotoSuperseded:
		r.sendText(ctx, userID, "Кот изменился до решения по фотографии. Заявка закрыта без оплаты.")
	default:
		r.sendText(ctx, userID, "❌ Фото отклонено. Аватар не изменился. Можешь прислать другое через /photo.")
	}
	return true
}

// A completed request is historical: a later request may have replaced its photo.
func photoReviewStatusText(status app.CatPhotoStatus) string {
	switch status {
	case app.CatPhotoApplied:
		return "✅ Фото по этой заявке одобрено и установлено."
	case app.CatPhotoAwaitingPayment:
		return "✅ Фото одобрено. Ожидается оплата 10 ⭐."
	case app.CatPhotoRejected:
		return "❌ Фото по этой заявке отклонено."
	case app.CatPhotoSuperseded:
		return "Заявка закрыта: кот изменился до решения."
	case app.CatPhotoRefunded:
		return "По этой заявке оформлен возврат оплаты."
	default:
		return ""
	}
}

func (r *Router) showPhotoReviewDecision(ctx context.Context, cbc *cbCtx, callbackID string, req app.CatPhotoRequest) {
	text := photoReviewStatusText(req.Status)
	_ = r.send.AnswerCallbackText(ctx, callbackID, text)
	err := r.send.Call(ctx, "editMessageCaption", map[string]any{
		"chat_id": cbc.chatID, "message_id": cbc.msgID,
		"caption":      fmt.Sprintf("📷 Заявка #%d\n%s", req.ID, text),
		"reply_markup": map[string]any{"inline_keyboard": [][]map[string]any{}},
	})
	// A duplicate callback may target an already updated moderator card.
	if err != nil && !strings.Contains(err.Error(), "message is not modified") {
		r.log.Warn("photo review card update failed; decision remains saved")
	}
}

func (r *Router) sendCatPhotoInvoice(ctx context.Context, telegramID, requestID int64) {
	invoice, err := r.app.CatPhotos.CreateInvoice(ctx, requestID, telegramID)
	if err != nil {
		r.sendText(ctx, telegramID, "Не удалось подготовить счёт. Повтори /photo позже.")
		return
	}
	err = r.send.Call(ctx, "sendInvoice", map[string]any{
		"chat_id":        telegramID,
		"title":          "Сменить фото кота",
		"description":    "Одобренная модератором смена фотографии кота. Разовый платёж 10 ⭐. Возврат и поддержка: /paysupport.",
		"payload":        invoice.Payload,
		"provider_token": "", "currency": "XTR",
		"start_parameter": "cat_photo",
		"prices":          []map[string]any{{"label": "Смена фото кота", "amount": invoice.Stars}},
	})
	if err != nil {
		r.log.Warn("cat photo invoice send failed")
		r.sendText(ctx, telegramID, "Не удалось отправить счёт. Повтори /photo позже.")
	}
}
