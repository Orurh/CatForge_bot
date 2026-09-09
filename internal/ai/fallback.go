package ai

import (
	"context"
	"hash/fnv"
	"strings"

	"catforge/internal/domain"
	"catforge/internal/gamecontent"
)

type FallbackProvider struct{}

func NewFallbackProvider() FallbackProvider { return FallbackProvider{} }

func (FallbackProvider) Name() string { return "procedural" }

func (FallbackProvider) Generate(_ context.Context, prompt Prompt) (ProviderResult, error) {
	request := prompt.Request
	if request.Type == GenerationEventNarrative && request.Event != nil {
		return ProviderResult{Text: fallbackEventNarrative(request), Emotion: "excited", Model: "procedural-v1"}, nil
	}
	if request.Type == GenerationAutonomousCat && request.Event != nil {
		return ProviderResult{Text: fallbackAutonomousEvent(request), Emotion: fallbackEmotion(request.Cat.Trait), Model: "procedural-v1"}, nil
	}
	if request.Type == GenerationWeeklySummary && request.WeeklySummary != nil {
		return ProviderResult{Text: fallbackWeeklySummary(*request.WeeklySummary), Emotion: "excited", Model: "procedural-v1"}, nil
	}
	if request.Type == GenerationTrainingNarrative && request.Training != nil {
		return ProviderResult{Text: fallbackTrainingNarrative(request), Emotion: fallbackEmotion(request.Cat.Trait), Model: "procedural-v1"}, nil
	}
	if request.Type == GenerationArenaBanter {
		return ProviderResult{Text: fallbackArenaBanter(request.ArenaBanter), Emotion: "smug", Model: "procedural-v1"}, nil
	}
	if request.Type == GenerationYardBanter {
		return ProviderResult{Text: fallbackYardBanter(request.YardBanter), Emotion: "smug", Model: "procedural-v1"}, nil
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(request.Cat.Name))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(request.UserMessage))
	key, data := fallbackReply(request)
	line := gamecontent.Render(key, uint64(h.Sum32()), data)
	return ProviderResult{Text: line, Emotion: fallbackEmotion(request.Cat.Trait), Model: "procedural-v1"}, nil
}

func fallbackYardBanter(banter *YardBanterContext) string {
	key := "fallback.yard_banter.idle.default"
	selector := uint64(0)
	if banter != nil {
		selector = uint64(banter.Friendship + banter.Rivalry + banter.Respect + banter.CatAContribution + banter.CatBContribution)
		tone := "default"
		switch {
		case banter.Rivalry >= 3 && banter.Rivalry >= banter.Friendship+2 && banter.Rivalry >= banter.Respect:
			tone = "rivalry"
		case banter.Friendship >= 3 && banter.Friendship > banter.Rivalry && banter.Friendship >= banter.Respect:
			tone = "friendship"
		case banter.Respect >= 3 && banter.Respect > banter.Rivalry && banter.Respect > banter.Friendship:
			tone = "respect"
		}
		if banter.Trigger == "yard_event" {
			key = "fallback.yard_banter.event." + tone
			if banter.OutcomeTier == string(domain.YardOutcomeFailure) {
				key = "fallback.yard_banter.event.failure"
			} else if banter.OutcomeTier == string(domain.YardOutcomeExceptional) {
				key = "fallback.yard_banter.event.exceptional"
			} else if banter.CatAChoice != "" && banter.CatBChoice != "" && banter.CatAChoice != banter.CatBChoice {
				key = "fallback.yard_banter.event.opposite"
			}
		} else {
			key = "fallback.yard_banter.idle." + tone
		}
	}
	return gamecontent.Render(key, selector, banter)
}

func fallbackArenaBanter(banter *ArenaBanterContext) string {
	key := "fallback.arena_banter.rivalry"
	selector := uint64(0)
	if banter != nil {
		selector = uint64(banter.Friendship + banter.Rivalry + banter.Respect + banter.Rounds + banter.WinnerHP + banter.WinnerStreak)
		switch {
		case banter.FightKind == string(domain.FightKindRevenge):
			key = "fallback.arena_banter.revenge"
		case banter.DecidingFight:
			key = "fallback.arena_banter.deciding"
		case banter.Rivalry >= 8 && banter.Rivalry >= banter.Friendship+2 && banter.Rivalry >= banter.Respect:
			key = "fallback.arena_banter.high_rivalry"
		case banter.Friendship >= 3 && banter.Friendship > banter.Rivalry && banter.Friendship >= banter.Respect:
			key = "fallback.arena_banter.friendship"
		case banter.Respect >= 3 && banter.Respect > banter.Rivalry && banter.Respect > banter.Friendship:
			key = "fallback.arena_banter.respect"
		case banter.Upset:
			key = "fallback.arena_banter.upset"
		case banter.Close:
			key = "fallback.arena_banter.close"
		case banter.FirstFight:
			key = "fallback.arena_banter.first"
		}
	}
	return gamecontent.Render(key, selector, banter)
}

func fallbackTrainingNarrative(request GenerationRequest) string {
	cat, training := request.Cat, *request.Training
	key := "fallback.training.default"
	if gamecontent.Has("fallback.training.encounter." + training.Encounter) {
		key = "fallback.training.encounter." + training.Encounter
	}
	selector := uint64(training.EnergyCost + training.Level + training.LevelsGained)
	story := gamecontent.Render(key, selector, nil)
	if training.RivalName != "" {
		story += " " + gamecontent.Render("fallback.training.rival", selector, training)
	} else if relationship, tone, ok := dominantRelationship(request.Cat.Name, request.Relationships); ok && (tone == "friendship" || tone == "respect") {
		story += " " + gamecontent.Render("fallback.training."+tone, selector, relationship)
	} else if cat.Trait == domain.TraitLazy {
		story += " " + gamecontent.Render("fallback.training.lazy", selector, nil)
	}
	return story
}

type relationshipFallbackData struct {
	CatAName     string
	CatBName     string
	OtherCatName string
	Friendship   int
	Rivalry      int
	Respect      int
}

func fallbackReply(request GenerationRequest) (string, any) {
	if request.Type == GenerationHumanReplyToCat || request.Type == GenerationCatReply {
		contextText := strings.ToLower(request.PreviousMessage + " " + request.UserMessage)
		for _, relationship := range request.Relationships {
			data, tone, ok := relationshipForCat(request.Cat.Name, relationship)
			if ok && tone != "" && strings.Contains(contextText, strings.ToLower(data.OtherCatName)) {
				return "fallback.relationship_reply." + tone, data
			}
		}
	}
	return fallbackReplyKey(request.Type, request.Cat.Trait, request.HumorMode), nil
}

func dominantRelationship(catName string, relationships []RelationshipContext) (relationshipFallbackData, string, bool) {
	var best relationshipFallbackData
	bestTone, bestStrength := "", 0
	for _, relationship := range relationships {
		data, tone, ok := relationshipForCat(catName, relationship)
		if !ok || tone == "" {
			continue
		}
		strength := data.Rivalry
		if tone == "friendship" {
			strength = data.Friendship
		} else if tone == "respect" {
			strength = data.Respect
		}
		if strength > bestStrength {
			best, bestTone, bestStrength = data, tone, strength
		}
	}
	return best, bestTone, bestTone != ""
}

func relationshipForCat(catName string, relationship RelationshipContext) (relationshipFallbackData, string, bool) {
	other := ""
	switch catName {
	case relationship.CatAName:
		other = relationship.CatBName
	case relationship.CatBName:
		other = relationship.CatAName
	}
	if strings.TrimSpace(other) == "" {
		return relationshipFallbackData{}, "", false
	}
	data := relationshipFallbackData{
		CatAName: relationship.CatAName, CatBName: relationship.CatBName,
		OtherCatName: other, Friendship: relationship.Friendship,
		Rivalry: relationship.Rivalry, Respect: relationship.Respect,
	}
	switch {
	case relationship.Rivalry >= 5 && relationship.Rivalry >= relationship.Friendship+2 && relationship.Rivalry >= relationship.Respect:
		return data, "rivalry", true
	case relationship.Friendship >= 3 && relationship.Friendship > relationship.Rivalry && relationship.Friendship >= relationship.Respect:
		return data, "friendship", true
	case relationship.Respect >= 3 && relationship.Respect > relationship.Rivalry && relationship.Respect > relationship.Friendship:
		return data, "respect", true
	default:
		return data, "", true
	}
}

func dominantPair(relationships []RelationshipContext) (relationshipFallbackData, string, bool) {
	var best relationshipFallbackData
	bestTone, bestStrength := "", 0
	for _, relationship := range relationships {
		data, tone, ok := relationshipForCat(relationship.CatAName, relationship)
		if !ok || tone == "" {
			continue
		}
		strength := relationship.Rivalry
		if tone == "friendship" {
			strength = relationship.Friendship
		} else if tone == "respect" {
			strength = relationship.Respect
		}
		if strength > bestStrength {
			best, bestTone, bestStrength = data, tone, strength
		}
	}
	return best, bestTone, bestTone != ""
}

func fallbackWeeklySummary(summary WeeklySummaryContext) string {
	selector := uint64(summary.EventsResolved + summary.YardScore + summary.SecretsFound)
	parts := []string{gamecontent.Render("fallback.week.summary", selector, summary)}
	if summary.CatOfWeek != "" {
		parts = append(parts, gamecontent.Render("fallback.week.cat_of_week", selector, struct{ CatName string }{summary.CatOfWeek}))
	}
	if summary.SecretsFound > 0 {
		parts = append(parts, gamecontent.Render("fallback.week.secrets", selector, summary))
	}
	return strings.Join(parts, " ")
}

func fallbackAutonomousEvent(request GenerationRequest) string {
	cat, event := request.Cat, *request.Event
	prefix := yardEventFallbackPrefix(event.EventType)
	story := ""
	if domain.YardEventOutcomeTier(event.OutcomeTier).IsSuccess() {
		slug := traitContentSlug(cat.Trait)
		if slug == "default" {
			slug = "sleepy"
		}
		story = gamecontent.Render(prefix+"autonomous.success."+slug, uint64(event.TeamScore), nil)
	} else {
		story = gamecontent.Render(prefix+"autonomous.failure", uint64(event.TeamScore), nil)
	}
	if relationship, tone, ok := dominantRelationship(cat.Name, request.Relationships); ok {
		story += " " + gamecontent.Render("fallback.autonomous.relationship."+tone, uint64(event.TeamScore), relationship)
	}
	return story
}

func fallbackEventNarrative(request GenerationRequest) string {
	event := *request.Event
	tier := domain.YardEventOutcomeTier(event.OutcomeTier)
	key := yardEventFallbackPrefix(event.EventType) + string(tier)
	if !gamecontent.Has(key) {
		key = yardEventFallbackPrefix(event.EventType) + "failure"
		if tier.IsSuccess() {
			key = yardEventFallbackPrefix(event.EventType) + "success"
		}
	}
	selector := uint64(event.TeamScore + event.YardScore + len(event.Participants))
	story := gamecontent.Render(key, selector, nil)
	if relationship, tone, ok := dominantPair(request.Relationships); ok {
		story += " " + gamecontent.Render("fallback.event.relationship."+tone, selector, relationship)
	}
	return story
}

func yardEventFallbackPrefix(eventType string) string {
	switch domain.YardEventType(eventType) {
	case domain.YardEventBigDog, domain.YardEventBigBox:
		return "yard_event." + eventType + "."
	default:
		return "yard_event." + string(domain.YardEventFishTruck) + "."
	}
}

func fallbackReplyKey(kind GenerationType, trait domain.Trait, humor HumorMode) string {
	slug := traitContentSlug(trait)
	if kind == GenerationHumanReplyToCat {
		mode := "normal"
		if humor == HumorBold {
			mode = "bold"
		}
		if slug == "default" {
			return "fallback.followup." + mode + ".default"
		}
		return "fallback.followup." + mode + "." + slug
	}
	if kind == GenerationFirstPersonalityLine {
		if slug != "default" {
			return "fallback.first_line." + slug
		}
		return "fallback.reply.normal.default"
	}
	if humor == HumorBold {
		if slug != "default" {
			return "fallback.reply.bold." + slug
		}
		return "fallback.reply.bold.default"
	}
	return "fallback.reply.normal." + slug
}

func traitContentSlug(trait domain.Trait) string {
	switch trait {
	case domain.TraitLazy, domain.TraitBully, domain.TraitPhilosopher, domain.TraitNeat, domain.TraitSleepy:
		return string(trait)
	default:
		return "default"
	}
}

func fallbackEmotion(trait domain.Trait) string {
	switch trait {
	case domain.TraitBully:
		return "smug"
	case domain.TraitPhilosopher:
		return "thoughtful"
	case domain.TraitNeat:
		return "judging"
	case domain.TraitLazy, domain.TraitSleepy:
		return "sleepy"
	default:
		return "neutral"
	}
}
