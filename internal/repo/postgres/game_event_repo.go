package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"catforge/internal/app"

	"github.com/jackc/pgx/v5/pgxpool"
)

type GameEventRepo struct{ pool *pgxpool.Pool }

func NewGameEventRepo(pool *pgxpool.Pool) *GameEventRepo { return &GameEventRepo{pool: pool} }

func (r *GameEventRepo) Publish(ctx context.Context, event app.GameEvent) error {
	if event.DedupeKey == "" {
		return fmt.Errorf("game event dedupe key is empty")
	}
	if event.PayloadVersion == 0 {
		event.PayloadVersion = app.GameEventPayloadVersion
	}
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal game event payload: %w", err)
	}
	var catID any
	if event.CatID != 0 {
		catID = event.CatID
	}
	var userID any
	if event.UserID != 0 {
		userID = event.UserID
	}
	var yardID any
	if event.YardID != 0 {
		yardID = event.YardID
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO game_events (
			event_key, user_id, cat_id, yard_id, event_type, occurred_at,
			rules_version, content_version, payload_version, notable, properties
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb)
		ON CONFLICT DO NOTHING
	`, event.DedupeKey, userID, catID, yardID, string(event.Kind), event.OccurredAt,
		event.RulesVersion, event.ContentVersion, event.PayloadVersion, event.Notable, payload)
	if err != nil {
		return fmt.Errorf("insert game event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return app.ErrDuplicateGameEvent
	}
	return nil
}
