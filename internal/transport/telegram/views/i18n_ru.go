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
	case "lazy":
		return "прокрастимятор"
	case "bully":
		return "царапыч"
	case "philosopher":
		return "философ"
	case "neat":
		return "облезлый"
	case "sleepy":
		return "хромуля"
	default:
		return string(t)
	}
}
