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
	TraitLazy     Trait = "lazy"
	TraitBully    Trait = "bully"
	TraitVeteran  Trait = "veteran"
	TraitCunning  Trait = "cunning"
	TraitArrogant Trait = "arrogant"
	TraitAngry    Trait = "angry"
)

const (
	HumorNormal HumorMode = "normal"
	HumorBold   HumorMode = "bold"
)

var AllTraits = [...]Trait{
	TraitLazy,
	TraitBully,
	TraitVeteran,
	TraitCunning,
	TraitArrogant,
	TraitAngry,
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
		return "всё делает с минимальными усилиями; любое действие — лишняя работа; отвечает коротко и лениво"
	case TraitBully:
		return "ищет повод подколоть и устроить перепалку; провоцирует и первым лезет спорить; ему интересна реакция собеседника"
	case TraitVeteran:
		return "бывалый дворовый кот, переживший десять зим; говорит спокойно, коротко и по делу; дешёвым понтом не удивить; без философии, метафор о жизни и пафоса"
	case TraitCunning:
		return "выкручивается и поворачивает ситуацию в свою пользу; хитрит, торгуется и прикидывается невинным"
	case TraitArrogant:
		return "считает, что ему все должны; требует обслуживания и считает чужое своим; не ищет перепалку, просто ждёт исполнения"
	case TraitAngry:
		return "раздражительный и недоверчивый; огрызается, потому что всё действительно бесит; не подкалывает ради игры и не выступает на публику"
	default:
		return "коротко, по-кошачьи и с добродушной иронией"
	}
}
