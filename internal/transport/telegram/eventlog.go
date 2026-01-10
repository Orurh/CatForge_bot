package telegram

import (
	"context"

	"catforge/internal/app"
	"catforge/internal/domain"
	"catforge/internal/logx"
	"catforge/internal/transport/telegram/narrative"
)

type TelegramEventLog struct {
	s   *Sender
	log logx.Logger
}

func NewTelegramEventLog(s *Sender, log logx.Logger) *TelegramEventLog {
	if log == nil {
		log = logx.Nop()
	}
	return &TelegramEventLog{s: s, log: log}
}

func (l *TelegramEventLog) Training(ctx context.Context, e app.TrainingEvent) error {
	badge := publicCatBadge(e.Breed, e.Level)

	name := e.CatName
	if name == "" {
		name = "Кот"
	}
	prefix := badge + " " + name + ": "
	var line string
	switch e.Result.Outcome {
	case domain.TrainingNotEnoughEnergy:
		line = prefix + "пытался пойти на охоту, но энергии мало (" + itoa(e.Energy) + "/100; нужно 25)."
	default:
		story := narrative.HuntStory(e.Result.Encounter, e.Result.Flavor)
		line = prefix + story + ": +" + itoa64(e.Result.XPGain) + " XP" +
			" (энергия " + itoa(e.Energy) + "/100, -" + itoa(e.Result.EnergyCost) + "энергии потрачено на охоту)"


		if e.Result.Crit {
			line += " ✨ КРИТ"
		}
		if e.Result.LeveledUp > 0 {
			line += " → уровень " + itoa(e.Level)
		}
	}

	if err := l.s.Text(ctx, e.ChatID, line); err != nil {
		l.log.Error("eventlog send failed", logx.Any("err", err))
		return err
	}
	return nil
}
