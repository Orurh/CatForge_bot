package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TelegramUpdateRepo struct {
	pool *pgxpool.Pool
}

func NewTelegramUpdateRepo(pool *pgxpool.Pool) *TelegramUpdateRepo {
	return &TelegramUpdateRepo{pool: pool}
}

// Claim records an update before it is handled. Telegram may deliver the same
// update more than once, so only the first insert is allowed to execute it.
func (r *TelegramUpdateRepo) Claim(ctx context.Context, updateID int64) (bool, error) {
	ct, err := r.pool.Exec(ctx, `
		INSERT INTO telegram_updates (update_id)
		VALUES ($1)
		ON CONFLICT (update_id) DO NOTHING
	`, updateID)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() == 1, nil
}
