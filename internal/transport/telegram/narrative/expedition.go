package narrative

import (
	"catforge/internal/domain"
	"catforge/internal/gamecontent"
)

// ExpeditionStory is retained for the frozen legacy Expedition feature. Its
// procedural lines live in gamecontent alongside the active game texts.
func ExpeditionStory(result domain.ExpeditionResult, trait domain.Trait) string {
	flavor, rare := expeditionFlavor(result)
	story := render(expeditionIntroKey(result.Location), flavor, 0) + " " +
		render(expeditionEnemyKey(result.Enemy.Kind), flavor, 17) + ". "
	switch {
	case isCloseVictory(result):
		story += render("legacy_expedition.outcome.close", flavor, 31)
	case result.Outcome == domain.ExpeditionVictory:
		story += render("legacy_expedition.outcome.victory", flavor, 31)
	default:
		story += render("legacy_expedition.outcome.defeat", flavor, 31)
	}
	if key := traitReactionKey(trait); key != "" {
		story += " " + render(key, flavor, 47)
	}
	if result.Loot.Dropped {
		story += " " + render("legacy_expedition.loot", flavor, 59)
	}
	if rare {
		story += " " + gamecontent.Render("legacy_expedition.rare", uint64(flavor), nil)
	}
	return story
}

func expeditionIntroKey(location domain.ExpeditionLocation) string {
	switch location {
	case domain.ExpeditionRooftop:
		return "legacy_expedition.intro.rooftop"
	case domain.ExpeditionPark:
		return "legacy_expedition.intro.park"
	default:
		return "legacy_expedition.intro.alley"
	}
}

func expeditionEnemyKey(kind domain.EnemyKind) string {
	switch kind {
	case domain.EnemyStrayDog:
		return "legacy_expedition.enemy.stray_dog"
	case domain.EnemyWildLynx:
		return "legacy_expedition.enemy.wild_lynx"
	case domain.EnemyRatAccountant:
		return "legacy_expedition.enemy.rat_accountant"
	case domain.EnemyCourierDog:
		return "legacy_expedition.enemy.courier_dog"
	case domain.EnemyMoonLynx:
		return "legacy_expedition.enemy.moon_lynx"
	default:
		return "legacy_expedition.enemy.sewer_rat"
	}
}

func isCloseVictory(result domain.ExpeditionResult) bool {
	if result.Outcome != domain.ExpeditionVictory || result.CatHPAfter <= 0 {
		return false
	}
	initialHP := result.CatHPAfter
	for _, turn := range result.Turns {
		if turn.Actor == domain.BattleActorEnemy {
			initialHP = turn.DefenderHPAfter + turn.Damage
			break
		}
	}
	return initialHP > 0 && result.CatHPAfter*100 <= initialHP*10
}

func expeditionFlavor(result domain.ExpeditionResult) (uint16, bool) {
	hash := uint64(1469598103934665603)
	mix := func(value uint64) { hash ^= value + 0x9e3779b97f4a7c15 + (hash << 6) + (hash >> 2) }
	for _, value := range []byte(string(result.Location) + "|" + string(result.Difficulty) + "|" + string(result.Enemy.Kind) + "|" + result.Loot.ItemID) {
		mix(uint64(value))
	}
	for _, value := range []uint64{uint64(result.Outcome), uint64(result.Rounds), uint64(result.CatHPAfter), uint64(result.EnemyHPAfter), uint64(result.XPGain)} {
		mix(value)
	}
	return uint16(hash ^ hash>>16 ^ hash>>32), hash%100 == 0
}
