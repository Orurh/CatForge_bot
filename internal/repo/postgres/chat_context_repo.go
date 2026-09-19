package postgres

import (
	"context"
	"strings"
	"time"

	"catforge/internal/ai"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatContextRepo struct{ pool *pgxpool.Pool }

func NewChatContextRepo(pool *pgxpool.Pool) *ChatContextRepo { return &ChatContextRepo{pool: pool} }

func (r *ChatContextRepo) Remember(ctx context.Context, chatID int64, author, text string, now time.Time) error {
	text = strings.TrimSpace(text)
	if chatID == 0 || text == "" {
		return nil
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO chat_reaction_context(telegram_chat_id,author,text,created_at) VALUES($1,$2,$3,$4)`, chatID, strings.TrimSpace(author), []rune(text)[:min(len([]rune(text)), 500)], now)
	return err
}

func (r *ChatContextRepo) Recent(ctx context.Context, chatID int64, limit int) ([]ai.ChatMessage, error) {
	if limit <= 0 || limit > 8 {
		limit = 8
	}
	rows, err := r.pool.Query(ctx, `SELECT author,text FROM (SELECT author,text,id FROM chat_reaction_context WHERE telegram_chat_id=$1 ORDER BY id DESC LIMIT $2) recent ORDER BY id`, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []ai.ChatMessage
	for rows.Next() {
		var m ai.ChatMessage
		if err := rows.Scan(&m.Author, &m.Text); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
