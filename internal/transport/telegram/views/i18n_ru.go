package views

import "catforge/internal/domain"

func BreedIcon(b domain.Breed) string {
	switch b {
	case domain.BreedMaineCoon:
		return "🦁"
	case domain.BreedSiamese:
		return "🐈‍⬛"
	case domain.BreedBritish:
		return "🐻"
	case domain.BreedBengal:
		return "🐆"
	default:
		return "🐱"
	}
}

func BreedRU(b domain.Breed) string {
	switch b {
	case domain.BreedMaineCoon:
		return "мейн-кун"
	case domain.BreedSiamese:
		return "сиам"
	case domain.BreedBritish:
		return "британец"
	case domain.BreedBengal:
		return "бенгал"
	default:
		return string(b)
	}
}

func TraitRU(t domain.Trait) string {
	switch t {
	case domain.TraitLazy:
		return "лентяй"
	case domain.TraitBully:
		return "борзый"
	case domain.TraitVeteran:
		return "бывалый"
	case domain.TraitCunning:
		return "хитрый"
	case domain.TraitArrogant:
		return "наглый"
	case domain.TraitAngry:
		return "злой"
	default:
		return string(t)
	}
}
