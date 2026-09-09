package domain

import "time"

type User struct {
	ID         int64
	TelegramID int64
}

type Breed string
type Trait string
type HumorMode string

const (
	TraitLazy        Trait = "lazy"
	TraitBully       Trait = "bully"
	TraitPhilosopher Trait = "philosopher"
	TraitNeat        Trait = "neat"
	TraitSleepy      Trait = "sleepy"
)

const (
	HumorNormal HumorMode = "normal"
	HumorBold   HumorMode = "bold"
)

var AllTraits = [...]Trait{
	TraitLazy,
	TraitBully,
	TraitPhilosopher,
	TraitNeat,
	TraitSleepy,
}

func IsValidTrait(trait Trait) bool {
	for _, candidate := range AllTraits {
		if trait == candidate {
			return true
		}
	}
	return false
}

func IsValidHumorMode(mode HumorMode) bool {
	return mode == HumorNormal || mode == HumorBold
}

const (
	BreedMaineCoon Breed = "maine_coon"
	BreedSiamese   Breed = "siamese"
	BreedBritish   Breed = "british"
	BreedBengal    Breed = "bengal"
)

type Cat struct {
	TrainingCritBonusPercent int // transient equipped-item bonus, never persisted as a stat
	Feline                   FelineStats
	FirstItemGranted         bool
	LootItemID               string   // transient reward, persisted atomically with the action
	ProgressionFacts         []string // transient facts returned by the authoritative engine

	ID              int64
	UserID          int64
	StateVersion    int64
	Name            string
	Breed           Breed
	Trait           Trait
	Level           int
	XP              int64
	Coins           int64
	Energy          int
	LastTrainAt     time.Time
	EnergyUpdatedAt time.Time
	HPBase          int
	ATKBase         int
	DEFBase         int
	SPDBase         int
}

type CatPersonality struct {
	CatID            int64
	Trait            Trait
	SpeechStyle      string
	HumorMode        HumorMode
	AutoSpeakEnabled bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func DefaultSpeechStyle(trait Trait) string {
	switch trait {
	case TraitLazy:
		return "сухой юмор, минимум энтузиазма, избегает лишней работы"
	case TraitBully:
		return "самоуверенно поддевает, спорит и редко признаёт вину"
	case TraitPhilosopher:
		return "видит великий смысл в бытовых мелочах и говорит с невозмутимым пафосом"
	case TraitNeat:
		return "аккуратен, придирчив к порядку и слегка осуждает чужой бардак"
	case TraitSleepy:
		return "сонный, медленный и любую тему сводит к отдыху"
	default:
		return "коротко, по-кошачьи и с добродушной иронией"
	}
}
