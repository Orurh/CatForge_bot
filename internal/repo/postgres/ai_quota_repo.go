package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AIQuotaRepo struct{ pool *pgxpool.Pool }

func NewAIQuotaRepo(pool *pgxpool.Pool) *AIQuotaRepo { return &AIQuotaRepo{pool: pool} }

func (r *AIQuotaRepo) AllowAIRequest(ctx context.Context, userID, chatID int64, now time.Time, userLimit, chatLimit int) (bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	bucket := now.UTC().Truncate(time.Hour)
	allowed, err := consumeAIQuota(ctx, tx, "user", userID, bucket, userLimit)
	if err != nil || !allowed {
		return allowed, err
	}
	allowed, err = consumeAIQuota(ctx, tx, "chat", chatID, bucket, chatLimit)
	if err != nil || !allowed {
		return allowed, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func consumeAIQuota(ctx context.Context, tx pgx.Tx, scopeType string, scopeID int64, bucket time.Time, limit int) (bool, error) {
	var used int
	err := tx.QueryRow(ctx, `
		INSERT INTO ai_rate_limits (scope_type, scope_id, bucket_start, used)
		VALUES ($1, $2, $3, 1)
		ON CONFLICT (scope_type, scope_id, bucket_start)
		DO UPDATE SET used = ai_rate_limits.used + 1
		WHERE ai_rate_limits.used < $4
		RETURNING used
	`, scopeType, scopeID, bucket, limit).Scan(&used)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("consume AI quota: %w", err)
	}
	return true, nil
}
