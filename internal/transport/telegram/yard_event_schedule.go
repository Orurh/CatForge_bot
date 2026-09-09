package telegram

import (
	"catforge/internal/domain"
	"fmt"
	"strings"
	"time"
)

func formatYardEventSchedule(schedule domain.YardEventSchedule, now time.Time) string {
	lines := []string{"Во Дворе пока нет события.", fmt.Sprintf("🐾 Активных котов за последние 7 дней: %d. Нужно минимум 2.", schedule.ActiveCats)}
	if schedule.ActiveCats < 2 {
		lines = append(lines, "Пусть второй хозяин вызовет /event или /train со своим котом именно в этой группе. Просто создать кота в личке недостаточно.")
	}
	if schedule.NextAt.After(now) {
		lines = append(lines, "⏳ Ближайший автозапуск — не раньше "+schedule.NextAt.In(time.FixedZone("МСК", 3*60*60)).Format("02.01 в 15:04")+" МСК, если будут активны два кота.")
	} else if schedule.ActiveCats >= 2 {
		lines = append(lines, "Условия выполнены. Событие ожидает автоматического запуска; обычно это занимает около минуты.")
	} else {
		lines = append(lines, "Таймер уже прошёл — осталось собрать двух активных котов.")
	}
	lines = append(lines, "Первое событие приходит через 24–48 часов после создания Двора, следующие — через 24–48 часов после окончания предыдущего. /event показывает статус, а не запускает событие.")
	return strings.Join(lines, "\n\n")
}
