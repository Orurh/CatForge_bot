package ai

import (
	"context"
	"time"

	"catforge/internal/domain"
)

type GenerationType string

const (
	GenerationFirstPersonalityLine GenerationType = "first_personality_line"
	GenerationCatReply             GenerationType = "cat_reply"
	GenerationHumanReplyToCat      GenerationType = "human_reply_to_cat"
	GenerationEventNarrative       GenerationType = "event_narrative"
	GenerationAutonomousCat        GenerationType = "autonomous_cat"
	GenerationTrainingNarrative    GenerationType = "training_narrative"
	GenerationArenaBanter          GenerationType = "arena_banter"
	GenerationYardBanter           GenerationType = "yard_banter"
	GenerationWeeklySummary        GenerationType = "weekly_summary"
)

type HumorMode = domain.HumorMode

const (
	HumorNormal = domain.HumorNormal
	HumorBold   = domain.HumorBold
)

type CatContext struct {
	ID          int64
	Name        string
	Breed       domain.Breed
	Trait       domain.Trait
	SpeechStyle string
}

type EventParticipantContext struct {
	CatName      string
	Choice       string
	Contribution int
	MVP          bool
}

type EventContext struct {
	EventType     string
	OutcomeTier   string
	TeamScore     int
	TargetScore   int
	YardScore     int
	XPGain        int64
	SecretFound   bool
	StrategyBonus int
	Participants  []EventParticipantContext
}

type WeeklyCatContext struct {
	CatName             string
	EventsParticipated  int
	StealChoices        int
	DistractChoices     int
	ScoutChoices        int
	Contribution        int
	MVPCount            int
	TrainingEnergySpent int
	TrainingXP          int64
	Fights              int
	Wins                int
	Losses              int
	ArenaPoints         int
	YardPoints          int
	TrainingPoints      int
	TotalPoints         int
	WeeklyTitle         string
}

type WeeklySummaryContext struct {
	YardName           string
	EventsResolved     int
	FailedEvents       int
	PartialEvents      int
	SuccessfulEvents   int
	ExceptionalEvents  int
	TotalChoices       int
	UniqueParticipants int
	YardScore          int
	SecretsFound       int
	CatOfWeek          string
	TopTroublemaker    string
	TopScout           string
	TopDistractor      string
	Cats               []WeeklyCatContext
}

type TrainingContext struct {
	Encounter    string
	EnergyCost   int
	XPGain       int64
	CoinsGain    int64
	Critical     bool
	Level        int
	LevelsGained int
	RivalName    string
	Rivalry      int
}

// RelationshipContext contains persisted relationship totals. Generators may
// use them as narrative context but never mutate or reinterpret the values as
// new game state.
type RelationshipContext struct {
	CatAName   string
	CatBName   string
	Friendship int
	Rivalry    int
	Respect    int
}

type ArenaBanterContext struct {
	FightKind      string
	WinnerName     string
	LoserName      string
	WinnerHP       int
	Rounds         int
	Rivalry        int
	Friendship     int
	Respect        int
	LoserPairWins  int
	WinnerPairWins int
	WinnerStreak   int
	Close          bool
	Upset          bool
	FirstFight     bool
	DecidingFight  bool
}

type YardBanterContext struct {
	Trigger          string
	EventType        string
	OutcomeTier      string
	CatAChoice       string
	CatBChoice       string
	CatAContribution int
	CatBContribution int
	CatAMVP          bool
	CatBMVP          bool
	Rivalry          int
	Friendship       int
	Respect          int
}

type GenerationRequest struct {
	Type            GenerationType
	Cat             CatContext
	OtherCat        CatContext
	YardID          int64
	HumorMode       HumorMode
	UserMessage     string
	PreviousMessage string
	EventFacts      []string
	Relationships   []RelationshipContext
	Event           *EventContext
	WeeklySummary   *WeeklySummaryContext
	Training        *TrainingContext
	ArenaBanter     *ArenaBanterContext
	YardBanter      *YardBanterContext
}

type Prompt struct {
	System  string
	User    string
	Request GenerationRequest
}

type ProviderResult struct {
	Text      string
	Emotion   string
	Model     string
	TokensIn  int
	TokensOut int
	Blocked   bool
}

type Provider interface {
	Name() string
	Generate(ctx context.Context, prompt Prompt) (ProviderResult, error)
}

type Generation struct {
	Text     string
	Emotion  string
	Provider string
	Model    string
	Fallback bool
}

type UsageRecord struct {
	GenerationType GenerationType
	CatID          int64
	YardID         int64
	Provider       string
	Model          string
	TokensIn       int
	TokensOut      int
	Latency        time.Duration
	Success        bool
	Blocked        bool
	Fallback       bool
	ErrorCode      string
	CreatedAt      time.Time
}

type UsageSink interface {
	RecordAIUsage(ctx context.Context, record UsageRecord) error
}

type Generator interface {
	Generate(ctx context.Context, request GenerationRequest) (Generation, error)
	GenerateCatReply(ctx context.Context, request GenerationRequest) (Generation, error)
	GenerateEventNarrative(ctx context.Context, request GenerationRequest) (Generation, error)
	GenerateTrainingNarrative(ctx context.Context, request GenerationRequest) (Generation, error)
	GenerateArenaBanter(ctx context.Context, request GenerationRequest) (Generation, error)
	GenerateYardBanter(ctx context.Context, request GenerationRequest) (Generation, error)
	GenerateWeeklySummary(ctx context.Context, request GenerationRequest) (Generation, error)
}
