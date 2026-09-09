package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

type YardService struct {
	yards  YardRepository
	cats   CatRepository
	clock  Clock
	events GameEventSink
}

type YardSnapshot struct {
	Yard          *domain.Yard
	Members       []domain.YardMember
	Relationships []domain.CatRelationship
	Created       bool
	Joined        bool
}

func NewYardService(yards YardRepository, cats CatRepository, clock Clock, events GameEventSink) *YardService {
	return &YardService{yards: yards, cats: cats, clock: clock, events: events}
}

func (s *YardService) Enter(ctx context.Context, telegramChatID int64, chatName string, userID int64) (YardSnapshot, error) {
	if telegramChatID == 0 {
		return YardSnapshot{}, fmt.Errorf("telegram chat id is required")
	}
	cat, err := s.cats.GetByUserID(ctx, userID)
	if err != nil {
		return YardSnapshot{}, err
	}
	chatName = strings.TrimSpace(chatName)
	if chatName == "" {
		chatName = "Наш двор"
	}
	now := s.clock.Now()
	yard, created, joined, err := s.yards.EnsureAndJoin(ctx, telegramChatID, chatName, userID, cat.ID, now)
	if err != nil {
		return YardSnapshot{}, err
	}
	if s.events != nil && created {
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: fmt.Sprintf("yard:%d:created", yard.ID), Kind: GameEventYardCreated,
			UserID: userID, CatID: cat.ID, YardID: yard.ID, OccurredAt: now,
			ContentVersion: gameengine.CurrentContentVersion, PayloadVersion: GameEventPayloadVersion, Notable: true,
			Payload: YardCreatedPayload{TelegramChatID: telegramChatID, Name: yard.Name},
		})
	}
	if s.events != nil && joined {
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: fmt.Sprintf("yard:%d:user:%d:joined", yard.ID, userID), Kind: GameEventYardMemberJoined,
			UserID: userID, CatID: cat.ID, YardID: yard.ID, OccurredAt: now,
			ContentVersion: gameengine.CurrentContentVersion, PayloadVersion: GameEventPayloadVersion, Notable: true,
			Payload: YardMemberJoinedPayload{
				TelegramChatID: telegramChatID, CatName: cat.Name,
				Breed: string(cat.Breed), Trait: string(cat.Trait),
			},
		})
	}
	members, err := s.yards.ListMembers(ctx, yard.ID)
	if err != nil {
		return YardSnapshot{}, err
	}
	relationships, err := s.yards.ListRelationships(ctx, yard.ID)
	if err != nil {
		return YardSnapshot{}, err
	}
	return YardSnapshot{Yard: yard, Members: members, Relationships: relationships, Created: created, Joined: joined}, nil
}

func (s *YardService) Get(ctx context.Context, telegramChatID int64) (YardSnapshot, error) {
	yard, err := s.yards.GetByTelegramChatID(ctx, telegramChatID)
	if err != nil {
		return YardSnapshot{}, err
	}
	members, err := s.yards.ListMembers(ctx, yard.ID)
	if err != nil {
		return YardSnapshot{}, err
	}
	relationships, err := s.yards.ListRelationships(ctx, yard.ID)
	if err != nil {
		return YardSnapshot{}, err
	}
	return YardSnapshot{Yard: yard, Members: members, Relationships: relationships}, nil
}

func (s *YardService) Settings(ctx context.Context, telegramChatID int64) (*domain.Yard, error) {
	return s.yards.GetByTelegramChatID(ctx, telegramChatID)
}

func (s *YardService) SaveSettings(ctx context.Context, telegramChatID int64, settings domain.YardSettings) (*domain.Yard, error) {
	if !domain.IsValidHumorMode(settings.HumorMode) {
		return nil, fmt.Errorf("invalid yard humor mode")
	}
	if settings.MaxAutoMessagesDay < 0 || settings.MaxAutoMessagesDay > 2 {
		return nil, fmt.Errorf("invalid autonomous message limit")
	}
	yard, err := s.yards.GetByTelegramChatID(ctx, telegramChatID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	updated, err := s.yards.SaveSettings(ctx, yard.ID, settings, now)
	if err != nil {
		return nil, err
	}
	if s.events != nil {
		_ = s.events.Publish(ctx, GameEvent{
			DedupeKey: "yard:" + strconv.FormatInt(yard.ID, 10) + ":settings:" + strconv.FormatInt(now.UnixNano(), 10),
			Kind:      GameEventYardSettingsChanged, YardID: yard.ID, OccurredAt: now,
			ContentVersion: gameengine.CurrentContentVersion, PayloadVersion: GameEventPayloadVersion,
			Payload: YardSettingsChangedPayload{
				AutoMessagesEnabled: updated.AutoMessagesEnabled, MaxAutoMessagesDay: updated.MaxAutoMessagesDay,
				CatToCatBanter: updated.CatToCatBanter, FightsEnabled: updated.FightsEnabled,
				HumorMode: updated.HumorMode, QuietUntil: updated.QuietUntil,
			},
		})
	}
	return updated, nil
}
