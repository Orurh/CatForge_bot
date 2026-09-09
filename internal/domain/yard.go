package domain

import "time"

type Yard struct {
	ID                  int64
	TelegramChatID      int64
	Name                string
	HumorMode           HumorMode
	AutoMessagesEnabled bool
	MaxAutoMessagesDay  int
	CatToCatBanter      bool
	FightsEnabled       bool
	QuietUntil          time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type YardSettings struct {
	HumorMode           HumorMode
	AutoMessagesEnabled bool
	MaxAutoMessagesDay  int
	CatToCatBanter      bool
	FightsEnabled       bool
	QuietUntil          time.Time
}

type YardMember struct {
	YardID       int64
	UserID       int64
	CatID        int64
	CatName      string
	Breed        Breed
	Trait        Trait
	Level        int
	JoinedAt     time.Time
	LastActiveAt time.Time
}

type YardEventType string
type YardEventState string
type YardEventChoiceID string
type YardEventOutcomeTier string
type AutoMessageKind string

const (
	AutoMessageSingle AutoMessageKind = "single"
	AutoMessageBanter AutoMessageKind = "banter"
)

const (
	YardEventFishTruck YardEventType = "fish_truck"
	YardEventBigDog    YardEventType = "big_dog"
	YardEventBigBox    YardEventType = "big_box"

	YardEventActive   YardEventState = "active"
	YardEventResolved YardEventState = "resolved"

	YardChoiceSteal    YardEventChoiceID = "steal"
	YardChoiceDistract YardEventChoiceID = "distract"
	YardChoiceScout    YardEventChoiceID = "scout"

	YardOutcomeFailure     YardEventOutcomeTier = "fail"
	YardOutcomePartial     YardEventOutcomeTier = "partial"
	YardOutcomeSuccess     YardEventOutcomeTier = "success"
	YardOutcomeExceptional YardEventOutcomeTier = "exceptional"
)

func (tier YardEventOutcomeTier) IsSuccess() bool {
	return tier == YardOutcomeSuccess || tier == YardOutcomeExceptional
}

var AllYardEventTypes = [...]YardEventType{
	YardEventFishTruck,
	YardEventBigDog,
	YardEventBigBox,
}

func NextYardEventType(previous YardEventType) YardEventType {
	for index, candidate := range AllYardEventTypes {
		if candidate == previous {
			return AllYardEventTypes[(index+1)%len(AllYardEventTypes)]
		}
	}
	return YardEventFishTruck
}

var AllYardEventChoices = [...]YardEventChoiceID{
	YardChoiceSteal,
	YardChoiceDistract,
	YardChoiceScout,
}

func IsValidYardEventChoice(choice YardEventChoiceID) bool {
	if _, ok := SpecialActionByID(string(choice)); ok {
		return true
	}
	for _, candidate := range AllYardEventChoices {
		if choice == candidate {
			return true
		}
	}
	return false
}

type YardEvent struct {
	ID             int64
	YardID         int64
	Type           YardEventType
	State          YardEventState
	Seed           int64
	StartsAt       time.Time
	ResolvesAt     time.Time
	ContentVersion uint32
	CreatedAt      time.Time
}

type YardEventChoice struct {
	EventID     int64
	CatID       int64
	ChoiceID    YardEventChoiceID
	SubmittedAt time.Time
}

type YardEventParticipant struct {
	Feline        FelineStats
	Effects       []string
	SpecialAction string

	CatID            int64
	CatName          string
	Breed            Breed
	Trait            Trait
	SpeechStyle      string
	HumorMode        HumorMode
	AutoSpeakEnabled bool
	Choice           YardEventChoiceID
	Level            int
	HP               int
	ATK              int
	DEF              int
	SPD              int
}

type YardEventParticipantResult struct {
	ItemEffectTriggered bool     `json:"item_effect_triggered,omitempty"`
	LootItemID          string   `json:"loot_item_id,omitempty"`
	ProgressionFacts    []string `json:"progression_facts,omitempty"`

	CatID        int64             `json:"cat_id"`
	Choice       YardEventChoiceID `json:"choice"`
	Contribution int               `json:"contribution"`
	MVP          bool              `json:"mvp"`
}

type YardRelationshipEffect struct {
	CatAID          int64 `json:"cat_a_id"`
	CatBID          int64 `json:"cat_b_id"`
	FriendshipDelta int   `json:"friendship_delta"`
	RivalryDelta    int   `json:"rivalry_delta"`
	RespectDelta    int   `json:"respect_delta"`
}

type CatRelationship struct {
	CatAID     int64
	CatBID     int64
	CatAName   string
	CatBName   string
	Friendship int
	Rivalry    int
	Respect    int
	UpdatedAt  time.Time
}

type YardEventResult struct {
	OutcomeTier         YardEventOutcomeTier         `json:"outcome_tier"`
	TeamScore           int                          `json:"team_score"`
	TargetScore         int                          `json:"target_score"`
	YardScore           int                          `json:"yard_score"`
	XPGain              int64                        `json:"xp_gain"`
	SecretFound         bool                         `json:"secret_found"`
	StrategyBonus       int                          `json:"strategy_bonus"`
	Participants        []YardEventParticipantResult `json:"participants"`
	RelationshipEffects []YardRelationshipEffect     `json:"relationship_effects"`
}

type YardEventResolutionInput struct {
	Event          YardEvent
	TelegramChatID int64
	Participants   []YardEventParticipant
}

type YardWeeklyCatStats struct {
	AvailableEvents     *int   `json:"available_events"`
	CatID               int64  `json:"cat_id"`
	CatName             string `json:"cat_name"`
	Breed               Breed  `json:"breed"`
	Trait               Trait  `json:"trait"`
	EventsParticipated  int    `json:"event_participation"`
	StealChoices        int    `json:"steal_choices"`
	DistractChoices     int    `json:"distract_choices"`
	ScoutChoices        int    `json:"scout_choices"`
	Contribution        int    `json:"event_contribution"`
	MVPCount            int    `json:"mvp"`
	EventXP             int64  `json:"event_xp"`
	TrainingEnergySpent int    `json:"training_energy_spent"`
	TrainingXP          int64  `json:"training_xp"`
	Fights              int    `json:"fights"`
	Wins                int    `json:"wins"`
	Losses              int    `json:"losses"`
	WinStreak           int    `json:"win_streak"`
	RivalryGained       int    `json:"rivalry_gained"`
	ArenaPoints         int    `json:"arena_points"`
	YardPoints          int    `json:"yard_points"`
	TrainingPoints      int    `json:"training_points"`
	TotalPoints         int    `json:"total_points"`
	WeeklyTitle         string `json:"weekly_title,omitempty"`
}

// YardWeeklySummary contains only persisted, authoritative event results.
// Presentation and AI may describe these facts, but must not derive new game state.
type YardWeeklySummary struct {
	YardID             int64
	TelegramChatID     int64
	YardName           string
	PeriodStart        time.Time
	PeriodEnd          time.Time
	EventsResolved     int
	FailedEvents       int
	PartialEvents      int
	SuccessfulEvents   int
	ExceptionalEvents  int
	TotalChoices       int
	UniqueParticipants int
	YardScore          int
	SecretsFound       int
	Cats               []YardWeeklyCatStats
}

// EventOutcomeTier is the canonical progression result tier.
type EventOutcomeTier = YardEventOutcomeTier
