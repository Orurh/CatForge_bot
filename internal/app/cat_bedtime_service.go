package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"catforge/internal/ai"
	"catforge/internal/domain"
	"catforge/internal/gameengine"
)

const BedtimeFarewellWindow = 5 * time.Minute

func (s *CatBanterService) bedtimeWindow(now time.Time) (time.Time, bool) {
	if s.quietStart == s.quietEnd || s.isNight(now) {
		return time.Time{}, false
	}
	local := now.In(s.location)
	sleepAt := time.Date(local.Year(), local.Month(), local.Day(), s.quietStart, 0, 0, 0, s.location)
	if !sleepAt.After(now) {
		sleepAt = sleepAt.AddDate(0, 0, 1)
	}
	return sleepAt, !now.Before(sleepAt.Add(-BedtimeFarewellWindow))
}

func (s *CatBanterService) runBedtime(ctx context.Context, now, sleepAt time.Time, limit int) (int, error) {
	yards, err := s.bedtimeYards.ListBedtimeCandidates(ctx, now, sleepAt, now.Add(-IdleBanterActivityWindow), limit)
	if err != nil {
		return 0, err
	}
	count := 0
	for i := range yards {
		if !s.clock.Now().Before(sleepAt) {
			break
		}
		published, err := s.publishBedtime(ctx, &yards[i], sleepAt)
		if err != nil {
			return count, err
		}
		if published {
			count++
		}
	}
	return count, nil
}

func (s *CatBanterService) publishBedtime(ctx context.Context, yard *domain.Yard, sleepAt time.Time) (bool, error) {
	now := s.clock.Now()
	if !banterAllowed(yard, now) || !now.Before(sleepAt) {
		return false, nil
	}
	members, err := s.yards.ListMembers(ctx, yard.ID)
	if err != nil {
		return false, err
	}
	speakers := make([]banterSpeaker, 0, len(members))
	for _, member := range members {
		if member.LastActiveAt.Before(now.Add(-IdleBanterActivityWindow)) {
			continue
		}
		personality, err := s.personalities.GetByCatID(ctx, member.CatID)
		if err != nil {
			return false, err
		}
		if personality == nil || !personality.AutoSpeakEnabled {
			continue
		}
		speakers = append(speakers, banterSpeaker{ID: member.CatID, Name: member.CatName, Breed: member.Breed,
			Level: member.Level, Trait: personality.Trait, SpeechStyle: personality.SpeechStyle, HumorMode: personality.HumorMode})
	}
	if len(speakers) == 0 {
		return false, nil
	}
	sort.Slice(speakers, func(i, j int) bool { return speakers[i].ID < speakers[j].ID })
	// Rotate the speaker by the local sleep date; each yard sends a single farewell.
	speaker := speakers[(int64(sleepAt.YearDay())+yard.ID)%int64(len(speakers))]
	now = s.clock.Now()
	if !now.Before(sleepAt) {
		return false, nil
	}
	claimed, err := s.bedtimeYards.ClaimBedtimeFarewell(ctx, yard.ID, speaker.ID, now, sleepAt)
	if err != nil || !claimed {
		return false, err
	}
	humor := domain.HumorNormal
	if yard.HumorMode == domain.HumorBold && speaker.HumorMode == domain.HumorBold {
		humor = domain.HumorBold
	}
	request := ai.GenerationRequest{
		Type: ai.GenerationBedtimeFarewell, YardID: yard.ID, ChatID: yard.TelegramChatID, HumorMode: humor,
		Cat:        ai.CatContext{ID: speaker.ID, Name: speaker.Name, Breed: speaker.Breed, Trait: speaker.Trait, SpeechStyle: speaker.SpeechStyle},
		EventFacts: []string{"cat_is_going_to_sleep=true", "sleep_starts_at=" + strconv.Quote(sleepAt.Format(time.RFC3339))},
	}
	// Slow generation/delivery must not turn a farewell into a message during quiet hours.
	deadlineCtx, cancel := context.WithTimeout(ctx, sleepAt.Sub(now))
	defer cancel()
	generation, err := s.voice.Generate(deadlineCtx, request)
	if err != nil {
		return false, err
	}
	now = s.clock.Now()
	if !now.Before(sleepAt) || deadlineCtx.Err() != nil {
		return false, nil
	}
	currentYard, err := s.yards.GetByID(deadlineCtx, yard.ID)
	if err != nil || !banterAllowed(currentYard, now) {
		return false, err
	}
	personality, err := s.personalities.GetByCatID(deadlineCtx, speaker.ID)
	if err != nil || personality == nil || !personality.AutoSpeakEnabled {
		return false, err
	}
	if humor == domain.HumorBold && (currentYard.HumorMode != domain.HumorBold || personality.HumorMode != domain.HumorBold) {
		return false, nil
	}
	if !s.clock.Now().Before(sleepAt) || deadlineCtx.Err() != nil {
		return false, nil
	}
	err = s.events.Publish(deadlineCtx, GameEvent{
		DedupeKey: fmt.Sprintf("yard:%d:bedtime:%s", yard.ID, sleepAt.UTC().Format(time.RFC3339)),
		Kind:      GameEventAutonomousCatMessage, CatID: speaker.ID, YardID: yard.ID, OccurredAt: now,
		ContentVersion: gameengine.CurrentContentVersion, PayloadVersion: GameEventPayloadVersion,
		Payload: AutonomousCatMessagePayload{
			Trigger: "bedtime", TelegramChatID: yard.TelegramChatID, CatName: speaker.Name, Breed: speaker.Breed,
			Level: speaker.Level, Text: generation.Text, Provider: generation.Provider, Model: generation.Model, Fallback: generation.Fallback,
		},
	})
	return err == nil, err
}
