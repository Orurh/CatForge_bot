package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type GroupCooldownRepo struct{ pool *pgxpool.Pool }

func NewGroupCooldownRepo(pool *pgxpool.Pool) *GroupCooldownRepo {
	return &GroupCooldownRepo{pool: pool}
}

// Conditional upsert serializes callers; denied attempts never extend the timer.
func (r *GroupCooldownRepo) Claim(ctx context.Context, chatID, actorID int64, action string, now time.Time, interval time.Duration) (bool, time.Time, error) {
	var next time.Time
	err := r.pool.QueryRow(ctx, `INSERT INTO group_action_cooldowns(chat_id,actor_id,action,next_at)
 VALUES($1,$2,$3,$4::timestamptz + $5 * interval '1 microsecond')
 ON CONFLICT(chat_id,actor_id,action) DO UPDATE SET next_at = EXCLUDED.next_at
 WHERE group_action_cooldowns.next_at <= $4
 RETURNING next_at`, chatID, actorID, action, now, interval.Microseconds()).Scan(&next)
	if errors.Is(err, pgx.ErrNoRows) {
		err = r.pool.QueryRow(ctx, `SELECT next_at FROM group_action_cooldowns WHERE chat_id=$1 AND actor_id=$2 AND action=$3`, chatID, actorID, action).Scan(&next)
		// A failed action may release its reservation between the two queries.
		if errors.Is(err, pgx.ErrNoRows) {
			return false, now, nil
		}
		return false, next, err
	}
	return err == nil, next, err
}

func (r *GroupCooldownRepo) Release(ctx context.Context, chatID, actorID int64, action string, next time.Time) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM group_action_cooldowns WHERE chat_id=$1 AND actor_id=$2 AND action=$3 AND next_at=$4`, chatID, actorID, action, next)
	return err
}
