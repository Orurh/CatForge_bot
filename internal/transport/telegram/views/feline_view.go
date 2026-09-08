package views

import (
	"catforge/internal/domain"
	"catforge/internal/gamedata"
	"fmt"
	"strings"
)

func tenths(value int) string { return fmt.Sprintf("%d.%d", value/10, value%10) }
func FormatFelineStats(s domain.FelineStats) string {
	return "🩸 Когти: " + tenths(s.ClawsTenthMM) + " мм\n🐈 Вес: " + tenths(s.WeightGrams/100) + " кг\n🐾 Хвост: " + tenths(s.TailMM) + " см\n〰️ Усы: " + tenths(s.WhiskerSpanMM) + " см"
}
func FormatProgressionFacts(facts []string, loot string) string {
	var lines []string
	for _, fact := range facts {
		for _, u := range domain.ProgressionUnlocks {
			if fact == u.ID && fact != "first_item" {
				lines = append(lines, "✨ Открыто: "+u.Label+"!")
			}
		}
	}
	if d, ok := gamedata.ItemByID(loot); ok {
		line := "🎁 Находка: " + d.Name + "."
		for _, fact := range facts {
			if fact == "item_upgraded="+loot {
				line = "🧩 " + d.Name + " усилен."
			}
			if fact == "item_fragment="+loot {
				line = "🧩 Ещё один «" + d.Name + "» — копим усиление."
			}
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n" + strings.Join(lines, "\n")
}
