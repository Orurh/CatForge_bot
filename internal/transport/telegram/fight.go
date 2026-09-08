package telegram

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"catforge/internal/app"
	"catforge/internal/domain"
	"catforge/internal/gamecontent"
	"catforge/internal/logx"
	"catforge/internal/transport/telegram/views"
)

type fightTextData struct {
	CatName      string
	AttackerName string
	DefenderName string
	FirstName    string
	SecondName   string
	WinnerName   string
	LoserName    string
	WinnerHP     int
	Damage       int
	DefenderHP   int
	Round        int
	Rounds       int
	Rivalry      int
	Omitted      int
	CatAName     string
	CatBName     string
	CatAWins     int
	CatALosses   int
	CatBWins     int
	CatBLosses   int
	PairCatAWins int
	PairCatBWins int
	WinnerXPGain int64
	LoserXPGain  int64
}

const fightNarrativeTurnLimit = 10

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
		if r.sendFightDomainError(ctx, tgc, err, "") {
			return
		}
		r.log.Warn("arena toggle failed", logx.Any("err", err))
		r.sendText(ctx, tgc.chatID, contentText("fight.error.unavailable"))
		return
	}
	if outcome.Status == domain.FightQueueMatched {
		r.sendText(ctx, tgc.chatID, formatFightOutcome(outcome))
		r.sendFightBanter(ctx, tgc.chatID, outcome.Banter)
		return
	}
	r.sendText(ctx, tgc.chatID, formatFightOutcome(outcome))
}

func (r *Router) revengeFight(ctx context.Context, cbc *cbCtx, callbackID string) {
	fightID, err := strconv.ParseInt(strings.TrimPrefix(cbc.data, CBFightRevengePrefix), 10, 64)
	if err != nil || fightID <= 0 {
		_ = r.send.AnswerCallbackText(ctx, callbackID, contentText("fight.revenge.expired"))
		return
	}
	outcome, err := r.app.Fight.Revenge(ctx, cbc.chatID, fightID, cbc.userID)
	if err != nil {
		if r.sendFightDomainError(ctx, &cbc.tgCtx, err, callbackID) {
			return
		}
		r.log.Warn("fight revenge failed", logx.Any("err", err))
		_ = r.send.AnswerCallbackText(ctx, callbackID, contentText("fight.error.unavailable"))
		return
	}
	_ = r.send.AnswerCallbackText(ctx, callbackID, contentText("fight.revenge.accepted"))
	r.sendText(ctx, cbc.chatID, formatFightOutcome(outcome))
	r.sendFightBanter(ctx, cbc.chatID, outcome.Banter)
}

func (r *Router) sendFightBanter(ctx context.Context, chatID int64, banter *app.FightBanter) {
	text := formatFightBanter(banter)
	if text != "" {
		r.sendText(ctx, chatID, text)
	}
}

func formatFightBanter(banter *app.FightBanter) string {
	if banter == nil || banter.FirstCat == nil || banter.SecondCat == nil || strings.TrimSpace(banter.FirstLine) == "" || strings.TrimSpace(banter.SecondLine) == "" {
		return ""
	}
	return publicCatBadge(banter.FirstCat.Breed, banter.FirstCat.Level) + " " + banter.FirstCat.Name + ": " + strings.TrimSpace(banter.FirstLine) +
		"\n\n" + publicCatBadge(banter.SecondCat.Breed, banter.SecondCat.Level) + " " + banter.SecondCat.Name + ": " + strings.TrimSpace(banter.SecondLine)
}

func (r *Router) sendFightDomainError(ctx context.Context, tgc *tgCtx, err error, callbackID string) bool {
	key := ""
	switch {
	case errors.Is(err, domain.ErrConcurrentUpdate):
		r.sendText(ctx, tgc.chatID, "Кот успел измениться. Выйди на арену ещё раз — лимит боя не потрачен.")
		return true
	case errors.Is(err, domain.ErrFeatureLocked):
		r.sendText(ctx, tgc.chatID, "🔒 Арена откроется на Lv3.")
		return true
	case errors.Is(err, domain.ErrFightsDisabled):
		key = "fight.disabled"
	case errors.Is(err, domain.ErrFightDailyLimit):
		key = "fight.limit.daily"
	case errors.Is(err, domain.ErrFightPairCooldown):
		key = "fight.limit.pair"
	case errors.Is(err, domain.ErrFightRevengeExpired):
		key = "fight.revenge.expired"
	default:
		return false
	}
	text := gamecontent.Render(key, uint64(tgc.userID), nil)
	if callbackID != "" {
		_ = r.send.AnswerCallbackText(ctx, callbackID, text)
	} else {
		r.sendText(ctx, tgc.chatID, text)
	}
	return true
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
	first, second := catB, catA
	if len(outcome.Result.Turns) > 0 {
		if outcome.Result.Turns[0].AttackerCatID == catA.ID {
			first, second = catA, catB
		}
	}
	data := fightTextData{
		AttackerName: first.Name, DefenderName: second.Name, FirstName: first.Name, SecondName: second.Name,
		WinnerName: winner.Name, LoserName: loser.Name, WinnerHP: winnerHP,
		Rounds: outcome.Result.Rounds, Rivalry: outcome.Rivalry,
		CatAName: catA.Name, CatBName: catB.Name,
		CatAWins: outcome.Stats.CatAWins, CatALosses: outcome.Stats.CatALosses,
		CatBWins: outcome.Stats.CatBWins, CatBLosses: outcome.Stats.CatBLosses,
		PairCatAWins: outcome.Stats.PairCatAWins, PairCatBWins: outcome.Stats.PairCatBWins,
		WinnerXPGain: outcome.WinnerXPGain, LoserXPGain: outcome.LoserXPGain,
	}
	selector := outcome.Seed
	initiativeKey := "fight.initiative.speed"
	if catA.PhysicalStats().TailMM == catB.PhysicalStats().TailMM {
		initiativeKey = "fight.initiative.coin"
	}
	startKey := "fight.start"
	startData := data
	if outcome.Kind == domain.FightKindRevenge {
		startKey = "fight.revenge.start"
		startData.AttackerName = catB.Name
		startData.DefenderName = catA.Name
	}
	parts := []string{
		gamecontent.Render("fight.title", selector, data),
		gamecontent.Render(startKey, selector, startData),
		gamecontent.Render(initiativeKey, selector, data),
	}
	if turns := formatFightTurns(outcome, catA, catB); turns != "" {
		parts = append(parts, turns)
	}
	resultKey := "fight.result.win"
	if winnerHP <= 5 {
		resultKey = "fight.result.close"
	}
	parts = append(parts, gamecontent.Render(resultKey, selector+1, data))
	parts = append(parts, gamecontent.Render("fight.result.xp", selector, data))
	if winner.Level < loser.Level {
		parts = append(parts, gamecontent.Render("fight.result.upset", selector+2, data))
	}
	parts = append(parts, gamecontent.Render("fight.result.stats", selector, data))
	if note := views.FormatProgressionFacts(outcome.CatAFacts, ""); note != "" {
		parts = append(parts, catA.Name+":"+note)
	}
	if note := views.FormatProgressionFacts(outcome.CatBFacts, ""); note != "" {
		parts = append(parts, catB.Name+":"+note)
	}
	return strings.Join(parts, "\n\n")
}

func formatFightTurns(outcome app.FightOutcome, catA, catB *domain.Cat) string {
	turns := outcome.Result.Turns
	if len(turns) == 0 {
		return ""
	}
	indices := selectFightTurnIndices(turns, fightNarrativeTurnLimit)
	lines := make([]string, 0, len(indices)+1)
	previous := -1
	for _, index := range indices {
		if previous >= 0 && index > previous+1 {
			lines = append(lines, gamecontent.Render("fight.turns.skipped", outcome.Seed+uint64(index), fightTextData{Omitted: index - previous - 1}))
		}
		turn := turns[index]
		attacker, defender := fightCatsForTurn(turn, catA, catB)
		data := fightTextData{
			AttackerName: attacker.Name, DefenderName: defender.Name,
			Damage: turn.Damage, DefenderHP: turn.DefenderHPAfter, Round: turn.Round,
		}
		key := "fight.turn.attack"
		switch {
		case turn.Damage == 0:
			key = "fight.turn.dodge"
		case turn.DefenderHPAfter == 0 && turn.Crit:
			key = "fight.turn.finish_crit"
		case turn.DefenderHPAfter == 0:
			key = "fight.turn.finish"
		case turn.Crit:
			key = "fight.turn.crit"
		case turn.Damage <= 2:
			key = "fight.turn.glancing"
		}
		turnSelector := outcome.Seed + uint64(index+1)*97 + uint64(turn.AttackerCatID)
		lines = append(lines, gamecontent.Render(key, turnSelector, data))
		previous = index
	}
	return strings.Join(lines, "\n")
}

func fightCatsForTurn(turn domain.FightTurn, catA, catB *domain.Cat) (*domain.Cat, *domain.Cat) {
	if turn.AttackerCatID == catA.ID {
		return catA, catB
	}
	return catB, catA
}

func selectFightTurnIndices(turns []domain.FightTurn, limit int) []int {
	if limit <= 0 || len(turns) <= limit {
		indices := make([]int, len(turns))
		for index := range turns {
			indices[index] = index
		}
		return indices
	}
	selected := make(map[int]struct{}, limit)
	add := func(index int) {
		if index >= 0 && index < len(turns) && len(selected) < limit {
			selected[index] = struct{}{}
		}
	}
	add(0)
	add(1)
	add(len(turns) - 3)
	add(len(turns) - 2)
	add(len(turns) - 1)
	for index, turn := range turns {
		if turn.Crit {
			add(index)
		}
	}
	for index := 2; index < len(turns)-3 && len(selected) < limit; index++ {
		add(index)
	}
	indices := make([]int, 0, len(selected))
	for index := range selected {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	return indices
}
