package telegram

import (
	"context"
	"strings"
	"time"

	"catforge/internal/app"
	"catforge/internal/domain"
	"catforge/internal/gamecontent"
	"catforge/internal/logx"
	"catforge/internal/transport/telegram/narrative"
	"catforge/internal/transport/telegram/views"
)

type TelegramGameEventSink struct {
	s    *Sender
	refs app.CatMessageReferenceRepository
	log  logx.Logger
}

func NewTelegramGameEventSink(s *Sender, refs app.CatMessageReferenceRepository, log logx.Logger) *TelegramGameEventSink {
	if log == nil {
		log = logx.Nop()
	}
	return &TelegramGameEventSink{s: s, refs: refs, log: log}
}

func (l *TelegramGameEventSink) Publish(ctx context.Context, event app.GameEvent) error {
	switch event.Kind {
	case app.GameEventCatTrained:
		payload, ok := event.Payload.(app.CatTrainedPayload)
		if !ok {
			return nil
		}
		return l.training(ctx, payload, event.OccurredAt)
	case app.GameEventExpeditionFinished:
		payload, ok := event.Payload.(app.ExpeditionFinishedPayload)
		if !ok {
			return nil
		}
		return l.expedition(ctx, payload)
	case app.GameEventYardEventResolved:
		payload, ok := event.Payload.(app.YardEventResolvedPayload)
		if !ok {
			return nil
		}
		return l.yardEventResolved(ctx, payload)
	case app.GameEventYardEventStarted:
		payload, ok := event.Payload.(app.YardEventStartedPayload)
		if !ok || payload.TelegramChatID == 0 {
			return nil
		}
		return l.yardEventStarted(ctx, payload)
	case app.GameEventAutonomousCatMessage:
		payload, ok := event.Payload.(app.AutonomousCatMessagePayload)
		if !ok || payload.TelegramChatID == 0 || strings.TrimSpace(payload.Text) == "" {
			return nil
		}
		text := publicCatBadge(payload.Breed, payload.Level) + " " + payload.CatName + ": " + strings.TrimSpace(payload.Text)
		messageID, err := l.s.TextResult(ctx, payload.TelegramChatID, text)
		if err != nil {
			return err
		}
		now := event.OccurredAt
		if now.IsZero() {
			now = time.Now()
		}
		if l.refs != nil && event.CatID > 0 {
			if err := l.refs.Remember(ctx, payload.TelegramChatID, messageID, event.CatID, now.Add(app.CatMessageReferenceTTL)); err != nil {
				l.log.Warn("cat message reference save failed", logx.Int64("chat_id", payload.TelegramChatID), logx.Int("message_id", messageID), logx.Any("err", err))
			}
		}
		return nil
	case app.GameEventCatBanter:
		payload, ok := event.Payload.(app.CatBanterPayload)
		if !ok || payload.TelegramChatID == 0 || strings.TrimSpace(payload.FirstLine) == "" || strings.TrimSpace(payload.SecondLine) == "" {
			return nil
		}
		text := publicCatBadge(payload.FirstCatBreed, payload.FirstCatLevel) + " " + payload.FirstCatName + ": " + strings.TrimSpace(payload.FirstLine) +
			"\n\n" + publicCatBadge(payload.SecondCatBreed, payload.SecondCatLevel) + " " + payload.SecondCatName + ": " + strings.TrimSpace(payload.SecondLine)
		return l.s.Text(ctx, payload.TelegramChatID, text)
	default:
		return nil
	}
}

func (l *TelegramGameEventSink) yardEventStarted(ctx context.Context, payload app.YardEventStartedPayload) error {
	event := &domain.YardEvent{
		ID: payload.EventID, Type: domain.YardEventType(payload.EventType), State: domain.YardEventActive,
		Seed: payload.Seed, StartsAt: payload.StartsAt, ResolvesAt: payload.ResolvesAt,
	}
	status := app.YardEventStatus{Event: event, Counts: map[domain.YardEventChoiceID]int{}}
	if err := l.s.TextWithKeyboard(ctx, payload.TelegramChatID, formatYardEvent(status, payload.StartsAt), YardEventKeyboard(payload.EventID, event.Type)); err != nil {
		l.log.Error("yard event start send failed", logx.Any("err", err))
		return err
	}
	return nil
}

func (l *TelegramGameEventSink) yardEventResolved(ctx context.Context, event app.YardEventResolvedPayload) error {
	if event.TelegramChatID == 0 {
		return nil
	}
	text := formatYardEventResult(event)
	if err := l.s.Text(ctx, event.TelegramChatID, text); err != nil {
		l.log.Error("yard event result send failed", logx.Any("err", err))
		return err
	}
	return nil
}

type yardEventResultTextData struct {
	Badge         string
	CatName       string
	Choice        string
	Contribution  int
	MVPMark       string
	TeamScore     int
	TargetScore   int
	YardScore     int
	XPGain        int64
	StrategyBonus int
	Friendship    int
	Rivalry       int
	Respect       int
}

func formatYardEventResult(event app.YardEventResolvedPayload) string {
	result := event.Result
	selector := uint64(event.EventID)
	eventType := yardEventTypeOrDefault(domain.YardEventType(event.EventType))
	headlineKey := yardEventContentKey(eventType, "result.headline."+string(result.OutcomeTier))
	parts := []string{gamecontent.Render(headlineKey, selector, nil)}
	if strings.TrimSpace(event.Narrative) != "" {
		parts = append(parts, strings.TrimSpace(event.Narrative))
	}
	data := yardEventResultTextData{
		TeamScore: result.TeamScore, TargetScore: result.TargetScore,
		YardScore: result.YardScore, XPGain: result.XPGain,
	}
	parts = append(parts, gamecontent.Render("yard_event.result.summary", selector, data))
	if result.StrategyBonus > 0 {
		data.StrategyBonus = result.StrategyBonus
		parts = append(parts, gamecontent.Render("yard_event.result.strategy", selector, data))
	}
	if result.SecretFound {
		parts = append(parts, gamecontent.Render(yardEventContentKey(eventType, "result.secret"), selector, nil))
	}
	if len(event.Participants) > 0 {
		participantLines := []string{gamecontent.Render("yard_event.result.participants", selector, nil)}
		for _, participant := range event.Participants {
			participantData := yardEventResultTextData{
				Badge: publicCatBadge(participant.Breed, participant.Level), CatName: participant.CatName,
				Choice: yardEventChoiceName(eventType, participant.Choice), Contribution: participant.Contribution,
			}
			if participant.MVP {
				participantData.MVPMark = gamecontent.Render("yard_event.result.mvp_mark", selector, nil)
			}
			participantLines = append(participantLines, gamecontent.Render("yard_event.result.participant", selector+uint64(participant.CatID), participantData))
		}
		parts = append(parts, strings.Join(participantLines, "\n"))
	}
	for _, reward := range result.Participants {
		if note := views.FormatProgressionFacts(reward.ProgressionFacts, reward.LootItemID); note != "" {
			name := "Кот"
			for _, p := range event.Participants {
				if p.CatID == reward.CatID {
					name = p.CatName
					break
				}
			}
			parts = append(parts, name+":"+note)
		}
	}
	for _, effect := range result.RelationshipEffects {
		data.Friendship += effect.FriendshipDelta
		data.Rivalry += effect.RivalryDelta
		data.Respect += effect.RespectDelta
	}
	if data.Friendship > 0 || data.Rivalry > 0 || data.Respect > 0 {
		parts = append(parts, gamecontent.Render("yard_event.result.relationships", selector, data))
	}
	parts = append(parts, gamecontent.Render("yard_event.result.reward_note", selector, nil))
	return strings.Join(parts, "\n\n")
}

func yardEventChoiceName(eventType domain.YardEventType, choice domain.YardEventChoiceID) string {
	key := yardEventContentKey(eventType, "choice."+string(choice))
	if !gamecontent.Has(key) {
		return string(choice)
	}
	return gamecontent.Render(key, 0, nil)
}

func (l *TelegramGameEventSink) expedition(ctx context.Context, e app.ExpeditionFinishedPayload) error {
	if e.TargetChatID == 0 {
		return nil
	}
	line := publicCatBadge(e.Breed, e.Level) + " " + views.FormatExpeditionResultText(&domain.Cat{Name: e.CatName, Trait: e.Trait}, e.Result)
	if err := l.s.Text(ctx, e.TargetChatID, line); err != nil {
		l.log.Error("expedition eventlog send failed", logx.Any("err", err))
		return err
	}
	return nil
}

func (l *TelegramGameEventSink) training(ctx context.Context, e app.CatTrainedPayload, now time.Time) error {
	if e.TargetChatID == 0 {
		return nil
	}
	badge := publicCatBadge(e.Breed, e.Level)

	name := e.CatName
	if name == "" {
		name = "Кот"
	}
	prefix := badge + " " + name + ": "
	var line string
	switch e.Result.Outcome {
	case domain.TrainingNotEnoughEnergy:
		line = badge + " " + views.EnergyTrainingStatus(name, e.Energy, e.EnergyUpdatedAt, now)
	default:
		story := strings.TrimSpace(e.Narrative)
		if story == "" {
			story = narrative.HuntStory(e.Result.Encounter, e.Trait, e.Result.Flavor)
		}
		line = prefix + story + "\n⚡ −" + itoa(e.Result.EnergyCost) + " энергии" +
			" · осталось " + itoa(e.Energy) + "/100\n⭐ +" + itoa64(e.Result.XPGain) + " XP"

		if e.Result.Crit {
			line = "🔥 КРИТИЧЕСКАЯ ТРЕНИРОВКА!\n" + line + " · +100% базового XP за крит"
		}
		if e.Result.LeveledUp > 0 {
			line += " → уровень " + itoa(e.Level)
		}
	}

	line += views.FormatProgressionFacts(e.ProgressionFacts, e.LootItemID)
	if e.Result.Outcome == domain.TrainingOK {
		line += "\n\n" + views.EnergyRecoveryText(name, e.Energy, e.EnergyUpdatedAt, now)
	}
	if err := l.s.Text(ctx, e.TargetChatID, line); err != nil {
		l.log.Error("eventlog send failed", logx.Any("err", err))
		return err
	}
	return nil
}
