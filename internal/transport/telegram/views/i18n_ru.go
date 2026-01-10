package views

import "catforge/internal/domain"

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
		return "ленивый"
	case "bully":
		return "задира"
	case "philosopher":
		return "философ"
	case "neat":
		return "аккуратист"
	case "sleepy":
		return "соня"
	default:
		return string(t)
	}
}
