package app

import (
	"catforge/internal/ai"
	"catforge/internal/domain"
)

const maxAIRelationships = 5

func relationshipContextsForCat(catID int64, relationships []domain.CatRelationship, limit int) []ai.RelationshipContext {
	if limit <= 0 || limit > maxAIRelationships {
		limit = maxAIRelationships
	}
	contexts := make([]ai.RelationshipContext, 0, limit)
	for _, relationship := range relationships {
		var catName, otherName string
		switch catID {
		case relationship.CatAID:
			catName, otherName = relationship.CatAName, relationship.CatBName
		case relationship.CatBID:
			catName, otherName = relationship.CatBName, relationship.CatAName
		default:
			continue
		}
		contexts = append(contexts, ai.RelationshipContext{
			CatAName: catName, CatBName: otherName,
			Friendship: relationship.Friendship, Rivalry: relationship.Rivalry, Respect: relationship.Respect,
		})
		if len(contexts) == limit {
			break
		}
	}
	return contexts
}

func relationshipContextsAmong(catIDs map[int64]struct{}, relationships []domain.CatRelationship) []ai.RelationshipContext {
	contexts := make([]ai.RelationshipContext, 0, maxAIRelationships)
	for _, relationship := range relationships {
		if _, ok := catIDs[relationship.CatAID]; !ok {
			continue
		}
		if _, ok := catIDs[relationship.CatBID]; !ok {
			continue
		}
		contexts = append(contexts, ai.RelationshipContext{
			CatAName: relationship.CatAName, CatBName: relationship.CatBName,
			Friendship: relationship.Friendship, Rivalry: relationship.Rivalry, Respect: relationship.Respect,
		})
		if len(contexts) == maxAIRelationships {
			break
		}
	}
	return contexts
}
