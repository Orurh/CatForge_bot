package domain

// StatDelta describes stat changes (e.g. from leveling up).
type StatDelta struct {
	HP  int
	ATK int
	DEF int
	SPD int
}

func (d *StatDelta) Add(x StatDelta) {
	d.HP += x.HP
	d.ATK += x.ATK
	d.DEF += x.DEF
	d.SPD += x.SPD
}

func LevelUpDelta(b Breed) StatDelta {
	// Детерминированная прокачка: легко балансить, легко тестировать.
	switch b {
	case BreedMaineCoon: // "танк"
		return StatDelta{HP: 4, ATK: 2, DEF: 2, SPD: 1}
	case BreedSiamese: // "скорость"
		return StatDelta{HP: 4, ATK: 2, DEF: 1, SPD: 2}
	case BreedBritish: // "защита"
		return StatDelta{HP: 3, ATK: 2, DEF: 2, SPD: 1}
	case BreedBengal: // "урон"
		return StatDelta{HP: 3, ATK: 2, DEF: 1, SPD: 2}
	default:
		return StatDelta{HP: 3, ATK: 2, DEF: 2, SPD: 2}
	}
}

// ApplyLevelUps mutates cat base stats and returns total gained delta.
func ApplyLevelUps(c *Cat, levels int) StatDelta {
	var total StatDelta
	if c == nil || levels <= 0 {
		return total
	}

	step := LevelUpDelta(c.Breed)
	for i := 0; i < levels; i++ {
		c.HPBase += step.HP
		c.ATKBase += step.ATK
		c.DEFBase += step.DEF
		c.SPDBase += step.SPD
		total.Add(step)
	}
	return total
}

type XPProgress struct {
	XPGain       int64
	LevelsGained int
	StatsGained  StatDelta
	Level        int
	XP           int64
}

// GrantXP is retained for legacy simulations. Live social rewards use the
// authoritative C++ Progress RPC, including physical stats and unlocks.
func GrantXP(c *Cat, gain int64) XPProgress {
	progress := XPProgress{XPGain: gain}
	if c == nil || gain <= 0 {
		if c != nil {
			progress.Level, progress.XP = c.Level, c.XP
		}
		return progress
	}
	c.XP += gain
	for c.XP >= int64(c.Level)*100 {
		c.XP -= int64(c.Level) * 100
		c.Level++
		progress.LevelsGained++
		progress.StatsGained.Add(ApplyLevelUps(c, 1))
	}
	progress.Level, progress.XP = c.Level, c.XP
	return progress
}
