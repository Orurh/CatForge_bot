package telegram

import (
	"catforge/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestYardEventScheduleExplainsBothGates(t *testing.T) {
	now := time.Date(2026, 9, 9, 14, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		cats int
		next time.Time
		want string
	}{
		{2, now.Add(time.Hour), "09.09 в 18:00 МСК"},
		{1, now.Add(time.Hour), "Пусть второй хозяин"},
		{1, now.Add(-time.Hour), "Таймер уже прошёл"},
		{2, now.Add(-time.Hour), "Условия выполнены"},
	} {
		got := formatYardEventSchedule(domain.YardEventSchedule{ActiveCats: tc.cats, NextAt: tc.next}, now)
		if !strings.Contains(got, tc.want) || !strings.Contains(got, "24–48") || !strings.Contains(got, "7 дней") {
			t.Errorf("missing waiting reason: %s", got)
		}
	}
}
