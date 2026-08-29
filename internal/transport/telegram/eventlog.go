package telegram

import (
	"context"
	"strings"

	"catforge/internal/app"
	"catforge/internal/domain"
	"catforge/internal/logx"
	"catforge/internal/transport/telegram/narrative"
	"catforge/internal/transport/telegram/views"
)

type TelegramGameEventSink struct {
	s   *Sender
	log logx.Logger
}

func NewTelegramGameEventSink(s *Sender, log logx.Logger) *TelegramGameEventSink {
	if log == nil {
		log = logx.Nop()
	}
	return &TelegramGameEventSink{s: s, log: log}
}

func (l *TelegramGameEventSink) Publish(ctx context.Context, event app.GameEvent) error {
	switch event.Kind {
	case app.GameEventCatTrained:
		payload, ok := event.Payload.(app.CatTrainedPayload)
		if !ok {
			return nil
		}
		return l.training(ctx, payload)
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
	case app.GameEventAutonomousCatMessage:
		payload, ok := event.Payload.(app.AutonomousCatMessagePayload)
		if !ok || payload.TelegramChatID == 0 || strings.TrimSpace(payload.Text) == "" {
			return nil
		}
		text := publicCatBadge(payload.Breed, payload.Level) + " " + payload.CatName + ": " + strings.TrimSpace(payload.Text)
		return l.s.Text(ctx, payload.TelegramChatID, text)
	default:
		return nil
	}
}

func (l *TelegramGameEventSink) yardEventResolved(ctx context.Context, event app.YardEventResolvedPayload) error {
	if event.TelegramChatID == 0 {
		return nil
	}
	result := event.Result
	headline := "😿 Машина с рыбой уехала раньше, чем коты завершили операцию."
	if result.Success {
		headline = "🐟 Операция «Машина с рыбой» удалась!"
	}
	text := headline + "\n\nСила команды: " + itoa(result.TeamScore) + "/" + itoa(result.TargetScore) +
		"\nОбщий улов: " + itoa(result.FishTotal) + " рыб"
	if strings.TrimSpace(event.Narrative) != "" {
		text = headline + "\n\n" + strings.TrimSpace(event.Narrative) +
			"\n\nСила команды: " + itoa(result.TeamScore) + "/" + itoa(result.TargetScore) +
			"\nОбщий улов: " + itoa(result.FishTotal) + " рыб"
	}
	if result.StrategyBonus > 0 {
		text += "\nБонус за сочетание ролей: +" + itoa(result.StrategyBonus)
	}
	if result.SecretFound {
		text += "\n✨ Разведчики нашли спрятанный ящик!"
	}
	if len(result.RelationshipEffects) > 0 {
		text += "\n🐾 Событие изменило отношения между котами."
	}
	for _, participant := range result.Participants {
		if participant.MVP {
			text += "\n🏆 MVP операции — кот №" + itoa64(participant.CatID)
			break
		}
	}
	if err := l.s.Text(ctx, event.TelegramChatID, text); err != nil {
		l.log.Error("yard event result send failed", logx.Any("err", err))
		return err
	}
	return nil
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

func (l *TelegramGameEventSink) training(ctx context.Context, e app.CatTrainedPayload) error {
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
		line = prefix + "собрался тренироваться, но энергии мало (" + itoa(e.Energy) + "/100; минимум " + itoa(domain.TrainingMinEnergy) + ")."
	default:
		story := strings.TrimSpace(e.Narrative)
		if story == "" {
			story = narrative.HuntStory(e.Result.Encounter, e.Trait, e.Result.Flavor)
		}
		line = prefix + story + "\n⚡ −" + itoa(e.Result.EnergyCost) + " энергии" +
			" · осталось " + itoa(e.Energy) + "/100\n⭐ +" + itoa64(e.Result.XPGain) + " XP" +
			"\n🪙 +" + itoa64(e.Result.CoinsGain) + " монет"

		if e.Result.Crit {
			line += " ✨ КРИТ"
		}
		if e.Result.LeveledUp > 0 {
			line += " → уровень " + itoa(e.Level)
		}
	}

	if err := l.s.Text(ctx, e.TargetChatID, line); err != nil {
		l.log.Error("eventlog send failed", logx.Any("err", err))
		return err
	}
	return nil
}
