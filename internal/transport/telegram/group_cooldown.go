package telegram

import (
	"catforge/internal/logx"
	"context"
	"fmt"
	"time"
)

type GroupCooldownStore interface {
	Claim(context.Context, int64, int64, string, time.Time, time.Duration) (bool, time.Time, error)
	Release(context.Context, int64, int64, string, time.Time) error
}

func (r *Router) SetGroupCooldownStore(store GroupCooldownStore) { r.groupCooldowns = store }

func (r *Router) claimGroupAction(ctx context.Context, tgc *tgCtx, actorID int64, action string, interval time.Duration) (bool, time.Time) {
	if r.groupCooldowns == nil || tgc.chatType == "private" {
		return true, time.Time{}
	}
	allowed, next, err := r.groupCooldowns.Claim(ctx, tgc.chatID, actorID, action, tgc.now, interval)
	if err != nil {
		r.log.Warn("group cooldown failed", logx.Any("err", err))
		return false, time.Time{}
	}
	return allowed, next
}

func (r *Router) notifyGroupCooldown(ctx context.Context, tgc *tgCtx, next time.Time) {
	if next.IsZero() {
		return
	}
	allowed, _ := r.claimGroupAction(ctx, tgc, 0, "limit_notice", time.Minute)
	if !allowed {
		return
	}
	seconds := max(1, int((next.Sub(tgc.now)+time.Second-1)/time.Second))
	wait := fmt.Sprintf("%d сек", seconds)
	if seconds >= 60 {
		wait = fmt.Sprintf("%d мин", (seconds+59)/60)
	}
	r.sendText(ctx, tgc.chatID, "Эту команду пока повторять рано. Попробуй через "+wait+". Повторный запрос не выполнен.")
}

func (r *Router) allowGroupCommand(ctx context.Context, tgc *tgCtx, cmd string) bool {
	if tgc.chatType == "private" || r.groupCooldowns == nil {
		return true
	}
	if !isPersonalCommand(cmd) && !isAdminCommand(cmd) && cmd != "/yard" && cmd != "/event" && cmd != "/fight" && cmd != "/week" && cmd != "/expedition" && cmd != "/collection" && cmd != "/bestiary" {
		return true
	}
	allowed, _ := r.claimGroupAction(ctx, tgc, tgc.userID, "input", 3*time.Second)
	if !allowed {
		return false
	}
	action, interval, actor := groupCommandCooldown(cmd, tgc.userID)
	if interval == 0 {
		return true
	}
	allowed, next := r.claimGroupAction(ctx, tgc, actor, action, interval)
	if !allowed {
		r.notifyGroupCooldown(ctx, tgc, next)
	}
	return allowed
}

func groupCommandCooldown(cmd string, userID int64) (string, time.Duration, int64) {
	switch cmd {
	case "/yard", "/event":
		return cmd, 30 * time.Second, 0
	case "/train", "/hunt":
		return "training", 10 * time.Second, userID
	case "/fight":
		return "fight", 10 * time.Second, userID
	case "/askcat", "/cat":
		return "cat_reply", 15 * time.Second, userID
	}
	return "", 0, userID
}

func (r *Router) allowGroupCallback(ctx context.Context, cbc *cbCtx, queryID string) bool {
	if cbc.chatType == "private" || r.groupCooldowns == nil {
		return true
	}
	allowed, _ := r.claimGroupAction(ctx, &cbc.tgCtx, cbc.userID, "input", 3*time.Second)
	if allowed {
		_, action, personal := ParsePersonalCallback(cbc.data)
		if personal {
			switch action {
			case CBTrainDo:
				allowed, _ = r.claimGroupAction(ctx, &cbc.tgCtx, cbc.userID, "training", 10*time.Second)
			case CBMenuFight:
				allowed, _ = r.claimGroupAction(ctx, &cbc.tgCtx, cbc.userID, "fight", 10*time.Second)
			}
		}
	}
	if !allowed {
		_ = r.send.AnswerCallbackText(ctx, queryID, "Слишком часто. Подожди немного и нажми ещё раз.")
	}
	return allowed
}
