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


type DailyClaimEvent struct {
    ChatID     int64
    TelegramID int64
    CatName    string
    Breed      domain.Breed
    Now        time.Time

    Streak     int
    XPGain     int64
    EnergyGain int
}

type ArenaFightEvent struct {
    ChatID     int64
    TelegramID int64
    CatName    string
    Breed      domain.Breed
    Level      int
    Now        time.Time

    OpponentName string
    OpponentBreed domain.Breed
    OpponentLevel int
    OpponentPower int

    AttackerPower int
    WinProb       int 
    Won           bool

    RatingDelta int
    NewRating   int

    XPGain     int64
    LeveledUp  int
    RageAfter  int
}
