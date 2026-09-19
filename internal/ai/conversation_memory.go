package ai

import (
	"context"
	"fmt"
	"time"
)

type SpeechMemory struct {
	CatID, OtherCatID     int64
	CatName, OtherCatName string
	Text                  string
}

type ConversationMemory interface {
	Recent(ctx context.Context, scope string, catID, otherCatID int64) ([]SpeechMemory, error)
	Remember(ctx context.Context, scope string, speech SpeechMemory) error
}

// SetConversationMemory must be called before serving concurrent requests.
func (g *Gateway) SetConversationMemory(memory ConversationMemory) { g.memory = memory }

func speechScope(request GenerationRequest) string {
	if request.Cat.ID <= 0 {
		return ""
	}
	switch request.Type {
	case GenerationCatReply, GenerationHumanReplyToCat, GenerationFirstPersonalityLine,
		GenerationAutonomousCat, GenerationArenaBanter, GenerationYardBanter, GenerationBedtimeFarewell:
	default:
		return ""
	}
	if request.YardID > 0 {
		return fmt.Sprintf("yard:%d", request.YardID)
	}
	if request.ChatID != 0 {
		// Never mix a cat's private conversation with any group or another cat.
		return fmt.Sprintf("chat:%d:cat:%d", request.ChatID, request.Cat.ID)
	}
	return ""
}

func (g *Gateway) recentSpeech(ctx context.Context, scope string, request GenerationRequest) []SpeechMemory {
	if g.memory == nil || scope == "" {
		return nil
	}
	memoryCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	history, err := g.memory.Recent(memoryCtx, scope, request.Cat.ID, request.OtherCat.ID)
	if err != nil {
		return nil // Memory is optional; a storage outage must not silence cats.
	}
	return history
}

func (g *Gateway) rememberSpeech(ctx context.Context, scope string, request GenerationRequest, text string) {
	if g.memory == nil || scope == "" {
		return
	}
	memoryCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_ = g.memory.Remember(memoryCtx, scope, SpeechMemory{
		CatID: request.Cat.ID, OtherCatID: request.OtherCat.ID,
		CatName: request.Cat.Name, OtherCatName: request.OtherCat.Name, Text: text,
	})
}
