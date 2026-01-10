package domain

import "time"

type User struct {
	ID         int64
	TelegramID int64
}

type Breed string
type Trait string

const (
	BreedMaineCoon Breed = "maine_coon"
	BreedSiamese   Breed = "siamese"
	BreedBritish   Breed = "british"
	BreedBengal    Breed = "bengal"
)

type Cat struct {
	ID              int64
	UserID          int64
	Name            string
	Breed           Breed
	Trait           Trait
	Level           int
	XP              int64
	Energy          int
	LastTrainAt     time.Time
	EnergyUpdatedAt time.Time
	HPBase          int
	ATKBase         int
	DEFBase         int
	SPDBase         int
}
