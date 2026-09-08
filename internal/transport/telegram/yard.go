package telegram

import (
	"strconv"
	"strings"

	"catforge/internal/app"
	"catforge/internal/transport/telegram/views"
)

const (
	yardMemberDisplayLimit       = 30
	yardRelationshipDisplayLimit = 5
)

func formatYardSnapshot(snapshot app.YardSnapshot) string {
	if snapshot.Yard == nil {
		return ""
	}
	lines := []string{"🏘 ДВОР", "«" + snapshot.Yard.Name + "»", ""}
	if snapshot.Created {
		lines = append(lines, "✨ Новый Двор создан.")
	}
	if snapshot.Joined {
		lines = append(lines, "🐾 Твой кот присоединился.")
	} else {
		lines = append(lines, "🐾 Твой кот уже здесь.")
	}

	lines = append(lines, "", "🐈 КОТЫ · "+strconv.Itoa(len(snapshot.Members)))
	memberLimit := min(yardMemberDisplayLimit, len(snapshot.Members))
	for index := 0; index < memberLimit; index++ {
		member := snapshot.Members[index]
		lines = append(lines,
			strconv.Itoa(index+1)+". "+publicCatBadge(member.Breed, member.Level)+" "+member.CatName,
			"└ Характер: "+views.TraitRU(member.Trait),
		)
	}
	if hidden := len(snapshot.Members) - memberLimit; hidden > 0 {
		lines = append(lines, "…и ещё: "+strconv.Itoa(hidden))
	}

	if len(snapshot.Relationships) > 0 {
		lines = append(lines, "", "🤝 СВЯЗИ")
		relationshipLimit := min(yardRelationshipDisplayLimit, len(snapshot.Relationships))
		for index := 0; index < relationshipLimit; index++ {
			relationship := snapshot.Relationships[index]
			if index > 0 {
				lines = append(lines, "")
			}
			lines = append(lines,
				relationship.CatAName+" ↔ "+relationship.CatBName,
				"├ 🤝 Дружба: "+strconv.Itoa(relationship.Friendship),
				"├ ⚔️ Соперничество: "+strconv.Itoa(relationship.Rivalry),
				"└ 🏅 Уважение: "+strconv.Itoa(relationship.Respect),
			)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
