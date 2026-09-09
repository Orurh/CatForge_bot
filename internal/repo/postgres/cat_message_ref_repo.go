package postgres

import (
	"context"
	"time"

	"catforge/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CatMessageRefRepo struct{ pool *pgxpool.Pool }

func NewCatMessageRefRepo(pool *pgxpool.Pool) *CatMessageRefRepo {
	return &CatMessageRefRepo{pool: pool}
}

func (r *CatMessageRefRepo) Remember(ctx context.Context, chatID int64, messageID int, catID int64, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		WITH expired AS (
			DELETE FROM cat_message_refs WHERE expires_at <= now()
		)
		INSERT INTO cat_message_refs (
			telegram_chat_id, telegram_message_id, cat_id, expires_at, claimed_at
		)
		VALUES ($1, $2, $3, $4, NULL)
		ON CONFLICT (telegram_chat_id, telegram_message_id) DO UPDATE
		SET cat_id = EXCLUDED.cat_id,
		    expires_at = EXCLUDED.expires_at,
		    claimed_at = NULL,
		    created_at = now()
	`, chatID, messageID, catID, expiresAt)
	return err
}

func (r *CatMessageRefRepo) Claim(ctx context.Context, chatID int64, messageID int, now time.Time) (*domain.CatMessageReference, bool, error) {
	var ref domain.CatMessageReference
	err := r.pool.QueryRow(ctx, `
		UPDATE cat_message_refs
		SET claimed_at = $3
		WHERE telegram_chat_id = $1
		  AND telegram_message_id = $2
		  AND claimed_at IS NULL
		  AND expires_at > $3
		RETURNING telegram_chat_id, telegram_message_id, cat_id, expires_at, claimed_at
	`, chatID, messageID, now).Scan(
		&ref.TelegramChatID, &ref.TelegramMessageID, &ref.CatID, &ref.ExpiresAt, &ref.ClaimedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &ref, true, nil
}
