package app

import (
	"strings"
	"testing"

	"catforge/internal/domain"
)

func TestRelationshipContextsOrientSpeakerAndApplyLimit(t *testing.T) {
	t.Parallel()
	relationships := []domain.CatRelationship{
		{CatAID: 1, CatBID: 42, CatAName: "Батон", CatBName: "Барсик", Friendship: 2, Rivalry: 8, Respect: 3},
		{CatAID: 42, CatBID: 3, CatAName: "Барсик", CatBName: "Сметана", Friendship: 7},
		{CatAID: 8, CatBID: 9, CatAName: "Чужой", CatBName: "Другой", Rivalry: 99},
	}
	contexts := relationshipContextsForCat(42, relationships, 1)
	if len(contexts) != 1 || contexts[0].CatAName != "Барсик" || contexts[0].CatBName != "Батон" || contexts[0].Rivalry != 8 {
		t.Fatalf("relationshipContextsForCat() = %+v", contexts)
	}
}

func TestEventNarrativeIncludesCurrentRelationshipTotals(t *testing.T) {
	t.Parallel()
	input := domain.YardEventResolutionInput{
		Event: domain.YardEvent{YardID: 9, Type: domain.YardEventFishTruck},
		Participants: []domain.YardEventParticipant{
			{CatID: 42, CatName: "Барсик"},
			{CatID: 50, CatName: "Батон"},
		},
	}
	result := domain.YardEventResult{
		Participants:        []domain.YardEventParticipantResult{{CatID: 42}, {CatID: 50}},
		RelationshipEffects: []domain.YardRelationshipEffect{{CatAID: 42, CatBID: 50, FriendshipDelta: 1}},
	}
	request := eventNarrativeRequest(input, result, []domain.CatRelationship{{
		CatAID: 42, CatBID: 50, CatAName: "Барсик", CatBName: "Батон",
		Friendship: 6, Rivalry: 2, Respect: 3,
	}})
	if len(request.Relationships) != 1 || request.Relationships[0].Friendship != 6 || request.Relationships[0].Respect != 3 {
		t.Fatalf("event relationship context = %+v", request.Relationships)
	}
	if !strings.Contains(strings.Join(request.EventFacts, "\n"), "friendship_delta=1") {
		t.Fatalf("event relationship delta missing: %v", request.EventFacts)
	}
}
