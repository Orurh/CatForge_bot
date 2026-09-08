package telegram

import (
	"catforge/internal/domain"
	"catforge/internal/gamecontent"
)

func yardEventTypeOrDefault(eventType domain.YardEventType) domain.YardEventType {
	switch eventType {
	case domain.YardEventFishTruck, domain.YardEventBigDog, domain.YardEventBigBox:
		return eventType
	default:
		return domain.YardEventFishTruck
	}
}

func yardEventContentKey(eventType domain.YardEventType, suffix string) string {
	eventType = yardEventTypeOrDefault(eventType)
	key := "yard_event." + string(eventType) + "." + suffix
	if gamecontent.Has(key) {
		return key
	}
	return "yard_event." + string(domain.YardEventFishTruck) + "." + suffix
}
