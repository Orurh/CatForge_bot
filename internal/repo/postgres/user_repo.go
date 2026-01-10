package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct{ pool *pgxpool.Pool }

func NewUserRepo(pool *pgxpool.Pool) *UserRepo { return &UserRepo{pool: pool} }

// EnsureUser создает пользователя, если его нет, и возвращает user_id (PK).
func (r *UserRepo) EnsureUser(ctx context.Context, telegramID int64) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id)
		VALUES ($1)
		ON CONFLICT (telegram_id) DO UPDATE SET last_seen_at = now()
		RETURNING id
	`, telegramID).Scan(&id)
	return id, err
}

func (r *UserRepo) GetPendingAction(ctx context.Context, userID int64) (string, error) {
	var a string
	err := r.pool.QueryRow(ctx, `
		SELECT pending_action
		FROM users
		WHERE id = $1
	`, userID).Scan(&a)
	return a, err
}

func (r *UserRepo) SetPendingAction(ctx context.Context, userID int64, action string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET pending_action = $2
		WHERE id = $1
	`, userID, action)
	return err
}

func (r *UserRepo) GetHomeChat(ctx context.Context, userID int64) (int64, string, error) {
	var chatID int64
	var chatType string
	err := r.pool.QueryRow(ctx, `
		SELECT home_chat_id, home_chat_type
		FROM users
		WHERE id = $1
	`, userID).Scan(&chatID, &chatType)
	return chatID, chatType, err
}

func (r *UserRepo) SetHomeChat(ctx context.Context, userID int64, chatID int64, chatType string) error {
	if chatID == 0 {
		_, err := r.pool.Exec(ctx, `
			UPDATE users
			SET home_chat_id = 0,
			    home_chat_type = '',
			    home_chat_bound_at = NULL,
			    home_chat_last_log_at = NULL
			WHERE id = $1
		`, userID)
		return err
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE users
		SET home_chat_id = $2,
		    home_chat_type = $3,
		    home_chat_bound_at = now()
		WHERE id = $1
	`, userID, chatID, chatType)
	return err
}

func (r *UserRepo) TryTouchHomeChatLog(ctx context.Context, userID int64, now time.Time, minInterval time.Duration) (bool, error) {
	threshold := now.Add(-minInterval)
	var ok int
	err := r.pool.QueryRow(ctx, `
		UPDATE users
		SET home_chat_last_log_at = $2
		WHERE id = $1
		  AND (home_chat_last_log_at IS NULL OR home_chat_last_log_at <= $3)
		RETURNING 1
	`, userID, now, threshold).Scan(&ok)
	if err != nil {
		return false, nil
	}
	return true, nil
}
