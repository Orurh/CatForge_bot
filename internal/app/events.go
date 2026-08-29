package app

import (
	"context"
	"errors"
	"time"

	"catforge/internal/domain"
)

// GameEventKind names a durable gameplay fact. Events describe what already
// happened; consumers may persist, analyse, or narrate them, but never alter it.
type GameEventKind string

const (
	GameEventUserStarted          GameEventKind = "user_started"
	GameEventCatCreated           GameEventKind = "cat_created"
	GameEventFirstPersonalityLine GameEventKind = "first_personality_line"
	GameEventCatReplyGenerated    GameEventKind = "cat_reply_generated"
	GameEventYardCreated          GameEventKind = "yard_created"
	GameEventYardMemberJoined     GameEventKind = "yard_member_joined"
	GameEventYardEventStarted     GameEventKind = "yard_event_started"
	GameEventYardChoiceSubmitted  GameEventKind = "yard_choice_submitted"
	GameEventYardEventResolved    GameEventKind = "yard_event_resolved"
	GameEventAutonomousCatMessage GameEventKind = "autonomous_cat_message"
	GameEventYardWeeklySummary    GameEventKind = "yard_weekly_summary"
	GameEventCatTrained           GameEventKind = "cat_trained"
	GameEventCatLeveledUp         GameEventKind = "cat_leveled_up"
	GameEventExpeditionStarted    GameEventKind = "expedition_started"
	GameEventExpeditionFinished   GameEventKind = "expedition_finished"
	GameEventItemFound            GameEventKind = "item_found"
	GameEventFightFinished        GameEventKind = "fight_finished"
)

const GameEventPayloadVersion = 1

var ErrDuplicateGameEvent = errors.New("duplicate game event")

type GameEvent struct {
	DedupeKey      string
	Kind           GameEventKind
	UserID         int64
	CatID          int64
	YardID         int64
	OccurredAt     time.Time
	RulesVersion   uint32
	ContentVersion uint32
	PayloadVersion int
	Notable        bool
	Payload        any
}

type UserStartedPayload struct {
	TelegramID int64  `json:"telegram_id"`
	ChatType   string `json:"chat_type"`
}

type CatCreatedPayload struct {
	CatName string       `json:"cat_name"`
	Breed   domain.Breed `json:"breed"`
	Trait   domain.Trait `json:"trait"`
	Level   int          `json:"level"`
}

type FirstPersonalityLinePayload struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Emotion  string `json:"emotion"`
	Fallback bool   `json:"fallback"`
}

type CatReplyGeneratedPayload struct {
	TargetChatID int64  `json:"target_chat_id"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Emotion      string `json:"emotion"`
	Fallback     bool   `json:"fallback"`
	Requested    bool   `json:"requested"`
}

type YardCreatedPayload struct {
	TelegramChatID int64  `json:"telegram_chat_id"`
	Name           string `json:"name"`
}

type YardMemberJoinedPayload struct {
	TelegramChatID int64  `json:"telegram_chat_id"`
	CatName        string `json:"cat_name"`
	Breed          string `json:"breed"`
	Trait          string `json:"trait"`
}

type YardEventStartedPayload struct {
	EventID    int64     `json:"event_id"`
	EventType  string    `json:"event_type"`
	Seed       int64     `json:"seed"`
	ResolvesAt time.Time `json:"resolves_at"`
}

type YardChoiceSubmittedPayload struct {
	EventID int64  `json:"event_id"`
	Choice  string `json:"choice"`
	First   bool   `json:"first"`
}

type YardEventResolvedPayload struct {
	EventID        int64                  `json:"event_id"`
	TelegramChatID int64                  `json:"telegram_chat_id"`
	Result         domain.YardEventResult `json:"result"`
	Narrative      string                 `json:"narrative"`
	Provider       string                 `json:"provider"`
	Model          string                 `json:"model"`
	Fallback       bool                   `json:"fallback"`
}

type AutonomousCatMessagePayload struct {
	TelegramChatID int64        `json:"telegram_chat_id"`
	CatName        string       `json:"cat_name"`
	Breed          domain.Breed `json:"breed"`
	Level          int          `json:"level"`
	Text           string       `json:"text"`
	Trigger        string       `json:"trigger"`
	Provider       string       `json:"provider"`
	Model          string       `json:"model"`
	Fallback       bool         `json:"fallback"`
}

type YardWeeklySummaryPayload struct {
	TelegramChatID   int64     `json:"telegram_chat_id"`
	PeriodStart      time.Time `json:"period_start"`
	PeriodEnd        time.Time `json:"period_end"`
	EventsResolved   int       `json:"events_resolved"`
	SuccessfulEvents int       `json:"successful_events"`
	Participants     int       `json:"unique_participants"`
	FishTotal        int       `json:"fish_total"`
	SecretsFound     int       `json:"secrets_found"`
	CatOfWeek        string    `json:"cat_of_week,omitempty"`
	TopTroublemaker  string    `json:"top_troublemaker,omitempty"`
	TopScout         string    `json:"top_scout,omitempty"`
	TopDistractor    string    `json:"top_distractor,omitempty"`
	Narrative        string    `json:"narrative,omitempty"`
	Provider         string    `json:"provider,omitempty"`
	Model            string    `json:"model,omitempty"`
	Fallback         bool      `json:"fallback"`
	AIRateLimited    bool      `json:"ai_rate_limited"`
}

type CatTrainedPayload struct {
	TargetChatID int64              `json:"target_chat_id,omitempty"`
	TelegramID   int64              `json:"telegram_id"`
	CatName      string             `json:"cat_name"`
	Breed        domain.Breed       `json:"breed"`
	Trait        domain.Trait       `json:"trait"`
	Energy       int                `json:"energy"`
	Level        int                `json:"level"`
	Result       domain.TrainResult `json:"result"`
	Narrative    string             `json:"narrative,omitempty"`
	RivalName    string             `json:"rival_name,omitempty"`
	Rivalry      int                `json:"rivalry,omitempty"`
	Provider     string             `json:"provider,omitempty"`
	Model        string             `json:"model,omitempty"`
	Fallback     bool               `json:"fallback"`
}

type CatLeveledUpPayload struct {
	TargetChatID int64            `json:"target_chat_id,omitempty"`
	CatName      string           `json:"cat_name"`
	Level        int              `json:"level"`
	LevelsGained int              `json:"levels_gained"`
	StatsGained  domain.StatDelta `json:"stats_gained"`
}

type ExpeditionStartedPayload struct {
	Location   domain.ExpeditionLocation   `json:"location"`
	Difficulty domain.ExpeditionDifficulty `json:"difficulty"`
	Seed       uint64                      `json:"seed"`
}

type ExpeditionFinishedPayload struct {
	TargetChatID int64                   `json:"target_chat_id,omitempty"`
	CatName      string                  `json:"cat_name"`
	Breed        domain.Breed            `json:"breed"`
	Trait        domain.Trait            `json:"trait"`
	Energy       int                     `json:"energy"`
	Level        int                     `json:"level"`
	Result       domain.ExpeditionResult `json:"result"`
}

type ItemFoundPayload struct {
	ItemID    string            `json:"item_id"`
	Rarity    domain.ItemRarity `json:"rarity"`
	IsNew     bool              `json:"is_new"`
	Fragments int               `json:"fragments"`
}

type FightFinishedPayload struct {
	FightID        int64              `json:"fight_id"`
	TelegramChatID int64              `json:"telegram_chat_id"`
	CatAName       string             `json:"cat_a_name"`
	CatBName       string             `json:"cat_b_name"`
	Result         domain.FightResult `json:"result"`
	Rivalry        int                `json:"rivalry"`
	Seed           uint64             `json:"seed"`
}

// GameEventFanout sends events in order. A failure stops the chain, so the
// durable store can be placed first and presentation adapters after it.
type GameEventFanout struct {
	sinks []GameEventSink
}

func NewGameEventFanout(sinks ...GameEventSink) *GameEventFanout {
	filtered := make([]GameEventSink, 0, len(sinks))
	for _, sink := range sinks {
		if sink != nil {
			filtered = append(filtered, sink)
		}
	}
	return &GameEventFanout{sinks: filtered}
}

func (f *GameEventFanout) Publish(ctx context.Context, event GameEvent) error {
	for _, sink := range f.sinks {
		if err := sink.Publish(ctx, event); err != nil {
			if errors.Is(err, ErrDuplicateGameEvent) {
				return nil
			}
			return errors.Join(errors.New("publish game event"), err)
		}
	}
	return nil
}
