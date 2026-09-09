package app

import (
	"context"
	"time"

	"catforge/internal/domain"
	"catforge/internal/pkg/randx"
)

// Clock provides current time (for deterministic tests).
type Clock interface {
	Now() time.Time
}

type RNG = randx.RNG

// GameEventSink consumes immutable gameplay facts. PostgreSQL is the first
// consumer; Telegram and future AI/Yard adapters subscribe to the same stream.
type GameEventSink interface {
	Publish(ctx context.Context, event GameEvent) error
}

type UpdateRepository interface {
	Claim(ctx context.Context, updateID int64) (bool, error)
}

type UserRepository interface {
	EnsureUser(ctx context.Context, telegramID int64) (int64, error)
	GetPendingAction(ctx context.Context, userID int64) (string, error)
	SetPendingAction(ctx context.Context, userID int64, action string) error
	GetPendingInput(ctx context.Context, userID, chatID int64) (*PendingInput, error)
	SavePendingInput(ctx context.Context, input PendingInput) error
	ClearPendingInput(ctx context.Context, userID, chatID int64) error
	ClaimDailyCommand(ctx context.Context, userID, chatID int64, command string, now time.Time) (bool, error)
}

type PendingInput struct {
	UserID          int64
	ChatID          int64
	Kind            string
	PromptMessageID int64
	ExpiresAt       time.Time
}

type CatRepository interface {
	GetByUserID(ctx context.Context, userID int64) (*domain.Cat, error)
	Create(ctx context.Context, userID int64, name string, breed domain.Breed, trait domain.Trait, hp, atk, def, spd int) (*domain.Cat, error)
	DeleteByUserID(ctx context.Context, userID int64) error
	SaveProgress(ctx context.Context, userID, expectedVersion int64, cat domain.Cat) (bool, error)
	SetName(ctx context.Context, userID int64, name string) (*domain.Cat, error)
}

type PersonalityRepository interface {
	GetByCatID(ctx context.Context, catID int64) (*domain.CatPersonality, error)
	SetTrait(ctx context.Context, catID int64, trait domain.Trait, speechStyle string) (*domain.CatPersonality, error)
	SetHumorMode(ctx context.Context, catID int64, mode domain.HumorMode) error
	SetAutoSpeak(ctx context.Context, catID int64, enabled bool) error
}

type AIQuotaRepository interface {
	AllowAIRequest(ctx context.Context, userID, chatID int64, now time.Time, userLimit, chatLimit int) (bool, error)
}

type CatMessageReferenceRepository interface {
	Remember(ctx context.Context, chatID int64, messageID int, catID int64, expiresAt time.Time) error
	Claim(ctx context.Context, chatID int64, messageID int, now time.Time) (*domain.CatMessageReference, bool, error)
}

type YardRepository interface {
	EnsureAndJoin(ctx context.Context, telegramChatID int64, name string, userID, catID int64, now time.Time) (yard *domain.Yard, created, joined bool, err error)
	GetByTelegramChatID(ctx context.Context, telegramChatID int64) (*domain.Yard, error)
	GetByID(ctx context.Context, yardID int64) (*domain.Yard, error)
	ListMembers(ctx context.Context, yardID int64) ([]domain.YardMember, error)
	ListRelationships(ctx context.Context, yardID int64) ([]domain.CatRelationship, error)
	SaveSettings(ctx context.Context, yardID int64, settings domain.YardSettings, now time.Time) (*domain.Yard, error)
	ClaimAutoMessageSlot(ctx context.Context, yardID int64, now time.Time, limit int, kind domain.AutoMessageKind) (bool, error)
}

// IdleBanterYardRepository is implemented by persistent yard stores that can
// select yards whose deterministic 24-48 hour banter window is due.
type IdleBanterYardRepository interface {
	ListIdleBanterCandidates(ctx context.Context, now, activeSince time.Time, limit int) ([]domain.Yard, error)
}

type YardEventRepository interface {
	NextEventType(ctx context.Context, yardID int64) (domain.YardEventType, error)
	StartOrGet(ctx context.Context, yardID int64, eventType domain.YardEventType, seed int64, startsAt, resolvesAt time.Time, contentVersion uint32) (event *domain.YardEvent, created bool, err error)
	GetCurrentActive(ctx context.Context, telegramChatID int64) (*domain.YardEvent, error)
	GetActive(ctx context.Context, telegramChatID, eventID int64) (*domain.YardEvent, error)
	ListStartCandidates(ctx context.Context, now, activeSince time.Time, limit int) ([]domain.Yard, error)
	SubmitChoice(ctx context.Context, telegramChatID, eventID, userID int64, choice domain.YardEventChoiceID, now time.Time) (saved domain.YardEventChoice, first bool, err error)
	ChoiceCounts(ctx context.Context, eventID int64) (map[domain.YardEventChoiceID]int, error)
	DueEventIDs(ctx context.Context, now time.Time, limit int) ([]int64, error)
	GetResolutionInput(ctx context.Context, eventID int64) (domain.YardEventResolutionInput, error)
	SaveResolution(ctx context.Context, eventID int64, result domain.YardEventResult, rulesVersion, contentVersion uint32, resolvedAt time.Time) (bool, error)
	WeeklySummary(ctx context.Context, yardID int64, periodStart, periodEnd time.Time) (domain.YardWeeklySummary, error)
}

type FightRepository interface {
	ToggleQueue(ctx context.Context, yardID, userID, catID int64, now, expiresAt time.Time, limits domain.FightLimits) (domain.FightQueueToggle, error)
	GetRevenge(ctx context.Context, telegramChatID, sourceFightID, loserUserID int64, now time.Time, limits domain.FightLimits) (domain.FightRevenge, error)
	SaveFight(ctx context.Context, record domain.FightRecord, result domain.FightResult, catAName, catBName string, limits domain.FightLimits) (domain.FightSaveResult, error)
	CatStats(ctx context.Context, catID int64) (domain.ArenaStats, error)
}

type ItemRepository interface {
	ListOwned(ctx context.Context, userID int64) ([]domain.OwnedItem, error)
	ListBestiary(ctx context.Context, userID int64) ([]domain.BestiaryEntry, error)
	Equip(ctx context.Context, userID int64, slot domain.ItemSlot, itemID string) error
	Upgrade(ctx context.Context, userID int64, itemID string, expectedLevel, fragments int, coins int64) (domain.OwnedItem, error)
	SaveExpedition(ctx context.Context, userID, expectedVersion int64, cat domain.Cat, dropItemID string, enemyKind domain.EnemyKind, victory bool) (ExpeditionSaveResult, error)
}

type ExpeditionSaveResult struct {
	Saved bool
	Item  domain.OwnedItem
	IsNew bool
}
