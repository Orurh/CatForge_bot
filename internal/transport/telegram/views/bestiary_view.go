package views

import (
	"fmt"
	"strings"

	"catforge/internal/domain"
)

var bestiaryGroups = []struct {
	location string
	kinds    []domain.EnemyKind
}{
	{"🛹 Переулки", []domain.EnemyKind{domain.EnemySewerRat, domain.EnemyRatAccountant}},
	{"🏙 Крыши", []domain.EnemyKind{domain.EnemyStrayDog, domain.EnemyCourierDog}},
	{"🌲 Старый парк", []domain.EnemyKind{domain.EnemyWildLynx, domain.EnemyMoonLynx}},
}

func FormatBestiary(entries []domain.BestiaryEntry) string {
	seen := make(map[domain.EnemyKind]domain.BestiaryEntry, len(entries))
	for _, entry := range entries {
		seen[entry.EnemyKind] = entry
	}
	lines := []string{fmt.Sprintf("📖 Бестиарий • открыто %d/6", len(seen)), ""}
	for _, group := range bestiaryGroups {
		lines = append(lines, group.location)
		for _, kind := range group.kinds {
			entry, ok := seen[kind]
			if !ok {
				lines = append(lines, "❔ Неизвестный противник")
				continue
			}
			mark := "✅"
			if domain.IsRareEnemy(kind) {
				mark = "✨"
			}
			lines = append(lines, fmt.Sprintf("%s %s • встреч %d • побед %d", mark, EnemyName(kind), entry.Encounters, entry.Victories))
		}
		lines = append(lines, "")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func EnemyName(kind domain.EnemyKind) string {
	switch kind {
	case domain.EnemyStrayDog:
		return "Бродячая собака"
	case domain.EnemyWildLynx:
		return "Дикая рысь"
	case domain.EnemyRatAccountant:
		return "Крысиный Бухгалтер"
	case domain.EnemyCourierDog:
		return "Пёс-курьер"
	case domain.EnemyMoonLynx:
		return "Лунная рысь"
	default:
		return "Канализационная крыса"
	}
}
