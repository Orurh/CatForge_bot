package narrative

import (
	battletext "catforge/internal/battle"
	"hash/fnv"
	"math/rand"
	"strings"
)

func ArenaFightStory(attacker, defender, seed string, attackerWon bool, rageAfter int) []string {
	r := rand.New(rand.NewSource(int64(hash64(seed))))

	lines := []string{
		"⚔️ Начинается битва ⚔️",
		replaceAB(pickRand(r, battletext.BattleIntro), attacker, defender),
	}

	turns := 3 + r.Intn(4) // 3..6
	needShield := true
	needFail := true

	for i := 0; i < turns; i++ {
		actor, target := attacker, defender
		if i%2 == 1 {
			actor, target = defender, attacker
		}

		remaining := turns - i
		if needShield && remaining <= 2 {
			lines = append(lines, "🛡 "+replaceAB(pickRand(r, battletext.Defense), actor, target))
			needShield = false
			continue
		}
		if needFail && remaining <= 2 {
			lines = append(lines, "🤡 "+replaceAB(pickRand(r, battletext.Fail), actor, target))
			needFail = false
			continue
		}

		roll := r.Intn(100)
		switch {
		case roll < 18:
			lines = append(lines, "🛡 "+replaceAB(pickRand(r, battletext.AttackTrick), actor, target))
			needShield = false
		case roll < 30:
			lines = append(lines, "🤡 "+replaceAB(pickRand(r, battletext.Fail), actor, target))
			needFail = false
		case roll < 42:
			lines = append(lines, "🧶 "+replaceAB(pickRand(r, battletext.AttackTrick), actor, target))
		case roll < 52:
			lines = append(lines, "✨ "+replaceAB(pickRand(r, battletext.Magic), actor, target))
		case roll < 68:
			lines = append(lines, "😼 "+replaceAB(pickRand(r, battletext.AttackMock), actor, target))
		case roll < 82 && rageAfter > 0:
			lines = append(lines, "😾 "+replaceAB(pickRand(r, battletext.AttackRage), actor, target))
		case roll < 92:
			lines = append(lines, "🐾🐾 "+replaceAB(pickRand(r, battletext.AttackStrong), actor, target))
		default:
			lines = append(lines, "🐾 "+replaceAB(pickRand(r, battletext.AttackNormal), actor, target))
		}
	}

	winner, loser := attacker, defender
	if !attackerWon {
		winner, loser = defender, attacker
	}
	lines = append(lines, "💥 "+replaceAB(pickRand(r, battletext.FinishWin), winner, loser))

	return lines
}

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func pickRand(r *rand.Rand, list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[r.Intn(len(list))]
}

func replaceAB(s, a, b string) string {
	return strings.NewReplacer("{A}", a, "{B}", b).Replace(s)
}

