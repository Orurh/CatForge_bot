package domain

type ExpeditionOutcome int

const (
	ExpeditionVictory ExpeditionOutcome = iota
	ExpeditionDefeat
	ExpeditionNotEnoughEnergy
)

func ExpeditionCost(difficulty ExpeditionDifficulty) int {
	switch difficulty {
	case ExpeditionEasy:
		return 20
	case ExpeditionNormal:
		return 30
	case ExpeditionHard:
		return 40
	default:
		return 0
	}
}

type ExpeditionLocation string

const (
	ExpeditionAlley   ExpeditionLocation = "alley"
	ExpeditionRooftop ExpeditionLocation = "rooftop"
	ExpeditionPark    ExpeditionLocation = "park"
)

type ExpeditionDifficulty string

const (
	ExpeditionEasy   ExpeditionDifficulty = "easy"
	ExpeditionNormal ExpeditionDifficulty = "normal"
	ExpeditionHard   ExpeditionDifficulty = "hard"
)

type EnemyKind string

const (
	EnemySewerRat      EnemyKind = "sewer_rat"
	EnemyStrayDog      EnemyKind = "stray_dog"
	EnemyWildLynx      EnemyKind = "wild_lynx"
	EnemyRatAccountant EnemyKind = "rat_accountant"
	EnemyCourierDog    EnemyKind = "courier_dog"
	EnemyMoonLynx      EnemyKind = "moon_lynx"
)

func IsRareEnemy(kind EnemyKind) bool {
	return kind == EnemyRatAccountant || kind == EnemyCourierDog || kind == EnemyMoonLynx
}

type BattleActor int

const (
	BattleActorCat BattleActor = iota
	BattleActorEnemy
)

type Enemy struct {
	Kind  EnemyKind
	Level int
	HP    int
	ATK   int
	DEF   int
	SPD   int
}

type BestiaryEntry struct {
	EnemyKind  EnemyKind
	Encounters int
	Victories  int
}

type BattleTurn struct {
	Round           int
	Actor           BattleActor
	Damage          int
	Crit            bool
	DefenderHPAfter int
}

// LootRoll is the deterministic engine result. ItemID and ownership fields are
// resolved by the application from the versioned item catalog and persisted
// atomically with the expedition.
type LootRoll struct {
	Dropped   bool
	Rarity    ItemRarity
	ItemIndex int
	ItemID    string
	New       bool
	Fragments int
}

type ExpeditionResult struct {
	Outcome      ExpeditionOutcome
	Location     ExpeditionLocation
	Difficulty   ExpeditionDifficulty
	Enemy        Enemy
	EnergyCost   int
	XPGain       int64
	CoinsGain    int64
	Rounds       int
	CatHPAfter   int
	EnemyHPAfter int
	LeveledUp    int
	StatsGained  StatDelta
	Turns        []BattleTurn
	Loot         LootRoll
}
