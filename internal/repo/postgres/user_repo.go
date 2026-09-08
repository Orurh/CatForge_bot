package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"catforge/internal/app"
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

func (r *UserRepo) GetPendingInput(ctx context.Context, userID, chatID int64) (*app.PendingInput, error) {
	var input app.PendingInput
	err := r.pool.QueryRow(ctx, `
		SELECT user_id, telegram_chat_id, kind, prompt_message_id, expires_at
		FROM pending_inputs
		WHERE user_id = $1 AND telegram_chat_id = $2 AND expires_at > now()
	`, userID, chatID).Scan(&input.UserID, &input.ChatID, &input.Kind, &input.PromptMessageID, &input.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &input, nil
}

func (r *UserRepo) SavePendingInput(ctx context.Context, input app.PendingInput) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO pending_inputs (user_id, telegram_chat_id, kind, prompt_message_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, telegram_chat_id) DO UPDATE
		SET kind = EXCLUDED.kind,
		    prompt_message_id = EXCLUDED.prompt_message_id,
		    expires_at = EXCLUDED.expires_at
	`, input.UserID, input.ChatID, input.Kind, input.PromptMessageID, input.ExpiresAt)
	return err
}

func (r *UserRepo) ClearPendingInput(ctx context.Context, userID, chatID int64) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM pending_inputs WHERE user_id = $1 AND telegram_chat_id = $2
	`, userID, chatID)
	return err
}

func (r *UserRepo) ClaimDailyCommand(ctx context.Context, userID, chatID int64, command string, now time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO command_daily_usage
		(user_id, telegram_chat_id, command_name, usage_day, used_at)
		VALUES ($1, $2, $3, $4::date, $5)
		ON CONFLICT DO NOTHING
	`, userID, chatID, command, now.Format("2006-01-02"), now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
