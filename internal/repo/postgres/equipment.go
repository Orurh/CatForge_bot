package postgres

import (
	"catforge/internal/domain"
	"catforge/internal/gamedata"
	"context"
	"github.com/jackc/pgx/v5"
)

type equipmentQuery interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func physicalEquipment(ctx context.Context, q equipmentQuery, userID int64, level int) (domain.FelineStats, []string, error) {
	rows, err := q.Query(ctx, `SELECT e.item_id,i.item_level FROM cat_equipment e JOIN cat_items i ON i.user_id=e.user_id AND i.item_id=e.item_id WHERE e.user_id=$1 ORDER BY e.slot`, userID)
	if err != nil {
		return domain.FelineStats{}, nil, err
	}
	defer rows.Close()
	var total domain.FelineStats
	var effects []string
	for rows.Next() {
		var id string
		var itemLevel int
		if err = rows.Scan(&id, &itemLevel); err != nil {
			return total, nil, err
		}
		if d, ok := gamedata.ItemByID(id); ok && domain.SlotUnlocked(d.Slot, level) {
			total.Add(d.PhysicalBonus(itemLevel))
			effects = append(effects, d.EffectID)
		}
	}
	return total, effects, rows.Err()
}

func trainingCritBonus(ctx context.Context, q equipmentQuery, userID int64, level int) (int, error) {
	rows, err := q.Query(ctx, `SELECT item_id FROM cat_equipment WHERE user_id=$1`, userID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	bonus := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		if d, ok := gamedata.ItemByID(id); ok && domain.SlotUnlocked(d.Slot, level) {
			bonus += min(max(d.TrainingCritBonusPercent, 0), 3)
		}
	}
	return min(bonus, 5), rows.Err()
}
