package telegram

import (
	"context"
	"errors"
	"strings"

	"catforge/internal/app"
	"catforge/internal/domain"
	"catforge/internal/gamecontent"
	"catforge/internal/logx"
)

type fightTextData struct {
	CatName      string
	AttackerName string
	DefenderName string
	WinnerName   string
	LoserName    string
	WinnerHP     int
	Rounds       int
	Rivalry      int
}

func (r *Router) toggleFight(ctx context.Context, tgc *tgCtx, message *Message) {
	if tgc == nil || tgc.chatType == "private" {
		if tgc != nil {
			r.sendText(ctx, tgc.chatID, contentText("help.yard_private"))
		}
		return
	}
	chatName := tgc.chatName
	if message != nil && message.Chat != nil {
		chatName = message.Chat.Title
	}
	if _, err := r.app.Yard.Enter(ctx, tgc.chatID, chatName, tgc.userID); err != nil {
		if errors.Is(err, domain.ErrNoCat) {
			r.openPrivatePersonalUI(ctx, tgc)
			return
		}
		r.log.Warn("arena membership failed", logx.Any("err", err))
		r.sendText(ctx, tgc.chatID, contentText("fight.error.unavailable"))
		return
	}

	outcome, err := r.app.Fight.Toggle(ctx, tgc.chatID, tgc.userID)
	if err != nil {
		r.log.Warn("arena toggle failed", logx.Any("err", err))
		r.sendText(ctx, tgc.chatID, contentText("fight.error.unavailable"))
		return
	}
	r.sendText(ctx, tgc.chatID, formatFightOutcome(outcome))
}

func formatFightOutcome(outcome app.FightOutcome) string {
	if outcome.Cat == nil {
		return contentText("fight.error.unavailable")
	}
	selector := uint64(outcome.Cat.ID)
	data := fightTextData{CatName: outcome.Cat.Name}
	switch outcome.Status {
	case domain.FightQueueWaiting:
		return gamecontent.Render("fight.queue.waiting", selector, data)
	case domain.FightQueueCanceled:
		return gamecontent.Render("fight.queue.canceled", selector, data)
	case domain.FightQueueMatched:
		return formatFinishedFight(outcome)
	default:
		return contentText("fight.error.unavailable")
	}
}

func formatFinishedFight(outcome app.FightOutcome) string {
	catA, catB := outcome.Opponent, outcome.Cat
	if catA == nil || catB == nil {
		return contentText("fight.error.unavailable")
	}
	winner, loser := catA, catB
	winnerHP := outcome.Result.FinalHPA
	if outcome.Result.WinnerCatID == catB.ID {
		winner, loser = catB, catA
		winnerHP = outcome.Result.FinalHPB
	}
	data := fightTextData{
		AttackerName: catB.Name, DefenderName: catA.Name,
		WinnerName: winner.Name, LoserName: loser.Name, WinnerHP: winnerHP,
		Rounds: outcome.Result.Rounds, Rivalry: outcome.Rivalry,
	}
	selector := outcome.Seed
	parts := []string{gamecontent.Render("fight.start", selector, data)}
	resultKey := "fight.result.win"
	if winnerHP <= 5 {
		resultKey = "fight.result.close"
	}
	parts = append(parts, gamecontent.Render(resultKey, selector+1, data))
	if winner.Level < loser.Level {
		parts = append(parts, gamecontent.Render("fight.result.upset", selector+2, data))
	}
	parts = append(parts, gamecontent.Render("fight.result.stats", selector, data))
	return strings.Join(parts, "\n\n")
}
