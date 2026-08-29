package narrative

import (
	"catforge/internal/domain"
	"catforge/internal/gamecontent"
)

// HuntStory composes a deterministic story from editable content, the action
// flavor and the cat trait.
func HuntStory(enc domain.Encounter, trait domain.Trait, flavor uint16) string {
	story := render("training.intro", flavor, 0) + " " +
		render(trainingEncounterKey(enc), flavor, 37) + ". " +
		render("training.outcome", flavor, 71)
	if key := traitReactionKey(trait); key != "" {
		story += " " + render(key, flavor, 89)
	}
	if flavor%100 == 0 {
		story += " " + render("training.absurd", flavor, 113)
	}
	return story
}

func render(key string, flavor uint16, salt uint64) string {
	return gamecontent.Render(key, uint64(flavor)+salt, nil)
}

func trainingEncounterKey(enc domain.Encounter) string {
	switch enc {
	case domain.EncounterPigeon:
		return "training.encounter.pigeon"
	case domain.EncounterLizard:
		return "training.encounter.lizard"
	case domain.EncounterBigRat:
		return "training.encounter.big_rat"
	default:
		return "training.encounter.mice_pack"
	}
}

func traitReactionKey(trait domain.Trait) string {
	switch trait {
	case domain.TraitLazy:
		return "trait.lazy.reaction"
	case domain.TraitBully:
		return "trait.bully.reaction"
	case domain.TraitPhilosopher:
		return "trait.philosopher.reaction"
	case domain.TraitNeat:
		return "trait.neat.reaction"
	case domain.TraitSleepy:
		return "trait.sleepy.reaction"
	default:
		return ""
	}
}
