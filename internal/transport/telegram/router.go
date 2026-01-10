package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"catforge/internal/app"
	"catforge/internal/domain"
	"catforge/internal/logx"
	"catforge/internal/transport/telegram/views"
)

type Router struct {
	app           *app.App
	send          *Sender
	publicBaseURL string
	botUsername   string
	log           logx.Logger
}

func NewRouter(app *app.App, send *Sender, publicBaseURL, botUsername string, log logx.Logger) *Router {
	if log == nil {
		log = logx.Nop()
	}
	return &Router{app: app, send: send, publicBaseURL: publicBaseURL, botUsername: botUsername, log: log}
}

// ---- HTTP entry ----

func (r *Router) WebhookHandler(w http.ResponseWriter, req *http.Request) {
	var upd Update
	if err := json.NewDecoder(req.Body).Decode(&upd); err != nil {
		r.log.Warn("bad webhook payload", logx.Any("err", err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx := req.Context()

	if upd.Message != nil && upd.Message.From != nil {
		r.onMessage(ctx, upd.Message)
		w.WriteHeader(http.StatusOK)
		return
	}

	if upd.CallbackQuery != nil {
		r.onCallback(ctx, upd.CallbackQuery)
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type tgCtx struct {
	userID   int64
	tgID     int64
	chatID   int64
	chatType string // private|group|supergroup|channel
	now      time.Time
}

func (r *Router) buildCtxFromMessage(ctx context.Context, m *Message) (*tgCtx, error) {
	if m == nil || m.From == nil || m.Chat == nil {
		return nil, errors.New("bad message payload")
	}
	chatType := strings.TrimSpace(m.Chat.Type)
	if chatType == "" {
		chatType = "private"
	}

	tgID := m.From.ID
	userID, err := r.app.EnsureUser(ctx, tgID)
	if err != nil {
		return nil, err
	}

	return &tgCtx{
		userID:   userID,
		tgID:     tgID,
		chatID:   m.Chat.ID,
		chatType: chatType,
		now:      r.app.Clock.Now(),
	}, nil
}

type cbCtx struct {
	tgCtx
	msgID int
	data  string
}

func (r *Router) buildCtxFromCallback(ctx context.Context, cq *CallbackQuery) (*cbCtx, error) {
	if cq == nil {
		return nil, errors.New("nil callback")
	}
	// всегда стопаем спиннер первым; игнорируем ошибки, чтобы избежать циклов
	_ = r.send.AnswerCallback(ctx, cq.ID)

	if cq.From == nil {
		return nil, errors.New("callback without from")
	}
	if cq.Message == nil || cq.Message.Chat == nil {
		return nil, errors.New("callback without message/chat (inline?)")
	}

	chatType := strings.TrimSpace(cq.Message.Chat.Type)
	if chatType == "" {
		chatType = "private"
	}

	tgID := cq.From.ID
	userID, err := r.app.EnsureUser(ctx, tgID)
	if err != nil {
		return nil, err
	}

	return &cbCtx{
		tgCtx: tgCtx{
			userID:   userID,
			tgID:     tgID,
			chatID:   cq.Message.Chat.ID,
			chatType: chatType,
			now:      r.app.Clock.Now(),
		},
		msgID: cq.Message.MessageID,
		data:  strings.TrimSpace(cq.Data),
	}, nil
}

type uiTarget struct {
	chatID int64
	msgID  int
}

type screen struct {
	photoURL string
	caption  string
	kb       any
}

func targetFromTG(tgc *tgCtx) uiTarget { return uiTarget{chatID: tgc.chatID, msgID: 0} }
func targetFromCB(cbc *cbCtx) uiTarget { return uiTarget{chatID: cbc.chatID, msgID: cbc.msgID} }
func (t uiTarget) isEdit() bool        { return t.msgID > 0 }

func (r *Router) showScreen(ctx context.Context, t uiTarget, s screen) {
	if t.isEdit() {
		r.editMediaKB(ctx, t.chatID, t.msgID, s.photoURL, s.caption, s.kb)
		return
	}
	r.sendPhotoKB(ctx, t.chatID, s.photoURL, s.caption, s.kb)
}

// ---- Message routing ----

func (r *Router) onMessage(ctx context.Context, m *Message) {
	tgc, err := r.buildCtxFromMessage(ctx, m)
	if err != nil {
		r.log.Warn("message ctx build failed", logx.Any("err", err))
		return
	}

	cmd, args := parseCommandAndArgs(m.Text)

	if tgc.chatType != "private" {
		r.onPublicCommand(ctx, tgc, cmd)
		return
	}

	// если пользователь в личке не указал имя кота, то попробуем его установить
	if cmd == "" && r.tryConsumePendingCatName(ctx, tgc, m.Text) {
		return
	}

	r.onPrivateCommand(ctx, tgc, cmd, args)
}

func (r *Router) onPublicCommand(ctx context.Context, tgc *tgCtx, cmd string) {
	switch cmd {
	case "/start":
		// привяжем чат как "домашний" для логов охоты из лички.
		r.bindHomeChat(ctx, tgc, true /*silent*/)
		r.renderPublicStart(ctx, tgc)
	case "/train", "/hunt":
		r.doTrainPublic(ctx, tgc)
	case "/stats":
		r.doStatsPublic(ctx, tgc)
	case "/bind":
		r.bindHomeChat(ctx, tgc, false /*silent*/)
	case "/unbind":
		r.unbindHomeChat(ctx, tgc)
	default:
	}
}

func (r *Router) bindHomeChat(ctx context.Context, tgc *tgCtx, silent bool) {
	if tgc.chatType == "" || tgc.chatType == "private" {
		return
	}
	if err := r.app.Users.SetHomeChat(ctx, tgc.userID, tgc.chatID, tgc.chatType); err != nil {
		if !silent {
			r.sendText(ctx, tgc.chatID, "Не удалось привязать чат. Попробуй позже.")
		}
		return
	}
	if !silent {
		r.sendText(ctx, tgc.chatID, "Этот чат привязан как домашний: охота из лички будет логироваться сюда.\nОтвязать: /unbind")
	}
}

func (r *Router) unbindHomeChat(ctx context.Context, tgc *tgCtx) {
	if tgc.chatType == "" || tgc.chatType == "private" {
		return
	}
	if err := r.app.Users.SetHomeChat(ctx, tgc.userID, 0, ""); err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось отвязать чат. Попробуй позже.")
		return
	}
	r.sendText(ctx, tgc.chatID, "Чат отвязан. Охота из лички больше не будет публиковаться сюда.")
}

func (r *Router) onPrivateCommand(ctx context.Context, tgc *tgCtx, cmd, args string) {
	switch cmd {
	case "/start":
		r.renderPrivateStart(ctx, tgc)
	case "/menu", "/home":
		r.renderPrivateStart(ctx, tgc)
	case "/profile":
		r.renderProfileTo(ctx, tgc.userID, tgc.now, targetFromTG(tgc))
	case "/train", "/hunt":
		r.renderTrainingTo(ctx, tgc.userID, tgc.now, targetFromTG(tgc), "")
	case "/reset":
		r.renderResetAskTo(ctx, tgc.userID, targetFromTG(tgc))
	case "/name":
		r.renameCatCommand(ctx, tgc, args)
	case "/skip":
		_ = r.app.Users.SetPendingAction(ctx, tgc.userID, "")
	default:
		r.sendText(ctx, tgc.chatID, "Команды: /start, /menu, /profile, /hunt (/train), /reset, /name")
	}
}

// ---- Callback routing ----

func (r *Router) onCallback(ctx context.Context, cq *CallbackQuery) {
	cbc, err := r.buildCtxFromCallback(ctx, cq)
	if err != nil {
		// inline callback или битый апдейт — просто игнор
		r.log.Warn("callback ignored", logx.Any("err", err))
		return
	}

	// Callbacks should be used only in private UI.
	if cbc.chatType != "private" {
		return
	}

	// starter:* handled separately (prefix routing)
	if strings.HasPrefix(cbc.data, CBStarterPrefix) {
		r.chooseStarter(ctx, cbc)
		return
	}

	switch cbc.data {
	case CBNavMenu:
		r.renderMenuTo(ctx, cbc.userID, targetFromCB(cbc))
	case CBResetAsk:
		r.renderResetAskTo(ctx, cbc.userID, targetFromCB(cbc))
	case CBResetConfirm:
		r.renderResetConfirm(ctx, cbc)
	case CBMenuCat, CBProfileRefresh:
		r.renderProfileTo(ctx, cbc.userID, cbc.now, targetFromCB(cbc))
	case CBMenuTrain, CBTrainRefresh:
		r.renderTrainingTo(ctx, cbc.userID, cbc.now, targetFromCB(cbc), "")
	case CBNameAsk:
		r.renderNameAsk(ctx, cbc)
	case CBNameSkip:
		r.nameSkip(ctx, cbc)
	case CBTrainDo:
		r.doTrainPrivate(ctx, cbc)
	case CBMenuPVP:
		r.renderArena(ctx, cbc)
	case CBNoop:
		return
	default:
		r.sendText(ctx, cbc.chatID, "Неизвестное действие.")
	}
}

// ---- Rendering / actions ----

func (r *Router) renderPublicStart(ctx context.Context, tgc *tgCtx) {
	if strings.TrimSpace(r.botUsername) == "" {
		r.sendText(ctx, tgc.chatID, "UI бота работает в личных сообщениях.\nОткрой бота в личке и нажми /start.")
		return
	}
	r.sendTextKB(
		ctx,
		tgc.chatID,
		"UI бота работает в личных сообщениях.\nНажми кнопку ниже, чтобы открыть меню:",
		views.OpenPrivateKeyboard(r.botUsername),
	)
}

func (r *Router) doTrainPublic(ctx context.Context, tgc *tgCtx) {
	_, _, err := r.app.Training.Train(ctx, tgc.userID, tgc.tgID, tgc.chatID, tgc.chatType)
	if err == nil {
		return
	}
	if errors.Is(err, domain.ErrNoCat) {
		r.sendText(ctx, tgc.chatID, "У тебя ещё нет кота. Создай в личке: открой бота и нажми /start.")
		return
	}
	r.sendText(ctx, tgc.chatID, "Не удалось поохотиться. Попробуй позже.")
}

func (r *Router) renderPrivateStart(ctx context.Context, tgc *tgCtx) {
	s := r.buildHomeOrStarterScreen(ctx, tgc.userID)
	r.showScreen(ctx, targetFromTG(tgc), s)
}

func (r *Router) sendPhotoKB(ctx context.Context, chatID int64, photoURL, caption string, kb any) {
	if err := r.send.PhotoWithKeyboard(ctx, chatID, photoURL, caption, kb); err != nil {
		r.log.Warn("telegram sendPhoto failed", logx.Int64("chat_id", chatID), logx.Any("err", err))
	}
}

func (r *Router) editMediaKB(ctx context.Context, chatID int64, msgID int, photoURL, caption string, kb any) {
	if err := r.send.EditMediaWithKeyboard(ctx, chatID, msgID, photoURL, caption, kb); err != nil {
		r.log.Warn("telegram editMessageMedia failed", logx.Int64("chat_id", chatID), logx.Int("msg_id", msgID), logx.Any("err", err))
	}
}

func (r *Router) photoForUser(ctx context.Context, userID int64) string {
	cat, err := r.app.Profile.GetCat(ctx, userID)
	if err == nil && cat != nil {
		return views.CatAvatarURL(r.publicBaseURL, cat)
	}
	return views.StarterScreenURL(r.publicBaseURL)
}

func (r *Router) chooseStarter(ctx context.Context, cbc *cbCtx) {
	breedStr := strings.TrimPrefix(cbc.data, CBStarterPrefix)
	cat, err := r.app.Starter.ChooseStarterCat(ctx, cbc.userID, domain.Breed(breedStr))
	if err != nil {
		s := screen{
			photoURL: views.StarterScreenURL(r.publicBaseURL),
			caption:  "Не удалось создать кота. Попробуй ещё раз.\n\nВыбери породу стартового кота:",
			kb:       StarterBreedKeyboard(),
		}
		r.showScreen(ctx, targetFromCB(cbc), s)
		return
	}

	_ = r.app.Users.SetPendingAction(ctx, cbc.userID, app.PendingAwaitCatName)

	photo := views.CatAvatarURL(r.publicBaseURL, cat)
	caption := "Дом кота:\n" +
		"Кот выбран: " + views.BreedRU(cat.Breed) + " • " + views.TraitRU(cat.Trait) + "\n\n" +
		"Имя по умолчанию: " + cat.Name + "\n" +
		"Теперь напиши желаемое имя одним сообщением.\n" +
		"Ограничение: 1.." + strconv.Itoa(domain.CatNameMaxLen) + " символов.\n" +
		"Можно пропустить: /skip"

	r.editMediaKB(ctx, cbc.chatID, cbc.msgID, photo, caption, NamePromptKeyboard())
}

func (r *Router) renderMenuTo(ctx context.Context, userID int64, t uiTarget) {
	s := screen{
		photoURL: r.photoForUser(ctx, userID),
		caption:  "Дом кота:",
		kb:       MainMenuKeyboard(),
	}
	r.showScreen(ctx, t, s)
}

func (r *Router) renderResetAskTo(ctx context.Context, userID int64, t uiTarget) {
	text := "♻️ Сброс кота\n\n" +
		"Это удалит кота и весь прогресс (уровень, XP, энергию).\n" +
		"После этого можно снова пройти /start и выбрать породу."
	s := screen{
		photoURL: r.photoForUser(ctx, userID),
		caption:  text,
		kb:       ResetConfirmKeyboard(),
	}
	r.showScreen(ctx, t, s)
}

func (r *Router) renderResetConfirm(ctx context.Context, cbc *cbCtx) {
	if err := r.app.Profile.ResetCat(ctx, cbc.userID); err != nil {
		s := screen{
			photoURL: views.StarterScreenURL(r.publicBaseURL),
			caption:  "Кота не нашли. Выбери породу стартового кота:",
			kb:       StarterBreedKeyboard(),
		}
		r.showScreen(ctx, targetFromCB(cbc), s)
		return
	}
	s := screen{
		photoURL: views.StarterScreenURL(r.publicBaseURL),
		caption:  "✅ Кот удалён. Выбери породу стартового кота:",
		kb:       StarterBreedKeyboard(),
	}
	r.showScreen(ctx, targetFromCB(cbc), s)

}

func (r *Router) renderProfileTo(ctx context.Context, userID int64, now time.Time, t uiTarget) {
	cat, err := r.app.Profile.GetCat(ctx, userID)
	if err != nil || cat == nil {
		s := screen{
			photoURL: views.StarterScreenURL(r.publicBaseURL),
			caption:  "Сначала выбери кота через /start.",
			kb:       MainMenuKeyboard(),
		}
		r.showScreen(ctx, t, s)
		return
	}
	s := screen{
		photoURL: views.CatAvatarURL(r.publicBaseURL, cat),
		caption:  views.FormatCatProfile(cat, now),
		kb:       ProfileKeyboard(),
	}
	r.showScreen(ctx, t, s)

}

func (r *Router) renderNameAsk(ctx context.Context, cbc *cbCtx) {
	cat, err := r.app.Profile.GetCat(ctx, cbc.userID)
	if err != nil || cat == nil {
		r.editMediaKB(ctx, cbc.chatID, cbc.msgID, views.StarterScreenURL(r.publicBaseURL),
			"Сначала выбери кота через /start.", MainMenuKeyboard())
		return
	}
	_ = r.app.Users.SetPendingAction(ctx, cbc.userID, app.PendingAwaitCatName)

	photo := views.CatAvatarURL(r.publicBaseURL, cat)
	text := "✏️ Имя кота\n\n" +
		"Текущее имя: " + cat.Name + "\n\n" +
		"Напиши новое имя одним сообщением.\n" +
		"Ограничение: 1.." + strconv.Itoa(domain.CatNameMaxLen) + " символов.\n" +
		"Можно пропустить: /skip"
	r.editMediaKB(ctx, cbc.chatID, cbc.msgID, photo, text, NamePromptKeyboard())
}

func (r *Router) nameSkip(ctx context.Context, cbc *cbCtx) {
	_ = r.app.Users.SetPendingAction(ctx, cbc.userID, "")
	r.renderProfileTo(ctx, cbc.userID, cbc.now, targetFromCB(cbc))
}

func (r *Router) tryConsumePendingCatName(ctx context.Context, tgc *tgCtx, text string) bool {
	a, err := r.app.Users.GetPendingAction(ctx, tgc.userID)
	if err != nil || a != app.PendingAwaitCatName {
		return false
	}

	name := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(text, "\n", " "), "\r", " "))
	name = strings.TrimSpace(name)
	if name == "" {
		r.sendText(ctx, tgc.chatID, "Имя пустое. Напиши имя одним сообщением или /skip.")
		return true
	}
	if utf8.RuneCountInString(name) > domain.CatNameMaxLen {
		r.sendText(ctx, tgc.chatID, "Слишком длинное имя. Максимум: "+strconv.Itoa(domain.CatNameMaxLen))
		return true
	}

	cat, err := r.app.Profile.RenameCat(ctx, tgc.userID, name)
	if err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось сохранить имя. Попробуй ещё раз или /skip.")
		return true
	}

	_ = r.app.Users.SetPendingAction(ctx, tgc.userID, "")
	r.sendText(ctx, tgc.chatID, "Имя установлено: "+cat.Name+"\nОткрой «Профиль кота», чтобы увидеть обновление.")
	return true
}

func (r *Router) renameCatCommand(ctx context.Context, tgc *tgCtx, args string) {
	name := strings.TrimSpace(args)
	if name == "" {
		r.sendText(ctx, tgc.chatID, "Использование: /name Барсик\nИли нажми «✏️ Имя» в профиле и напиши имя одним сообщением.")
		return
	}
	if utf8.RuneCountInString(name) > domain.CatNameMaxLen {
		r.sendText(ctx, tgc.chatID, "Слишком длинное имя. Максимум: "+strconv.Itoa(domain.CatNameMaxLen))
		return
	}
	cat, err := r.app.Profile.RenameCat(ctx, tgc.userID, name)
	if err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось переименовать. Попробуй позже.")
		return
	}
	_ = r.app.Users.SetPendingAction(ctx, tgc.userID, "")
	r.sendText(ctx, tgc.chatID, "Имя обновлено: "+cat.Name)
}

func (r *Router) renderTrainingTo(ctx context.Context, userID int64, now time.Time, t uiTarget, prefix string) {
	cat, err := r.app.Profile.GetCat(ctx, userID)
	if err != nil || cat == nil {
		s := screen{
			photoURL: views.StarterScreenURL(r.publicBaseURL),
			caption:  "Сначала выбери кота через /start.",
			kb:       MainMenuKeyboard(),
		}
		r.showScreen(ctx, t, s)
		return
	}
	s := r.buildTrainingScreen(cat, now, prefix)
	r.showScreen(ctx, t, s)
}

func (r *Router) doTrainPrivate(ctx context.Context, cbc *cbCtx) {
	cat, res, err := r.app.Training.Train(ctx, cbc.userID, cbc.tgID, cbc.chatID, "private")
	if err != nil {
		s := screen{
			photoURL: r.photoForUser(ctx, cbc.userID),
			caption:  "Не удалось провести охоту. Попробуй позже.",
			kb:       MainMenuKeyboard(),
		}
		r.showScreen(ctx, targetFromCB(cbc), s)
		return
	}

	head := views.FormatTrainingResultText(cat, r.app.Clock.Now(), res)
	r.renderTrainingTo(ctx, cbc.userID, r.app.Clock.Now(), targetFromCB(cbc), head)
}

func (r *Router) renderArena(ctx context.Context, cbc *cbCtx) {
	cat, err := r.app.Profile.GetCat(ctx, cbc.userID)
	if err != nil || cat == nil {
		r.editTextKB(ctx, cbc.chatID, cbc.msgID, "Арена недоступна: сначала выбери кота через /start.", MainMenuKeyboard())
		return
	}

	p := domain.Power(cat)
	text := "Арена (скоро)\n" +
		"Твоя боевая сила: " + strconv.Itoa(p) + "\n\n" +
		"Скоро здесь будет PvE бой с логом, потом PvP."
	r.editTextKB(ctx, cbc.chatID, cbc.msgID, text, ArenaKeyboard())
}

// ---- low-level send wrappers ----

func (r *Router) sendText(ctx context.Context, chatID int64, text string) {
	if err := r.send.Text(ctx, chatID, text); err != nil {
		r.log.Warn("telegram sendMessage failed", logx.Int64("chat_id", chatID), logx.Any("err", err))
	}
}

func (r *Router) sendTextKB(ctx context.Context, chatID int64, text string, kb any) {
	if err := r.send.TextWithKeyboard(ctx, chatID, text, kb); err != nil {
		r.log.Warn("telegram sendMessage failed", logx.Int64("chat_id", chatID), logx.Any("err", err))
	}
}

func (r *Router) editTextKB(ctx context.Context, chatID int64, msgID int, text string, kb any) {
	if err := r.send.EditTextWithKeyboard(ctx, chatID, msgID, text, kb); err != nil {
		r.log.Warn("telegram editMessageText failed", logx.Int64("chat_id", chatID), logx.Int("msg_id", msgID), logx.Any("err", err))
	}
}

func parseCommandAndArgs(text string) (cmd string, args string) {
	t := strings.TrimSpace(text)
	if t == "" || !strings.HasPrefix(t, "/") {
		return "", ""
	}
	i := strings.IndexFunc(t, unicode.IsSpace)
	token := t
	if i >= 0 {
		token = t[:i]
		args = strings.TrimSpace(t[i:])
	}
	if at := strings.IndexByte(token, '@'); at >= 0 {
		token = token[:at]
	}
	return token, args
}

func (r *Router) doStatsPublic(ctx context.Context, tgc *tgCtx) {
	cat, err := r.app.Profile.GetCat(ctx, tgc.userID)
	if err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось получить статистику. Попробуй позже.")
		return
	}
	if cat == nil {
		r.sendText(ctx, tgc.chatID, "У тебя ещё нет кота. Создай в личке: открой бота и нажми /start.")
		return
	}

	energy := domain.RegenEnergy(cat.Energy, cat.EnergyUpdatedAt, tgc.now)
	power := domain.Power(cat)
	badge := publicCatBadge(cat.Breed, cat.Level)

	line := badge + " Кот " + cat.Name + ": " +
		"уровень " + itoa(cat.Level) + ", энергия " + itoa(energy) + "/100, сила " + itoa(power)

	r.sendText(ctx, tgc.chatID, line)
}

// ---- Screen builders (pure-ish presentation assembly) ----

func (r *Router) buildHomeOrStarterScreen(ctx context.Context, userID int64) screen {
	cat, err := r.app.Profile.GetCat(ctx, userID)
	if err == nil && cat != nil {
		return screen{
			photoURL: views.CatAvatarURL(r.publicBaseURL, cat),
			caption:  "Дом кота:",
			kb:       MainMenuKeyboard(),
		}
	}
	return screen{
		photoURL: views.StarterScreenURL(r.publicBaseURL),
		caption:  "Выбери породу стартового кота:",
		kb:       StarterBreedKeyboard(),
	}
}

func (r *Router) buildTrainingScreen(cat *domain.Cat, now time.Time, prefix string) screen {
	photo := views.CatAvatarURL(r.publicBaseURL, cat)
	text, canTrain := views.FormatTrainingScreen(cat, now)
	caption := text
	if strings.TrimSpace(prefix) != "" {
		caption = prefix + "\n\n" + text
	}
	return screen{
		photoURL: photo,
		caption:  caption,
		kb:       TrainingKeyboard(canTrain),
	}
}
