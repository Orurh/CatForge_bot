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
	GenerationEventNarrative       GenerationType = "event_narrative"
	GenerationAutonomousCat        GenerationType = "autonomous_cat"
	GenerationTrainingNarrative    GenerationType = "training_narrative"
	GenerationArenaBanter          GenerationType = "arena_banter"
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
	FishReward   int
	MVP          bool
}

type EventContext struct {
	EventType     string
	Success       bool
	TeamScore     int
	TargetScore   int
	FishTotal     int
	SecretFound   bool
	StrategyBonus int
	Participants  []EventParticipantContext
}

type WeeklyCatContext struct {
	CatName            string
	EventsParticipated int
	StealChoices       int
	DistractChoices    int
	ScoutChoices       int
	Contribution       int
	FishReward         int
	MVPCount           int
}

type WeeklySummaryContext struct {
	YardName           string
	EventsResolved     int
	SuccessfulEvents   int
	TotalChoices       int
	UniqueParticipants int
	FishTotal          int
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

type GenerationRequest struct {
	Type          GenerationType
	Cat           CatContext
	YardID        int64
	HumorMode     HumorMode
	UserMessage   string
	EventFacts    []string
	Event         *EventContext
	WeeklySummary *WeeklySummaryContext
	Training      *TrainingContext
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
	GenerateWeeklySummary(ctx context.Context, request GenerationRequest) (Generation, error)
}
