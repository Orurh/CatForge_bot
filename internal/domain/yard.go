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
	QuietUntil          time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type YardSettings struct {
	HumorMode           HumorMode
	AutoMessagesEnabled bool
	MaxAutoMessagesDay  int
	CatToCatBanter      bool
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

const (
	YardEventFishTruck YardEventType = "fish_truck"

	YardEventActive   YardEventState = "active"
	YardEventResolved YardEventState = "resolved"

	YardChoiceSteal    YardEventChoiceID = "steal"
	YardChoiceDistract YardEventChoiceID = "distract"
	YardChoiceScout    YardEventChoiceID = "scout"
)

var AllYardEventChoices = [...]YardEventChoiceID{
	YardChoiceSteal,
	YardChoiceDistract,
	YardChoiceScout,
}

func IsValidYardEventChoice(choice YardEventChoiceID) bool {
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
	CatID            int64
	CatName          string
	Breed            Breed
	Trait            Trait
	SpeechStyle      string
	AutoSpeakEnabled bool
	Choice           YardEventChoiceID
	Level            int
	HP               int
	ATK              int
	DEF              int
	SPD              int
}

type YardEventParticipantResult struct {
	CatID        int64
	Choice       YardEventChoiceID
	Contribution int
	FishReward   int
	MVP          bool
}

type YardRelationshipEffect struct {
	CatAID          int64
	CatBID          int64
	FriendshipDelta int
	RivalryDelta    int
	RespectDelta    int
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
	Success             bool
	TeamScore           int
	TargetScore         int
	FishTotal           int
	SecretFound         bool
	StrategyBonus       int
	Participants        []YardEventParticipantResult
	RelationshipEffects []YardRelationshipEffect
}

type YardEventResolutionInput struct {
	Event          YardEvent
	TelegramChatID int64
	Participants   []YardEventParticipant
}

type YardWeeklyCatStats struct {
	CatID              int64
	CatName            string
	Breed              Breed
	Trait              Trait
	EventsParticipated int
	StealChoices       int
	DistractChoices    int
	ScoutChoices       int
	Contribution       int
	FishReward         int
	MVPCount           int
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
	SuccessfulEvents   int
	TotalChoices       int
	UniqueParticipants int
	FishTotal          int
	SecretsFound       int
	Cats               []YardWeeklyCatStats
}
