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
	GameEventCatAutoSpeakChanged  GameEventKind = "cat_autospeak_changed"
	GameEventCatReplyGenerated    GameEventKind = "cat_reply_generated"
	GameEventHumanRepliedToCat    GameEventKind = "human_reply_to_cat"
	GameEventCatFollowupGenerated GameEventKind = "reaction_to_cat"
	GameEventYardCreated          GameEventKind = "yard_created"
	GameEventYardMemberJoined     GameEventKind = "yard_member_joined"
	GameEventYardSettingsChanged  GameEventKind = "yard_settings_changed"
	GameEventYardEventStarted     GameEventKind = "yard_event_started"
	GameEventYardChoiceSubmitted  GameEventKind = "yard_choice_submitted"
	GameEventYardEventResolved    GameEventKind = "yard_event_resolved"
	GameEventAutonomousCatMessage GameEventKind = "autonomous_cat_message"
	GameEventCatBanter            GameEventKind = "cat_banter"
	GameEventYardWeeklySummary    GameEventKind = "yard_weekly_summary"
	GameEventCatTrained           GameEventKind = "cat_trained"
	GameEventCatLeveledUp         GameEventKind = "cat_leveled_up"
	GameEventExpeditionStarted    GameEventKind = "expedition_started"
	GameEventExpeditionFinished   GameEventKind = "expedition_finished"
	GameEventItemFound            GameEventKind = "item_found"
	GameEventFightFinished        GameEventKind = "fight_finished"
)

const GameEventPayloadVersion = 2

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

type CatAutoSpeakChangedPayload struct {
	Enabled bool `json:"enabled"`
}

type CatReplyGeneratedPayload struct {
	TargetChatID int64  `json:"target_chat_id"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Emotion      string `json:"emotion"`
	Fallback     bool   `json:"fallback"`
	Requested    bool   `json:"requested"`
}

type HumanRepliedToCatPayload struct {
	TelegramChatID      int64 `json:"telegram_chat_id"`
	SourceMessageID     int   `json:"source_message_id"`
	HumanReplyMessageID int   `json:"human_reply_message_id"`
}

type CatFollowupGeneratedPayload struct {
	TelegramChatID      int64  `json:"telegram_chat_id"`
	SourceMessageID     int    `json:"source_message_id"`
	HumanReplyMessageID int    `json:"human_reply_message_id"`
	Provider            string `json:"provider"`
	Model               string `json:"model"`
	Emotion             string `json:"emotion"`
	Fallback            bool   `json:"fallback"`
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

type YardSettingsChangedPayload struct {
	AutoMessagesEnabled bool             `json:"auto_messages_enabled"`
	MaxAutoMessagesDay  int              `json:"max_auto_messages_day"`
	CatToCatBanter      bool             `json:"cat_to_cat_banter"`
	FightsEnabled       bool             `json:"fights_enabled"`
	HumorMode           domain.HumorMode `json:"humor_mode"`
	QuietUntil          time.Time        `json:"quiet_until,omitempty"`
}

type YardEventStartedPayload struct {
	EventID        int64     `json:"event_id"`
	TelegramChatID int64     `json:"telegram_chat_id,omitempty"`
	EventType      string    `json:"event_type"`
	Seed           int64     `json:"seed"`
	StartsAt       time.Time `json:"starts_at"`
	ResolvesAt     time.Time `json:"resolves_at"`
}

type YardChoiceSubmittedPayload struct {
	EventID int64  `json:"event_id"`
	Choice  string `json:"choice"`
	First   bool   `json:"first"`
}

type YardEventResolvedPayload struct {
	EventID        int64                               `json:"event_id"`
	TelegramChatID int64                               `json:"telegram_chat_id"`
	EventType      string                              `json:"event_type"`
	Result         domain.YardEventResult              `json:"result"`
	Participants   []YardEventParticipantResultPayload `json:"participants"`
	Narrative      string                              `json:"narrative"`
	Provider       string                              `json:"provider"`
	Model          string                              `json:"model"`
	Fallback       bool                                `json:"fallback"`
}

type YardEventParticipantResultPayload struct {
	CatID        int64                    `json:"cat_id"`
	CatName      string                   `json:"cat_name"`
	Breed        domain.Breed             `json:"breed"`
	Level        int                      `json:"level"`
	Choice       domain.YardEventChoiceID `json:"choice"`
	Contribution int                      `json:"contribution"`
	MVP          bool                     `json:"mvp"`
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

type CatBanterPayload struct {
	TelegramChatID int64        `json:"telegram_chat_id"`
	FirstCatID     int64        `json:"first_cat_id"`
	FirstCatName   string       `json:"first_cat_name"`
	FirstCatBreed  domain.Breed `json:"first_cat_breed"`
	FirstCatLevel  int          `json:"first_cat_level"`
	FirstLine      string       `json:"first_line"`
	SecondCatID    int64        `json:"second_cat_id"`
	SecondCatName  string       `json:"second_cat_name"`
	SecondCatBreed domain.Breed `json:"second_cat_breed"`
	SecondCatLevel int          `json:"second_cat_level"`
	SecondLine     string       `json:"second_line"`
	Trigger        string       `json:"trigger"`
	SourceEventID  int64        `json:"source_event_id,omitempty"`
	Provider       string       `json:"provider"`
	Model          string       `json:"model"`
	Fallback       bool         `json:"fallback"`
}

type YardWeeklySummaryPayload struct {
	TelegramChatID    int64                       `json:"telegram_chat_id"`
	PeriodStart       time.Time                   `json:"period_start"`
	PeriodEnd         time.Time                   `json:"period_end"`
	EventsResolved    int                         `json:"events_resolved"`
	FailedEvents      int                         `json:"failed_events"`
	PartialEvents     int                         `json:"partial_events"`
	SuccessfulEvents  int                         `json:"successful_events"`
	ExceptionalEvents int                         `json:"exceptional_events"`
	Participants      int                         `json:"unique_participants"`
	YardScore         int                         `json:"yard_score"`
	SecretsFound      int                         `json:"secrets_found"`
	CatOfWeek         string                      `json:"cat_of_week,omitempty"`
	TopTroublemaker   string                      `json:"top_troublemaker,omitempty"`
	TopScout          string                      `json:"top_scout,omitempty"`
	TopDistractor     string                      `json:"top_distractor,omitempty"`
	Narrative         string                      `json:"narrative,omitempty"`
	Provider          string                      `json:"provider,omitempty"`
	Model             string                      `json:"model,omitempty"`
	Fallback          bool                        `json:"fallback"`
	AIRateLimited     bool                        `json:"ai_rate_limited"`
	Cats              []domain.YardWeeklyCatStats `json:"cats"`
}

type CatTrainedPayload struct {
	EnergyUpdatedAt  time.Time          `json:"energy_updated_at,omitempty"`
	LootItemID       string             `json:"loot_item_id,omitempty"`
	ProgressionFacts []string           `json:"progression_facts,omitempty"`
	TargetChatID     int64              `json:"target_chat_id,omitempty"`
	TelegramID       int64              `json:"telegram_id"`
	CatName          string             `json:"cat_name"`
	Breed            domain.Breed       `json:"breed"`
	Trait            domain.Trait       `json:"trait"`
	Energy           int                `json:"energy"`
	Level            int                `json:"level"`
	Result           domain.TrainResult `json:"result"`
	Narrative        string             `json:"narrative,omitempty"`
	RivalName        string             `json:"rival_name,omitempty"`
	Rivalry          int                `json:"rivalry,omitempty"`
	Provider         string             `json:"provider,omitempty"`
	Model            string             `json:"model,omitempty"`
	Fallback         bool               `json:"fallback"`
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
	FightID        int64               `json:"fight_id"`
	TelegramChatID int64               `json:"telegram_chat_id"`
	CatAName       string              `json:"cat_a_name"`
	CatBName       string              `json:"cat_b_name"`
	Result         domain.FightResult  `json:"result"`
	Friendship     int                 `json:"friendship"`
	Rivalry        int                 `json:"rivalry"`
	Respect        int                 `json:"respect"`
	Seed           uint64              `json:"seed"`
	Kind           domain.FightKind    `json:"kind"`
	ParentFightID  int64               `json:"parent_fight_id,omitempty"`
	Stats          domain.FightStats   `json:"stats"`
	WinnerXPGain   int64               `json:"winner_xp_gain"`
	LoserXPGain    int64               `json:"loser_xp_gain"`
	Banter         *FightBanterPayload `json:"banter,omitempty"`
}

type FightBanterPayload struct {
	FirstCatID    int64  `json:"first_cat_id"`
	FirstCatName  string `json:"first_cat_name"`
	FirstLine     string `json:"first_line"`
	SecondCatID   int64  `json:"second_cat_id"`
	SecondCatName string `json:"second_cat_name"`
	SecondLine    string `json:"second_line"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Fallback      bool   `json:"fallback"`
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
