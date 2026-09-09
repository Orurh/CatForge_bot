package telegram

import (
	"catforge/internal/observability"
	"context"
	"crypto/subtle"
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
	"catforge/internal/gamecontent"
	"catforge/internal/logx"
	"catforge/internal/transport/telegram/views"
)

type Router struct {
	app           *app.App
	send          *Sender
	publicBaseURL string
	botUsername   string
	webhookSecret string
	log           logx.Logger
}

func NewRouter(app *app.App, send *Sender, publicBaseURL, botUsername, webhookSecret string, log logx.Logger) *Router {
	if log == nil {
		log = logx.Nop()
	}
	return &Router{app: app, send: send, publicBaseURL: publicBaseURL, botUsername: botUsername, webhookSecret: webhookSecret, log: log}
}

// ---- HTTP entry ----

func (r *Router) WebhookHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.webhookSecret != "" {
		got := req.Header.Get("X-Telegram-Bot-Api-Secret-Token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(r.webhookSecret)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
	var upd Update
	if err := json.NewDecoder(req.Body).Decode(&upd); err != nil {
		r.log.Warn("bad webhook payload", logx.Any("err", err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := r.HandleUpdate(req.Context(), upd); err != nil {
		r.log.Error("telegram update failed", logx.Int("update_id", upd.UpdateID), logx.Any("err", err))
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleUpdate is shared by webhook and long-polling transports.
func (r *Router) HandleUpdate(ctx context.Context, upd Update) (resultErr error) {
	started := time.Now()
	defer func() { observability.Observe("telegram", "handleUpdate", started, resultErr) }()
	// Payment receipts have their own durable idempotency. Never claim them
	// before the ledger commits, otherwise a failed write would lose the retry.
	if upd.PreCheckoutQuery != nil {
		return r.preCheckout(ctx, upd.PreCheckoutQuery)
	}
	if upd.Message != nil && (upd.Message.SuccessfulPayment != nil || upd.Message.RefundedPayment != nil) {
		return r.paymentReceipt(ctx, upd.Message)
	}
	if r.app.Updates != nil {
		claimed, err := r.app.Updates.Claim(ctx, int64(upd.UpdateID))
		if err != nil {
			r.log.Error("telegram update claim failed", logx.Int("update_id", upd.UpdateID), logx.Any("err", err))
			return err
		}
		if !claimed {
			return nil
		}
	}

	if upd.Message != nil && upd.Message.From != nil {
		r.onMessage(ctx, upd.Message)
		return nil
	}

	if upd.CallbackQuery != nil {
		r.onCallback(ctx, upd.CallbackQuery)
		return nil
	}
	return nil
}

type tgCtx struct {
	userID   int64
	tgID     int64
	chatID   int64
	chatType string // private|group|supergroup|channel
	chatName string
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
		chatName: m.Chat.Title,
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
			chatName: cq.Message.Chat.Title,
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
	if m == nil || m.From == nil || m.From.IsBot {
		return
	}
	tgc, err := r.buildCtxFromMessage(ctx, m)
	if err != nil {
		r.log.Warn("message ctx build failed", logx.Any("err", err))
		return
	}

	cmd, args := parseCommandForBot(m.Text, r.botUsername)
	if cmd == "" {
		if r.tryConsumeSupportAmount(ctx, tgc, m) {
			return
		}
		if tgc.chatType != "private" && r.tryCatFollowup(ctx, tgc, m) {
			return
		}
		r.tryConsumePendingCatName(ctx, tgc, m)
		return
	}
	if isPersonalCommand(cmd) {
		r.handlePersonalCommand(ctx, tgc, cmd, args, m)
		return
	}
	if tgc.chatType == "private" {
		r.handlePrivateYardHelp(ctx, tgc, cmd)
		return
	}
	if isAdminCommand(cmd) {
		r.handleAdminCommand(ctx, tgc, cmd, args)
		return
	}
	r.handleYardCommand(ctx, tgc, cmd, m)
}

func (r *Router) tryCatFollowup(ctx context.Context, tgc *tgCtx, message *Message) bool {
	if r.app.CatFollowup == nil || message == nil || message.ReplyToMessage == nil {
		return false
	}
	humanReply := strings.TrimSpace(message.Text)
	if humanReply == "" || strings.HasPrefix(humanReply, "/") {
		return false
	}
	source := message.ReplyToMessage
	result, matched, err := r.app.CatFollowup.Reply(
		ctx, tgc.userID, tgc.chatID, source.MessageID, message.MessageID,
		source.Text, humanReply,
	)
	if !matched {
		if err != nil {
			r.log.Warn("cat followup reference claim failed", logx.Int64("chat_id", tgc.chatID), logx.Any("err", err))
		}
		return false
	}
	if err != nil {
		if !errors.Is(err, domain.ErrAIRateLimited) {
			r.log.Warn("cat followup generation failed", logx.Int64("chat_id", tgc.chatID), logx.Any("err", err))
		}
		return true
	}
	if result == nil || result.Cat == nil || strings.TrimSpace(result.Generation.Text) == "" {
		return true
	}
	line := publicCatBadge(result.Cat.Breed, result.Cat.Level) + " " + result.Cat.Name + ": " + strings.TrimSpace(result.Generation.Text)
	if _, err := r.send.TextReplyResult(ctx, tgc.chatID, message.MessageID, line); err != nil {
		r.log.Warn("cat followup send failed", logx.Int64("chat_id", tgc.chatID), logx.Any("err", err))
	}
	return true
}

func isPersonalCommand(cmd string) bool {
	switch cmd {
	case "/start", "/menu", "/home", "/profile", "/train", "/hunt", "/name", "/reset", "/askcat", "/cat", "/autospeak", "/humor", "/skip", "/support", "/terms", "/paysupport":
		return true
	default:
		return false
	}
}

func isAdminCommand(cmd string) bool {
	return cmd == "/yardsettings" || cmd == "/quiet" || cmd == "/eventstart"
}

func (r *Router) handlePersonalCommand(ctx context.Context, tgc *tgCtx, cmd, args string, message *Message) {
	if tgc.chatType != "private" && cmd == "/name" {
		r.renameCatFromGroup(ctx, tgc, args)
		return
	}
	if tgc.chatType != "private" && personalCommandIsPrivate(cmd) {
		if cmd == "/profile" {
			r.renderPublicProfile(ctx, tgc)
			return
		}
		r.openPrivatePersonalUI(ctx, tgc)
		return
	}
	switch cmd {
	case "/support":
		if strings.TrimSpace(args) == "" {
			r.showSupport(ctx, tgc)
		} else {
			r.chooseSupportAmount(ctx, tgc, args)
		}
	case "/terms":
		r.sendText(ctx, tgc.chatID, r.termsText())
	case "/paysupport":
		r.showPaySupport(ctx, tgc)
	case "/start", "/menu", "/home":
		r.app.RecordUserStarted(ctx, tgc.userID, tgc.tgID, tgc.chatType)
		r.renderStart(ctx, tgc, message)
	case "/profile":
		r.ensureYardMembership(ctx, tgc, message)
		r.renderProfileTo(ctx, tgc.userID, tgc.now, targetFromTG(tgc))
	case "/train", "/hunt":
		r.ensureYardMembership(ctx, tgc, message)
		r.doTrainCommand(ctx, tgc)
	case "/name":
		r.renameCatCommand(ctx, tgc, args)
	case "/reset":
		r.renderResetAskTo(ctx, tgc.userID, targetFromTG(tgc))
	case "/askcat":
		r.doRequestedCatReply(ctx, tgc, message, args, false)
	case "/cat":
		r.doRequestedCatReply(ctx, tgc, message, "", true)
	case "/humor":
		r.setHumorMode(ctx, tgc, args)
	case "/autospeak":
		r.configureCatAutoSpeak(ctx, tgc, args)
	case "/skip":
		_ = r.app.Users.ClearPendingInput(ctx, tgc.userID, tgc.chatID)
	}
}

func personalCommandIsPrivate(cmd string) bool {
	switch cmd {
	case "/start", "/menu", "/home", "/profile", "/name", "/reset", "/autospeak", "/humor", "/skip", "/support", "/terms", "/paysupport":
		return true
	default:
		return false
	}
}

func (r *Router) openPrivatePersonalUI(ctx context.Context, tgc *tgCtx) {
	text := contentText("help.personal_private")
	if strings.TrimSpace(r.botUsername) == "" {
		r.sendText(ctx, tgc.chatID, text)
		return
	}
	r.sendTextKB(ctx, tgc.chatID, text, views.OpenPrivateKeyboard(r.botUsername))
}

func (r *Router) renderPublicProfile(ctx context.Context, tgc *tgCtx) {
	cat, err := r.app.Profile.GetCat(ctx, tgc.userID)
	if err != nil || cat == nil {
		r.openPrivatePersonalUI(ctx, tgc)
		return
	}
	allowed, err := r.app.Users.ClaimDailyCommand(ctx, tgc.userID, tgc.chatID, "profile", app.GameTime(tgc.now))
	if err != nil {
		r.log.Warn("group profile quota failed", logx.Any("err", err))
		r.sendText(ctx, tgc.chatID, contentText("profile.group.error"))
		return
	}
	if !allowed {
		r.sendTextKB(ctx, tgc.chatID, contentText("profile.group.daily_limit"), views.OpenPrivateKeyboard(r.botUsername))
		return
	}
	arena, _ := r.app.Profile.ArenaStats(ctx, cat.ID)
	r.sendPhotoKB(ctx, tgc.chatID, views.CatAvatarURL(r.publicBaseURL, cat), views.FormatCatProfileWithArena(cat, tgc.now, arena), nil)
}

func (r *Router) handleYardCommand(ctx context.Context, tgc *tgCtx, cmd string, message *Message) {
	switch cmd {
	case "/yard":
		r.enterYard(ctx, tgc, message)
	case "/event":
		r.showYardEvent(ctx, tgc)
	case "/fight":
		r.toggleFight(ctx, tgc, message)
	case "/week":
		r.showYardWeeklySummary(ctx, tgc, message)
	case "/expedition":
		r.sendText(ctx, tgc.chatID, contentText("feature.disabled.legacy_expedition"))
	case "/collection", "/bestiary":
		r.sendText(ctx, tgc.chatID, contentText("feature.disabled.items"))
	}
}

func (r *Router) handleAdminCommand(ctx context.Context, tgc *tgCtx, cmd, args string) {
	switch cmd {
	case "/yardsettings":
		r.configureYard(ctx, tgc, args)
	case "/quiet":
		r.configureQuiet(ctx, tgc, args)
	case "/eventstart":
		r.startYardEventAdmin(ctx, tgc)
	}
}

func (r *Router) handlePrivateYardHelp(ctx context.Context, tgc *tgCtx, cmd string) {
	switch cmd {
	case "/yard", "/event", "/fight", "/week":
		r.sendText(ctx, tgc.chatID, contentText("help.yard_private"))
	case "/expedition":
		r.sendText(ctx, tgc.chatID, contentText("feature.disabled.legacy_expedition"))
	case "/collection", "/bestiary":
		r.sendText(ctx, tgc.chatID, contentText("feature.disabled.items"))
	}
}

func yardSettingsFrom(yard *domain.Yard) domain.YardSettings {
	return domain.YardSettings{
		HumorMode: yard.HumorMode, AutoMessagesEnabled: yard.AutoMessagesEnabled,
		MaxAutoMessagesDay: yard.MaxAutoMessagesDay, CatToCatBanter: yard.CatToCatBanter,
		FightsEnabled: yard.FightsEnabled, QuietUntil: yard.QuietUntil,
	}
}

func (r *Router) configureYard(ctx context.Context, tgc *tgCtx, args string) {
	if !r.requireYardAdmin(ctx, tgc) {
		return
	}
	yard, err := r.app.Yard.Settings(ctx, tgc.chatID)
	if err != nil {
		r.sendText(ctx, tgc.chatID, contentText("yard.settings.error.no_yard"))
		return
	}
	args = strings.ToLower(strings.TrimSpace(args))
	if args == "" || args == "status" {
		r.sendText(ctx, tgc.chatID, formatYardSettings(yard, tgc.now))
		return
	}
	parts := strings.Fields(args)
	if len(parts) != 2 {
		r.sendText(ctx, tgc.chatID, yardSettingsUsage())
		return
	}
	settings := yardSettingsFrom(yard)
	switch parts[0] {
	case "auto":
		value, ok := parseOnOff(parts[1])
		if !ok {
			r.sendText(ctx, tgc.chatID, yardSettingsUsage())
			return
		}
		settings.AutoMessagesEnabled = value
	case "banter":
		value, ok := parseOnOff(parts[1])
		if !ok {
			r.sendText(ctx, tgc.chatID, yardSettingsUsage())
			return
		}
		settings.CatToCatBanter = value
	case "fights":
		value, ok := parseOnOff(parts[1])
		if !ok {
			r.sendText(ctx, tgc.chatID, yardSettingsUsage())
			return
		}
		settings.FightsEnabled = value
	case "limit":
		value, convErr := strconv.Atoi(parts[1])
		if convErr != nil || value < 0 || value > 2 {
			r.sendText(ctx, tgc.chatID, yardSettingsUsage())
			return
		}
		settings.MaxAutoMessagesDay = value
	case "humor":
		mode := domain.HumorMode(parts[1])
		if !domain.IsValidHumorMode(mode) {
			r.sendText(ctx, tgc.chatID, yardSettingsUsage())
			return
		}
		settings.HumorMode = mode
	default:
		r.sendText(ctx, tgc.chatID, yardSettingsUsage())
		return
	}
	updated, err := r.app.Yard.SaveSettings(ctx, tgc.chatID, settings)
	if err != nil {
		r.sendText(ctx, tgc.chatID, contentText("yard.settings.error.save"))
		return
	}
	r.sendText(ctx, tgc.chatID, contentText("yard.settings.updated")+"\n\n"+formatYardSettings(updated, tgc.now))
}

func (r *Router) configureQuiet(ctx context.Context, tgc *tgCtx, args string) {
	if !r.requireYardAdmin(ctx, tgc) {
		return
	}
	yard, err := r.app.Yard.Settings(ctx, tgc.chatID)
	if err != nil {
		r.sendText(ctx, tgc.chatID, contentText("yard.settings.error.no_yard"))
		return
	}
	value := strings.ToLower(strings.TrimSpace(args))
	if value == "" || value == "status" {
		r.sendText(ctx, tgc.chatID, formatYardSettings(yard, tgc.now))
		return
	}
	settings := yardSettingsFrom(yard)
	switch value {
	case "24h", "on":
		settings.QuietUntil = tgc.now.Add(24 * time.Hour)
	case "off":
		settings.QuietUntil = time.Time{}
	default:
		r.sendText(ctx, tgc.chatID, contentText("yard.settings.quiet.usage"))
		return
	}
	updated, err := r.app.Yard.SaveSettings(ctx, tgc.chatID, settings)
	if err != nil {
		r.sendText(ctx, tgc.chatID, contentText("yard.settings.error.quiet"))
		return
	}
	r.sendText(ctx, tgc.chatID, contentText("yard.settings.updated")+"\n\n"+formatYardSettings(updated, tgc.now))
}

func (r *Router) configureCatAutoSpeak(ctx context.Context, tgc *tgCtx, args string) {
	cat, err := r.app.Profile.GetCat(ctx, tgc.userID)
	if err != nil || cat == nil {
		r.sendText(ctx, tgc.chatID, "Сначала создай кота через /start.")
		return
	}
	value := strings.ToLower(strings.TrimSpace(args))
	if value == "" || value == "status" {
		r.renderProfileTo(ctx, tgc.userID, tgc.now, uiTarget{chatID: tgc.chatID})
		return
	}

	enabled, ok := parseOnOff(value)
	if !ok {
		r.sendText(ctx, tgc.chatID, "Использование: /autospeak on или /autospeak off")
		return
	}
	if err := r.app.Personality.SetAutoSpeak(ctx, cat.ID, enabled); err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось изменить настройку кота.")
		return
	}
	r.renderProfileTo(ctx, tgc.userID, tgc.now, uiTarget{chatID: tgc.chatID})
}

func (r *Router) requireYardAdmin(ctx context.Context, tgc *tgCtx) bool {
	admin, err := r.send.IsChatAdmin(ctx, tgc.chatID, tgc.tgID)
	if err != nil {
		r.sendText(ctx, tgc.chatID, contentText("yard.settings.error.admin_check"))
		return false
	}
	if !admin {
		r.sendText(ctx, tgc.chatID, contentText("yard.settings.error.admin_only"))
	}
	return admin
}

func parseOnOff(value string) (bool, bool) {
	switch value {
	case "on":
		return true, true
	case "off":
		return false, true
	default:
		return false, false
	}
}

func yardSettingsUsage() string {
	return contentText("yard.settings.usage")
}

type yardSettingsTextData struct {
	YardName string
	Auto     string
	Limit    int
	Banter   string
	Fights   string
	Humor    string
	Quiet    string
}

func formatYardSettings(yard *domain.Yard, now time.Time) string {
	auto, banter, fights, quiet := contentText("yard.settings.value.off"), contentText("yard.settings.value.off"), contentText("yard.settings.value.off"), contentText("yard.settings.value.off")
	if yard.AutoMessagesEnabled {
		auto = contentText("yard.settings.value.on")
	}
	if yard.CatToCatBanter {
		banter = contentText("yard.settings.value.on")
	}
	if yard.FightsEnabled {
		fights = contentText("yard.settings.value.on")
	}
	if yard.QuietUntil.After(now) {
		quiet = gamecontent.Render("yard.settings.quiet.until", uint64(yard.ID), struct{ Until string }{yard.QuietUntil.Local().Format("02.01 15:04")})
	}
	status := gamecontent.Render("yard.settings.status", uint64(yard.ID), yardSettingsTextData{
		YardName: yard.Name, Auto: auto, Limit: yard.MaxAutoMessagesDay,
		Banter: banter, Fights: fights, Humor: string(yard.HumorMode), Quiet: quiet,
	})
	return status + "\n\n" + contentText("yard.settings.controls")
}

func (r *Router) doRequestedCatReply(ctx context.Context, tgc *tgCtx, message *Message, args string, requireReply bool) {
	if message == nil {
		return
	}
	userMessage := strings.TrimSpace(args)
	replyToID := message.MessageID
	if requireReply {
		if message.ReplyToMessage == nil || strings.TrimSpace(message.ReplyToMessage.Text) == "" {
			r.sendText(ctx, tgc.chatID, "Ответь командой /cat на текстовое сообщение, и кот вмешается.")
			return
		}
		userMessage = message.ReplyToMessage.Text
		replyToID = message.ReplyToMessage.MessageID
	} else if userMessage == "" {
		if message.ReplyToMessage != nil && strings.TrimSpace(message.ReplyToMessage.Text) != "" {
			userMessage = message.ReplyToMessage.Text
			replyToID = message.ReplyToMessage.MessageID
		} else {
			r.sendText(ctx, tgc.chatID, "Напиши вопрос после команды: /askcat стоит ли мне работать?\nИли ответь командой /askcat на текстовое сообщение.")
			return
		}
	}

	r.ensureYardMembership(ctx, tgc, message)
	cat, generation, err := r.app.CatReply.Reply(
		ctx, tgc.userID, tgc.chatID,
		app.CatReplyRequestKey(tgc.chatID, message.MessageID),
		userMessage,
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNoCat):
			r.sendText(ctx, tgc.chatID, "Сначала создай кота в личке через /start.")
		case errors.Is(err, domain.ErrAIRateLimited):
			r.sendText(ctx, tgc.chatID, "Кот устал разговаривать. Попробуй немного позже.")
		default:
			r.sendText(ctx, tgc.chatID, "Кот задумался и отказался формулировать. Попробуй позже.")
		}
		return
	}
	line := publicCatBadge(cat.Breed, cat.Level) + " " + cat.Name + ": " + generation.Text
	messageID, err := r.send.TextReplyResult(ctx, tgc.chatID, replyToID, line)
	if err != nil {
		r.log.Warn("cat reply send failed", logx.Int64("chat_id", tgc.chatID), logx.Any("err", err))
		return
	}
	if tgc.chatType != "private" && r.app.CatFollowup != nil {
		if err := r.app.CatFollowup.Remember(ctx, tgc.chatID, messageID, cat.ID); err != nil {
			r.log.Warn("cat message reference save failed", logx.Int64("chat_id", tgc.chatID), logx.Int("message_id", messageID), logx.Any("err", err))
		}
	}
}

func (r *Router) setHumorMode(ctx context.Context, tgc *tgCtx, args string) {
	cat, err := r.app.Profile.GetCat(ctx, tgc.userID)
	if err != nil || cat == nil {
		r.sendText(ctx, tgc.chatID, "Сначала создай кота через /start.")
		return
	}
	value := strings.ToLower(strings.TrimSpace(args))
	if value == "" || value == "status" {
		r.renderProfileTo(ctx, tgc.userID, tgc.now, uiTarget{chatID: tgc.chatID})
		return
	}

	mode := domain.HumorMode(value)
	if !domain.IsValidHumorMode(mode) {
		r.sendText(ctx, tgc.chatID, "Использование: /humor normal или /humor bold")
		return
	}
	if err := r.app.Personality.SetHumorMode(ctx, cat.ID, mode); err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось изменить режим юмора.")
		return
	}
	r.renderProfileTo(ctx, tgc.userID, tgc.now, uiTarget{chatID: tgc.chatID})
}

func (r *Router) enterYard(ctx context.Context, tgc *tgCtx, message *Message) {
	if message == nil || message.Chat == nil || tgc.chatType == "private" {
		r.sendText(ctx, tgc.chatID, "Двор создаётся только в группе.")
		return
	}
	snapshot, err := r.app.Yard.Enter(ctx, tgc.chatID, message.Chat.Title, tgc.userID)
	if err != nil {
		if errors.Is(err, domain.ErrNoCat) {
			r.sendText(ctx, tgc.chatID, "Сначала создай кота в личке через /start, затем вернись и повтори /yard.")
			return
		}
		r.sendText(ctx, tgc.chatID, "Не удалось открыть Двор. Попробуй позже.")
		return
	}
	r.sendText(ctx, tgc.chatID, formatYardSnapshot(snapshot))
}

func (r *Router) showYardEvent(ctx context.Context, tgc *tgCtx) {
	if tgc.chatType == "private" {
		r.sendText(ctx, tgc.chatID, "Событие Двора открывается только в группе.")
		return
	}
	if _, err := r.app.Yard.Enter(ctx, tgc.chatID, tgc.chatName, tgc.userID); err != nil {
		if errors.Is(err, domain.ErrNoCat) {
			r.openPrivatePersonalUI(ctx, tgc)
			return
		}
		r.sendText(ctx, tgc.chatID, contentText("yard_event.error.unavailable"))
		return
	}
	status, err := r.app.YardEvent.GetActive(ctx, tgc.chatID)
	if err != nil {
		if errors.Is(err, domain.ErrYardEventUnavailable) {
			r.sendText(ctx, tgc.chatID, contentText("yard_event.none"))
			return
		}
		r.sendText(ctx, tgc.chatID, contentText("yard_event.error.unavailable"))
		return
	}
	r.sendTextKB(ctx, tgc.chatID, formatYardEvent(status, tgc.now), YardEventKeyboard(status.Event.ID, status.Event.Type))
}

func (r *Router) startYardEventAdmin(ctx context.Context, tgc *tgCtx) {
	if !r.requireYardAdmin(ctx, tgc) {
		return
	}
	if _, err := r.app.Yard.Enter(ctx, tgc.chatID, tgc.chatName, tgc.userID); err != nil {
		if errors.Is(err, domain.ErrNoCat) {
			r.openPrivatePersonalUI(ctx, tgc)
			return
		}
		r.sendText(ctx, tgc.chatID, contentText("yard_event.error.unavailable"))
		return
	}
	status, err := r.app.YardEvent.StartOrGet(ctx, tgc.chatID)
	if err != nil {
		r.sendText(ctx, tgc.chatID, contentText("yard_event.error.unavailable"))
		return
	}
	r.sendTextKB(ctx, tgc.chatID, formatYardEvent(status, tgc.now), YardEventKeyboard(status.Event.ID, status.Event.Type))
}

func (r *Router) showYardWeeklySummary(ctx context.Context, tgc *tgCtx, message *Message) {
	if tgc.chatType == "private" || message == nil {
		r.sendText(ctx, tgc.chatID, "Недельные итоги доступны только в групповом Дворе.")
		return
	}
	result, err := r.app.WeeklySummary.Build(
		ctx, tgc.chatID, tgc.userID, app.WeeklySummaryRequestKey(tgc.chatID, message.MessageID),
	)
	if err != nil {
		if errors.Is(err, domain.ErrNoYard) {
			r.sendText(ctx, tgc.chatID, "Сначала создай Двор командой /yard.")
			return
		}
		r.sendText(ctx, tgc.chatID, "Не удалось собрать итоги Двора. Попробуй позже.")
		return
	}
	r.sendText(ctx, tgc.chatID, views.FormatYardWeeklySummary(result))
}

func (r *Router) chooseYardEvent(ctx context.Context, cbc *cbCtx, callbackID string) {
	parts := strings.SplitN(strings.TrimPrefix(cbc.data, CBYardChoicePrefix), ":", 2)
	if len(parts) != 2 {
		_ = r.send.AnswerCallbackText(ctx, callbackID, contentText("yard_event.choice.error.invalid"))
		return
	}
	eventID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || eventID <= 0 {
		_ = r.send.AnswerCallbackText(ctx, callbackID, contentText("yard_event.choice.error.invalid"))
		return
	}
	choice := domain.YardEventChoiceID(parts[1])
	status, err := r.app.YardEvent.Choose(ctx, cbc.chatID, eventID, cbc.userID, choice)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrFeatureLocked):
			_ = r.send.AnswerCallbackText(ctx, callbackID, strings.TrimPrefix(err.Error(), domain.ErrFeatureLocked.Error()+": "))
		case errors.Is(err, domain.ErrInvalidYardChoice):
			_ = r.send.AnswerCallbackText(ctx, callbackID, contentText("yard_event.choice.error.invalid"))
		case errors.Is(err, domain.ErrYardEventUnavailable):
			_ = r.send.AnswerCallbackText(ctx, callbackID, contentText("yard_event.choice.error.unavailable"))
		default:
			_ = r.send.AnswerCallbackText(ctx, callbackID, contentText("yard_event.choice.error.failed"))
		}
		return
	}
	if action, ok := domain.SpecialActionByID(string(choice)); ok {
		_ = r.send.AnswerCallbackText(ctx, callbackID, "Выбрано: "+action.Label)
	} else {
		_ = r.send.AnswerCallbackText(ctx, callbackID, gamecontent.Render(yardEventContentKey(status.Event.Type, "choice.saved."+string(choice)), uint64(cbc.userID), nil))
	}
	r.editTextKB(ctx, cbc.chatID, cbc.msgID, formatYardEvent(status, cbc.now), YardEventKeyboard(eventID, status.Event.Type))
}

type yardEventCardData struct {
	Total     int
	Remaining string
}

func formatYardEvent(status app.YardEventStatus, now time.Time) string {
	steal := status.Counts[domain.YardChoiceSteal]
	distract := status.Counts[domain.YardChoiceDistract]
	scout := status.Counts[domain.YardChoiceScout]
	total := steal + distract + scout
	selector := uint64(0)
	eventType := domain.YardEventFishTruck
	if status.Event != nil {
		selector = uint64(status.Event.Seed)
		eventType = status.Event.Type
	}
	data := yardEventCardData{Total: total}
	parts := []string{
		gamecontent.Render(yardEventContentKey(eventType, "card.title"), selector, data),
		gamecontent.Render(yardEventContentKey(eventType, "card.scene"), selector, data),
		gamecontent.Render(yardEventContentKey(eventType, "card.rules"), selector, data),
		gamecontent.Render(yardEventContentKey(eventType, "card.reward"), selector, data),
		gamecontent.Render("yard_event.card.progress", selector, data),
	}
	if status.Event != nil && !status.Event.ResolvesAt.IsZero() {
		remaining := status.Event.ResolvesAt.Sub(now)
		if remaining > 0 {
			data.Remaining = formatYardEventDuration(remaining)
			parts = append(parts, gamecontent.Render("yard_event.card.remaining", selector, data))
		}
	}
	if status.Event != nil {
		if action, ok := domain.FeaturedSpecial(eventType, status.Event.ID); ok {
			parts = append(parts, "✨ "+action.Label+". Нужно: "+action.Requirement)
		}
	}
	return strings.Join(parts, "\n\n")
}

func formatYardEventDuration(duration time.Duration) string {
	minutes := int(duration.Round(time.Minute) / time.Minute)
	if minutes < 1 {
		return "меньше минуты"
	}
	if minutes < 60 {
		return strconv.Itoa(minutes) + " мин"
	}
	hours := minutes / 60
	minutes %= 60
	if minutes == 0 {
		return strconv.Itoa(hours) + " ч"
	}
	return strconv.Itoa(hours) + " ч " + strconv.Itoa(minutes) + " мин"
}

// ---- Callback routing ----

func (r *Router) onCallback(ctx context.Context, cq *CallbackQuery) {
	cbc, err := r.buildCtxFromCallback(ctx, cq)
	if err != nil {
		if cq != nil {
			_ = r.send.AnswerCallback(ctx, cq.ID)
		}
		r.log.Warn("callback ignored", logx.Any("err", err))
		return
	}
	if strings.HasPrefix(cbc.data, CBYardChoicePrefix) {
		r.chooseYardEvent(ctx, cbc, cq.ID)
		return
	}
	if ownerUserID, action, personal := ParsePersonalCallback(cbc.data); personal {
		if ownerUserID != cbc.userID {
			_ = r.send.AnswerCallbackText(ctx, cq.ID, contentText("callback.not_owner"))
			return
		}
		if cbc.chatType != "private" && personalCallbackIsPrivate(action) {
			_ = r.send.AnswerCallbackText(ctx, cq.ID, contentText("callback.personal_private"))
			return
		}
		cbc.data = action
		if strings.HasPrefix(cbc.data, CBFightRevengePrefix) {
			r.revengeFight(ctx, cbc, cq.ID)
			return
		}
		_ = r.send.AnswerCallback(ctx, cq.ID)
	} else {
		// Compatibility for private cards sent before owner-bound callbacks were
		// introduced. Group callbacks never use this path.
		if cbc.chatType != "private" {
			_ = r.send.AnswerCallbackText(ctx, cq.ID, contentText("callback.unknown"))
			return
		}
		_ = r.send.AnswerCallback(ctx, cq.ID)
	}
	if legacyCallbackDisabled(cbc.data) {
		key := "feature.disabled.items"
		if strings.HasPrefix(cbc.data, CBExpeditionChoosePrefix) || strings.HasPrefix(cbc.data, CBExpeditionDoPrefix) || cbc.data == CBMenuExp || cbc.data == CBExpeditionRefresh {
			key = "feature.disabled.legacy_expedition"
		}
		r.sendText(ctx, cbc.chatID, contentText(key))
		return
	}

	if strings.HasPrefix(cbc.data, "support:") {
		r.supportCallback(ctx, cbc, cq.ID)
		return
	}
	if strings.HasPrefix(cbc.data, "pref:") {
		r.changePreference(ctx, cbc)
		return
	}

	// starter:* handled separately (prefix routing)
	if strings.HasPrefix(cbc.data, CBStarterPrefix) {
		r.chooseStarter(ctx, cbc)
		return
	}
	if strings.HasPrefix(cbc.data, CBExpeditionChoosePrefix) {
		r.renderExpeditionDifficulty(ctx, cbc, domain.ExpeditionLocation(strings.TrimPrefix(cbc.data, CBExpeditionChoosePrefix)))
		return
	}
	if strings.HasPrefix(cbc.data, CBExpeditionDoPrefix) {
		parts := strings.Split(strings.TrimPrefix(cbc.data, CBExpeditionDoPrefix), ":")
		if len(parts) != 2 {
			r.sendText(ctx, cbc.chatID, "Неверный маршрут экспедиции.")
			return
		}
		r.doExpeditionPrivate(ctx, cbc, domain.ExpeditionLocation(parts[0]), domain.ExpeditionDifficulty(parts[1]))
		return
	}
	if strings.HasPrefix(cbc.data, CBCollectionEquipPrefix) {
		r.equipCollectionItem(ctx, cbc, strings.TrimPrefix(cbc.data, CBCollectionEquipPrefix))
		return
	}
	if strings.HasPrefix(cbc.data, CBCollectionUpgradePrefix) {
		r.upgradeCollectionItem(ctx, cbc, strings.TrimPrefix(cbc.data, CBCollectionUpgradePrefix))
		return
	}
	if strings.HasPrefix(cbc.data, CBCollectionItemPrefix) {
		r.renderCollectionItem(ctx, cbc.userID, strings.TrimPrefix(cbc.data, CBCollectionItemPrefix), targetFromCB(cbc), "")
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
	case CBMenuExp, CBExpeditionRefresh:
		r.renderExpeditionTo(ctx, cbc.userID, cbc.now, targetFromCB(cbc), "")
	case CBCollection:
		r.renderCollectionTo(ctx, cbc.userID, targetFromCB(cbc))
	case CBBestiary:
		r.renderBestiaryTo(ctx, cbc.userID, targetFromCB(cbc))
	case CBNameAsk:
		r.renderNameAsk(ctx, cbc)
	case CBNameSkip:
		r.nameSkip(ctx, cbc)
	case CBTrainDo:
		r.doTrainCallback(ctx, cbc)
	case CBMenuYard:
		if cbc.chatType == "private" {
			r.sendText(ctx, cbc.chatID, contentText("help.yard_private"))
		} else {
			r.sendText(ctx, cbc.chatID, "Открой состояние Двора командой /yard.")
		}
	case CBMenuFight:
		r.toggleFight(ctx, &cbc.tgCtx, nil)
	case CBMenuAskCat:
		r.sendText(ctx, cbc.chatID, contentText("help.askcat"))
	case CBProfileAutoSpeak:
		r.changePreference(ctx, cbc)
	case CBProfileHumor:
		r.changePreference(ctx, cbc)
	case CBMenuPVP:
		r.renderArena(ctx, cbc)
	case CBNoop:
		return
	default:
		r.sendText(ctx, cbc.chatID, contentText("callback.unknown"))
	}
}

func legacyCallbackDisabled(action string) bool {
	return strings.HasPrefix(action, CBExpeditionChoosePrefix) || strings.HasPrefix(action, CBExpeditionDoPrefix) ||
		strings.HasPrefix(action, CBCollectionUpgradePrefix) ||
		action == CBMenuExp || action == CBExpeditionRefresh ||
		action == CBBestiary
}

func personalCallbackIsPrivate(action string) bool {
	if strings.HasPrefix(action, CBFightRevengePrefix) {
		return false
	}
	switch action {
	case CBTrainDo, CBMenuYard, CBMenuFight, CBMenuAskCat, CBNoop:
		return false
	default:
		return true
	}
}

// ---- Rendering / actions ----

func (r *Router) renderStart(ctx context.Context, tgc *tgCtx, message *Message) {
	r.ensureYardMembership(ctx, tgc, message)
	s := r.buildHomeOrStarterScreen(ctx, tgc.userID)
	r.showScreen(ctx, targetFromTG(tgc), s)
}

func (r *Router) ensureYardMembership(ctx context.Context, tgc *tgCtx, message *Message) {
	if tgc == nil || tgc.chatType == "private" {
		return
	}
	chatName := tgc.chatName
	if message != nil && message.Chat != nil {
		chatName = message.Chat.Title
	}
	_, _ = r.app.Yard.Enter(ctx, tgc.chatID, chatName, tgc.userID)
}

func (r *Router) doTrainCommand(ctx context.Context, tgc *tgCtx) {
	cat, result, generation, err := r.app.Training.Train(ctx, tgc.userID, tgc.tgID, tgc.chatID, tgc.chatType)
	if err != nil {
		if errors.Is(err, domain.ErrNoCat) {
			r.sendText(ctx, tgc.chatID, "У тебя ещё нет кота. Создай его здесь через /start.")
			return
		}
		r.sendText(ctx, tgc.chatID, "Не удалось потренироваться. Попробуй позже.")
		return
	}
	if tgc.chatType == "private" {
		r.sendText(ctx, tgc.chatID, views.FormatTrainingResultText(cat, result, generation.Text, r.app.Clock.Now()))
	}
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
			kb:       StarterBreedKeyboard(cbc.userID),
		}
		r.showScreen(ctx, targetFromCB(cbc), s)
		return
	}

	photo := views.CatAvatarURL(r.publicBaseURL, cat)
	caption := "Дом кота:\n" +
		"Кот выбран: " + views.BreedRU(cat.Breed) + " • " + views.TraitRU(cat.Trait) + "\n\n" +
		"Имя по умолчанию: " + cat.Name + "\n" +
		"Ниже появится персональный запрос имени."
	r.editMediaKB(ctx, cbc.chatID, cbc.msgID, photo, caption, ProfileKeyboard(cbc.userID))
	r.ensureYardMembership(ctx, &cbc.tgCtx, nil)
	r.promptForCatName(ctx, &cbc.tgCtx, cat, "Как зовут кота?")
}

func (r *Router) renderMenuTo(ctx context.Context, userID int64, t uiTarget) {
	r.showScreen(ctx, t, r.buildHomeOrStarterScreen(ctx, userID))
}

func (r *Router) renderResetAskTo(ctx context.Context, userID int64, t uiTarget) {
	text := "♻️ Сброс кота\n\n" +
		"Это удалит кота и весь прогресс (уровень, XP, энергию).\n" +
		"После этого можно снова пройти /start и выбрать породу."
	s := screen{
		photoURL: r.photoForUser(ctx, userID),
		caption:  text,
		kb:       ResetConfirmKeyboard(userID),
	}
	r.showScreen(ctx, t, s)
}

func (r *Router) renderResetConfirm(ctx context.Context, cbc *cbCtx) {
	if err := r.app.Profile.ResetCat(ctx, cbc.userID); err != nil {
		s := screen{
			photoURL: views.StarterScreenURL(r.publicBaseURL),
			caption:  "Кота не нашли. Выбери породу стартового кота:",
			kb:       StarterBreedKeyboard(cbc.userID),
		}
		r.showScreen(ctx, targetFromCB(cbc), s)
		return
	}
	s := screen{
		photoURL: views.StarterScreenURL(r.publicBaseURL),
		caption:  "✅ Кот удалён. Выбери породу стартового кота:",
		kb:       StarterBreedKeyboard(cbc.userID),
	}
	r.showScreen(ctx, targetFromCB(cbc), s)

}

func (r *Router) renderProfileTo(ctx context.Context, userID int64, now time.Time, t uiTarget) {
	cat, err := r.app.Profile.GetCat(ctx, userID)
	if err != nil || cat == nil {
		s := screen{
			photoURL: views.StarterScreenURL(r.publicBaseURL),
			caption:  contentText("menu.choose_starter"),
			kb:       StarterBreedKeyboard(userID),
		}
		r.showScreen(ctx, t, s)
		return
	}
	arena, _ := r.app.Profile.ArenaStats(ctx, cat.ID)
	entries, _ := r.app.Collection.List(ctx, userID)
	caption := views.FormatCatProfileWithArena(cat, now, arena)
	var personality *domain.CatPersonality
	if r.app.Personality != nil {
		personality, _ = r.app.Personality.Get(ctx, cat.ID)
	}
	kb := ProfileKeyboard(userID, personality)
	if cat.Level >= 2 && len(entries) > 0 {
		caption += "\n\n🎒 Снаряжение"
		for _, entry := range entries {
			if entry.Owned.Equipped && domain.SlotUnlocked(entry.Definition.Slot, cat.Level) {
				caption += "\n" + entry.Definition.Name + " · Lv" + strconv.Itoa(entry.Owned.Level)
			}
		}
		rows := kb["inline_keyboard"].([][]map[string]any)
		kb["inline_keyboard"] = append(rows, []map[string]any{{"text": "🎒 Предметы", "callback_data": PersonalCallback(userID, CBCollection)}})
	}
	s := screen{
		photoURL: views.CatAvatarURL(r.publicBaseURL, cat),
		caption:  caption,
		kb:       kb,
	}
	r.showScreen(ctx, t, s)

}

func (r *Router) renderCollectionTo(ctx context.Context, userID int64, t uiTarget) {
	cat, err := r.app.Profile.GetCat(ctx, userID)
	if err != nil || cat.Level < 2 {
		r.sendText(ctx, t.chatID, "🔒 Первый предмет откроется на Lv2.")
		return
	}
	entries, err := r.app.Collection.List(ctx, userID)
	if err != nil {
		r.showScreen(ctx, t, screen{photoURL: r.photoForUser(ctx, userID), caption: "Не удалось открыть коллекцию. Попробуй позже.", kb: ProfileKeyboard(userID)})
		return
	}
	r.showScreen(ctx, t, screen{photoURL: r.photoForUser(ctx, userID), caption: views.FormatCollection(entries), kb: CollectionKeyboard(entries)})
}

func (r *Router) renderBestiaryTo(ctx context.Context, userID int64, t uiTarget) {
	entries, err := r.app.Bestiary.List(ctx, userID)
	if err != nil {
		r.showScreen(ctx, t, screen{photoURL: r.photoForUser(ctx, userID), caption: "Не удалось открыть бестиарий.", kb: MainMenuKeyboard(userID)})
		return
	}
	r.showScreen(ctx, t, screen{photoURL: r.photoForUser(ctx, userID), caption: views.FormatBestiary(entries), kb: BestiaryKeyboard()})
}

func (r *Router) renderCollectionItem(ctx context.Context, userID int64, itemID string, t uiTarget, prefix string) {
	cat, err := r.app.Profile.GetCat(ctx, userID)
	if err != nil || cat.Level < 2 {
		r.renderCollectionTo(ctx, userID, t)
		return
	}
	entries, err := r.app.Collection.List(ctx, userID)
	if err != nil {
		r.renderCollectionTo(ctx, userID, t)
		return
	}
	for _, entry := range entries {
		if entry.Definition.ID == itemID {
			caption := views.FormatItem(entry, prefix)
			if !domain.SlotUnlocked(entry.Definition.Slot, cat.Level) {
				caption += "\n🔒 Этот слот откроется на Lv" + strconv.Itoa(domain.EquipmentSlotLevel(entry.Definition.Slot)) + "."
			}
			r.showScreen(ctx, t, screen{photoURL: views.ItemImageURL(entry.Definition.ID), caption: caption, kb: CollectionItemKeyboard(entry, cat.Level)})
			return
		}
	}
	r.renderCollectionTo(ctx, userID, t)
}

func (r *Router) equipCollectionItem(ctx context.Context, cbc *cbCtx, itemID string) {
	if err := r.app.Collection.Equip(ctx, cbc.userID, itemID); err != nil {
		r.renderCollectionItem(ctx, cbc.userID, itemID, targetFromCB(cbc), "Не удалось надеть предмет.")
		return
	}
	r.renderCollectionItem(ctx, cbc.userID, itemID, targetFromCB(cbc), "✅ Предмет надет — его бонусы действуют.")
}

func (r *Router) upgradeCollectionItem(ctx context.Context, cbc *cbCtx, itemID string) {
	_, err := r.app.Collection.Upgrade(ctx, cbc.userID, itemID)
	prefix := "⬆️ Предмет улучшен."
	if errors.Is(err, domain.ErrNotEnoughFragments) {
		prefix = "Не хватает фрагментов. Ищи дубликаты в экспедициях."
	} else if errors.Is(err, domain.ErrNotEnoughCoins) {
		prefix = "Не хватает монет."
	} else if err != nil {
		prefix = "Не удалось улучшить предмет."
	}
	r.renderCollectionItem(ctx, cbc.userID, itemID, targetFromCB(cbc), prefix)
}

func (r *Router) renderNameAsk(ctx context.Context, cbc *cbCtx) {
	cat, err := r.app.Profile.GetCat(ctx, cbc.userID)
	if err != nil || cat == nil {
		r.editMediaKB(ctx, cbc.chatID, cbc.msgID, views.StarterScreenURL(r.publicBaseURL),
			"Сначала выбери кота через /start.", MainMenuKeyboard(cbc.userID))
		return
	}
	r.promptForCatName(ctx, &cbc.tgCtx, cat, "Напиши новое имя для "+cat.Name)
}

func (r *Router) nameSkip(ctx context.Context, cbc *cbCtx) {
	_ = r.app.Users.ClearPendingInput(ctx, cbc.userID, cbc.chatID)
	r.renderProfileTo(ctx, cbc.userID, cbc.now, targetFromCB(cbc))
	r.sendFirstPersonalityLine(ctx, cbc.userID, cbc.chatID)
}

func (r *Router) promptForCatName(ctx context.Context, tgc *tgCtx, cat *domain.Cat, title string) {
	if tgc == nil || cat == nil {
		return
	}
	if tgc.chatType != "private" {
		r.openPrivatePersonalUI(ctx, tgc)
		return
	}
	prompt := "✏️ " + title + "\nОтветь на это сообщение именем — от 1 до " + strconv.Itoa(domain.CatNameMaxLen) + " символов."
	promptID, err := r.send.ForceReply(ctx, tgc.chatID, prompt, "Имя кота")
	if err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось открыть ввод имени. Используй /name Барсик.")
		return
	}
	if err := r.app.Users.SavePendingInput(ctx, app.PendingInput{
		UserID: tgc.userID, ChatID: tgc.chatID, Kind: app.PendingAwaitCatName,
		PromptMessageID: int64(promptID), ExpiresAt: tgc.now.Add(15 * time.Minute),
	}); err != nil {
		r.sendText(ctx, tgc.chatID, "Не удалось запомнить запрос имени. Используй /name Барсик.")
	}
}

func (r *Router) tryConsumePendingCatName(ctx context.Context, tgc *tgCtx, message *Message) bool {
	if message == nil || message.ReplyToMessage == nil {
		return false
	}
	pending, err := r.app.Users.GetPendingInput(ctx, tgc.userID, tgc.chatID)
	if err != nil || pending == nil || pending.Kind != app.PendingAwaitCatName || pending.PromptMessageID != int64(message.ReplyToMessage.MessageID) {
		return false
	}
	if tgc.chatType != "private" {
		_ = r.app.Users.ClearPendingInput(ctx, tgc.userID, tgc.chatID)
		r.openPrivatePersonalUI(ctx, tgc)
		return true
	}

	name := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(message.Text, "\n", " "), "\r", " "))
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

	_ = r.app.Users.ClearPendingInput(ctx, tgc.userID, tgc.chatID)
	replyText := "Имя установлено: " + cat.Name + "\nОткрой «Профиль кота», чтобы увидеть обновление."
	if line, ok := r.firstPersonalityLine(ctx, cat); ok {
		replyText += "\n\n" + line
	}
	r.sendText(ctx, tgc.chatID, replyText)
	return true
}

func (r *Router) sendFirstPersonalityLine(ctx context.Context, userID, chatID int64) {
	cat, err := r.app.Profile.GetCat(ctx, userID)
	if err != nil || cat == nil {
		return
	}
	if line, ok := r.firstPersonalityLine(ctx, cat); ok {
		r.sendText(ctx, chatID, line)
	}
}

func (r *Router) firstPersonalityLine(ctx context.Context, cat *domain.Cat) (string, bool) {
	if r.app.Personality == nil {
		return "", false
	}
	generation, err := r.app.Personality.FirstLine(ctx, cat)
	if err != nil || strings.TrimSpace(generation.Text) == "" {
		return "", false
	}
	return publicCatBadge(cat.Breed, cat.Level) + " " + cat.Name + ": " + generation.Text, true
}

func (r *Router) renameCatCommand(ctx context.Context, tgc *tgCtx, args string) {
	name := strings.TrimSpace(args)
	if name == "" {
		cat, err := r.app.Profile.GetCat(ctx, tgc.userID)
		if err != nil || cat == nil {
			r.sendText(ctx, tgc.chatID, "Сначала создай кота через /start.")
			return
		}
		r.promptForCatName(ctx, tgc, cat, "Напиши новое имя для "+cat.Name)
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
	_ = r.app.Users.ClearPendingInput(ctx, tgc.userID, tgc.chatID)
	r.sendText(ctx, tgc.chatID, "Имя обновлено: "+cat.Name)
}

func (r *Router) renameCatFromGroup(ctx context.Context, tgc *tgCtx, args string) {
	if strings.TrimSpace(args) == "" {
		command := "/name"
		if username := strings.TrimPrefix(strings.TrimSpace(r.botUsername), "@"); username != "" {
			command += "@" + username
		}
		r.sendText(ctx, tgc.chatID, "Использование: "+command+" НовоеИмя")
		return
	}
	r.renameCatCommand(ctx, tgc, args)
}

func (r *Router) renderTrainingTo(ctx context.Context, userID int64, now time.Time, t uiTarget, prefix string) {
	cat, err := r.app.Profile.GetCat(ctx, userID)
	if err != nil || cat == nil {
		s := screen{
			photoURL: views.StarterScreenURL(r.publicBaseURL),
			caption:  contentText("menu.choose_starter"),
			kb:       StarterBreedKeyboard(userID),
		}
		r.showScreen(ctx, t, s)
		return
	}
	s := r.buildTrainingScreen(cat, now, prefix)
	r.showScreen(ctx, t, s)
}

func (r *Router) doTrainCallback(ctx context.Context, cbc *cbCtx) {
	r.ensureYardMembership(ctx, &cbc.tgCtx, nil)
	cat, res, generation, err := r.app.Training.Train(ctx, cbc.userID, cbc.tgID, cbc.chatID, cbc.chatType)
	if err != nil {
		s := screen{
			photoURL: r.photoForUser(ctx, cbc.userID),
			caption:  "Не удалось провести охоту. Попробуй позже.",
			kb:       MainMenuKeyboard(cbc.userID),
		}
		r.showScreen(ctx, targetFromCB(cbc), s)
		return
	}
	if cbc.chatType != "private" {
		r.renderProfileTo(ctx, cbc.userID, r.app.Clock.Now(), targetFromCB(cbc))
		return
	}

	head := views.FormatTrainingResultText(cat, res, generation.Text, r.app.Clock.Now())
	r.renderTrainingTo(ctx, cbc.userID, r.app.Clock.Now(), targetFromCB(cbc), head)
}

func (r *Router) renderArena(ctx context.Context, cbc *cbCtx) {
	cat, err := r.app.Profile.GetCat(ctx, cbc.userID)
	if err != nil || cat == nil {
		r.editTextKB(ctx, cbc.chatID, cbc.msgID, "Арена недоступна: сначала выбери кота через /start.", MainMenuKeyboard(cbc.userID))
		return
	}

	p := domain.Power(cat)
	text := "Арена (скоро)\n" +
		"Твоя боевая сила: " + strconv.Itoa(p) + "\n\n" +
		"Скоро здесь будет PvE бой с логом, потом PvP."
	r.editTextKB(ctx, cbc.chatID, cbc.msgID, text, ArenaKeyboard())
}

func (r *Router) renderExpeditionTo(ctx context.Context, userID int64, now time.Time, t uiTarget, prefix string) {
	cat, err := r.app.Profile.GetCat(ctx, userID)
	if err != nil || cat == nil {
		r.showScreen(ctx, t, screen{photoURL: views.StarterScreenURL(r.publicBaseURL), caption: "Экспедиция недоступна: сначала выбери кота через /start.", kb: MainMenuKeyboard(userID)})
		return
	}
	text, canExplore := views.FormatExpeditionScreen(cat, now)
	if strings.TrimSpace(prefix) != "" {
		text = prefix + "\n\n" + text
	}
	r.showScreen(ctx, t, screen{photoURL: views.CatAvatarURL(r.publicBaseURL, cat), caption: text, kb: ExpeditionKeyboard(canExplore)})
}

func (r *Router) renderExpeditionDifficulty(ctx context.Context, cbc *cbCtx, location domain.ExpeditionLocation) {
	if location != domain.ExpeditionAlley && location != domain.ExpeditionRooftop && location != domain.ExpeditionPark {
		r.renderExpeditionTo(ctx, cbc.userID, cbc.now, targetFromCB(cbc), "")
		return
	}
	cat, err := r.app.Profile.GetCat(ctx, cbc.userID)
	if err != nil || cat == nil {
		r.renderExpeditionTo(ctx, cbc.userID, cbc.now, targetFromCB(cbc), "")
		return
	}
	energy := domain.RegenEnergy(cat.Energy, cat.EnergyUpdatedAt, cbc.now)
	r.showScreen(ctx, targetFromCB(cbc), screen{
		photoURL: views.CatAvatarURL(r.publicBaseURL, cat),
		caption:  views.FormatExpeditionDifficulty(cat, cbc.now, location),
		kb:       ExpeditionDifficultyKeyboard(string(location), energy),
	})
}

func (r *Router) doExpeditionPrivate(ctx context.Context, cbc *cbCtx, location domain.ExpeditionLocation, difficulty domain.ExpeditionDifficulty) {
	cat, result, err := r.app.Expedition.Explore(ctx, cbc.userID, location, difficulty)
	if err != nil {
		r.showScreen(ctx, targetFromCB(cbc), screen{photoURL: r.photoForUser(ctx, cbc.userID), caption: "Не удалось провести экспедицию. Попробуй позже.", kb: MainMenuKeyboard(cbc.userID)})
		return
	}
	head := views.FormatExpeditionResultText(cat, result)
	if result.Loot.Dropped && result.Loot.New {
		text, canExplore := views.FormatExpeditionScreen(cat, r.app.Clock.Now())
		r.showScreen(ctx, targetFromCB(cbc), screen{
			photoURL: views.CatAvatarURL(r.publicBaseURL, cat),
			caption:  head + "\n\n" + text,
			kb:       ExpeditionLootKeyboard(canExplore, result.Loot.ItemID),
		})
		return
	}
	r.renderExpeditionTo(ctx, cbc.userID, r.app.Clock.Now(), targetFromCB(cbc), head)
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
	return parseCommandForBot(text, "")
}

func parseCommandForBot(text, botUsername string) (cmd string, args string) {
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
		target := token[at+1:]
		configured := strings.TrimPrefix(strings.TrimSpace(botUsername), "@")
		if configured != "" && !strings.EqualFold(target, configured) {
			return "", ""
		}
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
		energy := domain.RegenEnergy(cat.Energy, cat.EnergyUpdatedAt, r.app.Clock.Now())
		return screen{
			photoURL: views.CatAvatarURL(r.publicBaseURL, cat),
			caption: views.BreedIcon(cat.Breed) + " " + cat.Name + "\n" +
				"Lv." + strconv.Itoa(cat.Level) + " • " + views.TraitRU(cat.Trait) + "\n" +
				"⚡ " + strconv.Itoa(energy) + "/100",
			kb: MainMenuKeyboard(userID, cat.Level),
		}
	}
	return screen{
		photoURL: views.StarterScreenURL(r.publicBaseURL),
		caption:  contentText("menu.choose_starter"),
		kb:       StarterBreedKeyboard(userID),
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
		kb:       TrainingKeyboard(cat.UserID, canTrain),
	}
}
