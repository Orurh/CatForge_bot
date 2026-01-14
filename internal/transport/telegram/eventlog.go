package telegram

import (
	"context"
	"fmt"

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
		line = prefix + "энергии мало. E: " + itoa(e.Energy) + "/100 (нужно 25)."

	default:
		story := narrative.HuntStory(e.Result.Encounter, e.Result.Flavor)
		line = prefix + story + ": +" + itoa64(e.Result.XPGain) + " XP" +
			" • E: " + itoa(e.Energy) + "/100 (-" + itoa(e.Result.EnergyCost) + ")" +
			" • Форма: " + itoa(e.Result.EffPercent) + "% (" + fmtMul(e.Result.EffPercent) + ")"
 


		if e.Result.Crit {
			line += " ✨КРИТ"
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

func (l *TelegramEventLog) DailyClaim(ctx context.Context, e app.DailyClaimEvent) error {
    badge := publicCatBadge(e.Breed, 1)
    name := e.CatName
    if name == "" { name = "Кот" }
    line := "🎁 " + badge + " " + name +
        " получает ежедневку: +" + itoa64(e.XPGain) + " XP, +" + itoa(e.EnergyGain) +
        " энергии • серия " + itoa(e.Streak) + " дн."
    if err := l.s.Text(ctx, e.ChatID, line); err != nil {
        l.log.Error("eventlog send failed", logx.Any("err", err))
        return err
    }
    return nil
}

func (l *TelegramEventLog) ArenaFight(ctx context.Context, e app.ArenaFightEvent) error {
    rel := "⚔️ равный"
    d := e.AttackerPower - e.OpponentPower
    switch {
    case d >= 120:
        rel = "✅ слабее"
    case d >= 40:
        rel = "🙂 чуть слабее"
    case d <= -120:
        rel = "💀 сильнее"
    case d <= -40:
        rel = "😬 чуть сильнее"
    }
    lvl := e.Level
    if lvl <= 0 {
        lvl = 1
    }
    badge := publicCatBadge(e.Breed, lvl)
    name := e.CatName
    if name == "" { name = "Кот" }
    outcome := "❌ поражение"
    if e.Won { outcome = "✅ победа" }

    opp := e.OpponentName
    if opp == "" {
        opp = "цель"
    }

	rd := fmt.Sprintf("%+d", e.RatingDelta)
    riskMul := "x1.00"
    if e.RiskMulPct > 0 {
        riskMul = fmt.Sprintf("x%.2f", float64(e.RiskMulPct)/100.0)
    }
    sp := ""
    if e.SeasonDelta != 0 {
        sp = " • сезон +" + itoa(e.SeasonDelta)
    }


    line := "🏟️ " + badge + " " + name + " vs " + opp + ": " + outcome +
        " • +" + itoa64(e.XPGain) + " XP" +
        " • рейтинг " + itoa(e.NewRating) + " (" + rd + ")" +
        " • " + rel +
        " • риск " + riskMul +
        sp +
        " • ярость " + itoa(e.RageAfter)
        

    if e.LeveledUp > 0 && lvl > 1 {
        line += " → уровень " + itoa(lvl)
    }

    if err := l.s.Text(ctx, e.ChatID, line); err != nil {
        l.log.Error("eventlog send failed", logx.Any("err", err))
        return err
    }
    return nil
}
