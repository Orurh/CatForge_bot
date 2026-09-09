package domain

import "slices"

type SpecialAction struct {
	ID             string
	Event          YardEventType
	Label          string
	RequiredEffect string
	Minimum        FelineStats
	Choice         YardEventChoiceID
	Requirement    string
}

var SpecialActions = []SpecialAction{
	{"service_entry", YardEventFishTruck, "🧹 Служебный вход", "service_entry", FelineStats{}, YardChoiceSteal, "Перчатка дворника"},
	{"bird_knowledge", YardEventFishTruck, "🐦 Договориться с птицами", "bird_knowledge", FelineStats{}, YardChoiceScout, "Голубиное перо"},
	{"dog_identity", YardEventBigDog, "🏷 Прикинуться своим", "dog_identity", FelineStats{}, YardChoiceDistract, "Собачий жетон"},
	{"foresight", YardEventBigDog, "👁 Заметить засаду", "foresight", FelineStats{}, YardChoiceScout, "Глаз старой рыси"},
	{"parkour", YardEventBigBox, "🧗 Маршрут по крыше", "parkour", FelineStats{}, YardChoiceSteal, "Кроссовочный шнурок"},
	{"ledge", YardEventBigBox, "🐾 Пролезть по карнизу", "", FelineStats{TailMM: 340}, YardChoiceScout, "Хвост 34 см"},
}

func SpecialActionByID(id string) (SpecialAction, bool) {
	for _, a := range SpecialActions {
		if a.ID == id {
			return a, true
		}
	}
	return SpecialAction{}, false
}
func (a SpecialAction) Available(s FelineStats, effects []string) bool {
	return (a.RequiredEffect == "" || slices.Contains(effects, a.RequiredEffect)) && s.ClawsTenthMM >= a.Minimum.ClawsTenthMM && s.WeightGrams >= a.Minimum.WeightGrams && s.TailMM >= a.Minimum.TailMM && s.WhiskerSpanMM >= a.Minimum.WhiskerSpanMM
}

func FeaturedSpecial(event YardEventType, seed int64) (SpecialAction, bool) {
	var candidates []SpecialAction
	for _, a := range SpecialActions {
		if a.Event == event {
			candidates = append(candidates, a)
		}
	}
	if len(candidates) == 0 {
		return SpecialAction{}, false
	}
	return candidates[uint64(seed)%uint64(len(candidates))], true
}
