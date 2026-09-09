package telegram

import (
	"catforge/internal/domain"
	"context"
	"strconv"
	"strings"
)

// Explicit target values make a repeated callback idempotent. The cat ID keeps
// an old card from changing settings of a newly created replacement cat.
func preferenceAction(kind string, catID int64, value string) string {
	return "pref:" + kind + ":" + strconv.FormatInt(catID, 10) + ":" + value
}

func (r *Router) changePreference(ctx context.Context, cbc *cbCtx) {
	if cbc.chatType != "private" || r.app.Personality == nil {
		return
	}
	cat, err := r.app.Profile.GetCat(ctx, cbc.userID)
	if err != nil || cat == nil {
		r.renderProfileTo(ctx, cbc.userID, cbc.now, targetFromCB(cbc))
		return
	}
	personality, err := r.app.Personality.Get(ctx, cat.ID)
	if err != nil || personality == nil {
		r.sendText(ctx, cbc.chatID, "Не удалось получить настройки. Попробуй обновить профиль.")
		return
	}
	kind, value := "", ""
	switch cbc.data {
	case CBProfileAutoSpeak:
		kind = "a"
		value = "on"
		if personality.AutoSpeakEnabled {
			value = "off"
		}
	case CBProfileHumor:
		kind = "h"
		value = "bold"
		if personality.HumorMode == domain.HumorBold {
			value = "normal"
		}
	default:
		parts := strings.Split(cbc.data, ":")
		if len(parts) != 4 || parts[0] != "pref" {
			return
		}
		id, parseErr := strconv.ParseInt(parts[2], 10, 64)
		if parseErr != nil || id != cat.ID {
			r.renderProfileTo(ctx, cbc.userID, cbc.now, targetFromCB(cbc))
			return
		}
		kind, value = parts[1], parts[3]
	}
	switch kind {
	case "a":
		enabled, ok := parseOnOff(value)
		if !ok {
			return
		}
		if enabled != personality.AutoSpeakEnabled {
			err = r.app.Personality.SetAutoSpeak(ctx, cat.ID, enabled)
		}
	case "h":
		mode := domain.HumorMode(value)
		if !domain.IsValidHumorMode(mode) {
			return
		}
		if mode != personality.HumorMode {
			err = r.app.Personality.SetHumorMode(ctx, cat.ID, mode)
		}
	default:
		return
	}
	if err != nil {
		r.sendText(ctx, cbc.chatID, "Не удалось сохранить настройку. Попробуй ещё раз.")
		return
	}
	r.renderProfileTo(ctx, cbc.userID, r.app.Clock.Now(), targetFromCB(cbc))
}
