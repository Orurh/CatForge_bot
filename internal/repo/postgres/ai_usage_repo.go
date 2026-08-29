package postgres

import (
	"context"

	"catforge/internal/ai"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AIUsageRepo struct{ pool *pgxpool.Pool }

func NewAIUsageRepo(pool *pgxpool.Pool) *AIUsageRepo { return &AIUsageRepo{pool: pool} }

func (r *AIUsageRepo) RecordAIUsage(ctx context.Context, record ai.UsageRecord) error {
	var catID any
	if record.CatID != 0 {
		catID = record.CatID
	}
	var yardID any
	if record.YardID != 0 {
		yardID = record.YardID
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO ai_usage_log (
			generation_type, cat_id, yard_id, provider, model, tokens_in, tokens_out,
			latency_ms, success, blocked, fallback, error_code, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, string(record.GenerationType), catID, yardID, record.Provider, record.Model,
		record.TokensIn, record.TokensOut, record.Latency.Milliseconds(), record.Success,
		record.Blocked, record.Fallback, record.ErrorCode, record.CreatedAt)
	return err
}
