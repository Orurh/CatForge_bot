package postgres

import (
	"context"
	"errors"
	"time"

	"catforge/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *YardRepo) ListBedtimeCandidates(ctx context.Context, now, sleepAt, activeSince time.Time, limit int) ([]domain.Yard, error) {
	if !now.Before(sleepAt) {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT y.id, y.telegram_chat_id, y.name, y.humor_mode, y.auto_messages_enabled,
		       y.max_auto_messages_day, y.cat_to_cat_banter, y.fights_enabled,
		       y.quiet_until, y.created_at, y.updated_at
		FROM yards y LEFT JOIN yard_bedtime_schedule s ON s.yard_id=y.id
		WHERE y.auto_messages_enabled AND y.cat_to_cat_banter AND y.max_auto_messages_day > 0
		  AND (y.quiet_until IS NULL OR y.quiet_until <= $1)
		  AND (s.sleep_at IS NULL OR s.sleep_at < $2)
		  AND EXISTS (SELECT 1 FROM yard_members m JOIN cat_personality p ON p.cat_id=m.cat_id
		              WHERE m.yard_id=y.id AND m.last_active_at >= $3 AND p.auto_speak_enabled)
		ORDER BY y.id LIMIT $4`, now, sleepAt, activeSince, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var yards []domain.Yard
	for rows.Next() {
		var yard domain.Yard
		if err := scanYard(rows, &yard); err != nil {
			return nil, err
		}
		yards = append(yards, yard)
	}
	return yards, rows.Err()
}

func (r *YardRepo) ClaimBedtimeFarewell(ctx context.Context, yardID, catID int64, now, sleepAt time.Time) (bool, error) {
	if !now.Before(sleepAt) {
		return false, nil
	}
	var claimed int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO yard_bedtime_schedule (yard_id, sleep_at)
		SELECT y.id, $3 FROM yards y
		WHERE y.id=$1 AND y.auto_messages_enabled AND y.cat_to_cat_banter AND y.max_auto_messages_day > 0
		  AND (y.quiet_until IS NULL OR y.quiet_until <= $2)
		  AND EXISTS (SELECT 1 FROM yard_members m JOIN cat_personality p ON p.cat_id=m.cat_id
		              WHERE m.yard_id=y.id AND m.cat_id=$4 AND p.auto_speak_enabled)
		ON CONFLICT (yard_id) DO UPDATE SET sleep_at=EXCLUDED.sleep_at
		WHERE yard_bedtime_schedule.sleep_at < EXCLUDED.sleep_at
		RETURNING yard_id`, yardID, now, sleepAt, catID).Scan(&claimed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
