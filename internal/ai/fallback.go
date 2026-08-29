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
		return ProviderResult{Text: fallbackEventNarrative(*request.Event), Emotion: "excited", Model: "procedural-v1"}, nil
	}
	if request.Type == GenerationAutonomousCat && request.Event != nil {
		return ProviderResult{Text: fallbackAutonomousEvent(request.Cat, *request.Event), Emotion: fallbackEmotion(request.Cat.Trait), Model: "procedural-v1"}, nil
	}
	if request.Type == GenerationWeeklySummary && request.WeeklySummary != nil {
		return ProviderResult{Text: fallbackWeeklySummary(*request.WeeklySummary), Emotion: "excited", Model: "procedural-v1"}, nil
	}
	if request.Type == GenerationTrainingNarrative && request.Training != nil {
		return ProviderResult{Text: fallbackTrainingNarrative(request.Cat, *request.Training), Emotion: fallbackEmotion(request.Cat.Trait), Model: "procedural-v1"}, nil
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(request.Cat.Name))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(request.UserMessage))
	line := gamecontent.Render(fallbackReplyKey(request.Type, request.Cat.Trait, request.HumorMode), uint64(h.Sum32()), nil)
	return ProviderResult{Text: line, Emotion: fallbackEmotion(request.Cat.Trait), Model: "procedural-v1"}, nil
}

func fallbackTrainingNarrative(cat CatContext, training TrainingContext) string {
	key := "fallback.training.default"
	if gamecontent.Has("fallback.training.encounter." + training.Encounter) {
		key = "fallback.training.encounter." + training.Encounter
	}
	selector := uint64(training.EnergyCost + training.Level + training.LevelsGained)
	story := gamecontent.Render(key, selector, nil)
	if training.RivalName != "" {
		story += " " + gamecontent.Render("fallback.training.rival", selector, training)
	} else if cat.Trait == domain.TraitLazy {
		story += " " + gamecontent.Render("fallback.training.lazy", selector, nil)
	}
	return story
}

func fallbackWeeklySummary(summary WeeklySummaryContext) string {
	selector := uint64(summary.EventsResolved + summary.FishTotal + summary.SecretsFound)
	parts := []string{gamecontent.Render("fallback.week.summary", selector, summary)}
	if summary.CatOfWeek != "" {
		parts = append(parts, gamecontent.Render("fallback.week.cat_of_week", selector, struct{ CatName string }{summary.CatOfWeek}))
	}
	if summary.SecretsFound > 0 {
		parts = append(parts, gamecontent.Render("fallback.week.secrets", selector, summary))
	}
	return strings.Join(parts, " ")
}

func fallbackAutonomousEvent(cat CatContext, event EventContext) string {
	if event.Success {
		slug := traitContentSlug(cat.Trait)
		if slug == "default" {
			slug = "sleepy"
		}
		return gamecontent.Render("yard_event.autonomous.success."+slug, uint64(event.TeamScore), nil)
	}
	return gamecontent.Render("yard_event.autonomous.failure", uint64(event.TeamScore), nil)
}

func fallbackEventNarrative(event EventContext) string {
	key := "yard_event.fish_truck.failure"
	if event.Success {
		key = "yard_event.fish_truck.success"
	}
	selector := uint64(event.TeamScore + event.FishTotal + len(event.Participants))
	lead := gamecontent.Render(key, selector, nil)
	mvp := ""
	for _, participant := range event.Participants {
		if participant.MVP {
			mvp = participant.CatName
			break
		}
	}
	parts := []string{lead, gamecontent.Render("yard_event.fish_truck.catch", selector, event)}
	if event.SecretFound {
		parts = append(parts, gamecontent.Render("yard_event.fish_truck.secret", selector, nil))
	}
	if mvp != "" {
		parts = append(parts, gamecontent.Render("yard_event.fish_truck.mvp", selector, struct{ CatName string }{mvp}))
	}
	return strings.Join(parts, " ")
}

func fallbackReplyKey(kind GenerationType, trait domain.Trait, humor HumorMode) string {
	slug := traitContentSlug(trait)
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
		return "fallback.reply.normal.default"
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
