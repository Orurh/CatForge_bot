package domain

import "testing"

func TestNextYardEventTypeRotatesAllTemplates(t *testing.T) {
	t.Parallel()
	if got := NextYardEventType(""); got != YardEventFishTruck {
		t.Fatalf("first event = %q", got)
	}
	if got := NextYardEventType(YardEventFishTruck); got != YardEventBigDog {
		t.Fatalf("event after fish truck = %q", got)
	}
	if got := NextYardEventType(YardEventBigDog); got != YardEventBigBox {
		t.Fatalf("event after big dog = %q", got)
	}
	if got := NextYardEventType(YardEventBigBox); got != YardEventFishTruck {
		t.Fatalf("event after big box = %q", got)
	}
}
