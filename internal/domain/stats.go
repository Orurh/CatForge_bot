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
		return StatDelta{HP: 6, ATK: 1, DEF: 2, SPD: 1}
	case BreedSiamese: // "скорость"
		return StatDelta{HP: 2, ATK: 2, DEF: 1, SPD: 3}
	case BreedBritish: // "защита"
		return StatDelta{HP: 3, ATK: 1, DEF: 3, SPD: 1}
	case BreedBengal: // "урон"
		return StatDelta{HP: 2, ATK: 3, DEF: 1, SPD: 2}
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
