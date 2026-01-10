package app

import (
	"time"

	"catforge/internal/domain"
)

// TrainingEvent — DTO события для публичного лога (группа/канал).
// Сформирован usecase-слоем, уже содержит "правильные" числа для отображения.
type TrainingEvent struct {
	ChatID     int64
	TelegramID int64
	CatName    string
	Breed      domain.Breed
	Now        time.Time

	Energy int // актуальная энергия на Now (после regen / после Train)
	Level  int // актуальный уровень на Now

	Result domain.TrainResult
}
