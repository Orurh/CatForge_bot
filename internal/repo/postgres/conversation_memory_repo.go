package postgres

import (
	"context"

	"catforge/internal/ai"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConversationMemoryRepo struct{ pool *pgxpool.Pool }

func NewConversationMemoryRepo(pool *pgxpool.Pool) *ConversationMemoryRepo {
	return &ConversationMemoryRepo{pool: pool}
}

func (r *ConversationMemoryRepo) Recent(ctx context.Context, scope string, catID, otherCatID int64) ([]ai.SpeechMemory, error) {
	rows, err := r.pool.Query(ctx, `
		WITH scoped AS (
		 SELECT * FROM cat_speech_memory WHERE scope_key=$1 AND created_at >= now()-interval '7 days'
		), recent AS (
		 (SELECT id FROM scoped ORDER BY id DESC LIMIT 5)
		 UNION (SELECT id FROM scoped WHERE cat_id=$2 OR other_cat_id=$2 ORDER BY id DESC LIMIT 3)
		 UNION (SELECT id FROM scoped WHERE $3::bigint>0 AND (cat_id=$3 OR other_cat_id=$3) ORDER BY id DESC LIMIT 3)
		)
		SELECT cat_id, COALESCE(other_cat_id,0), cat_name, other_cat_name, speech
		FROM scoped JOIN recent USING(id) ORDER BY id
	`, scope, catID, otherCatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var history []ai.SpeechMemory
	for rows.Next() {
		var line ai.SpeechMemory
		if err := rows.Scan(&line.CatID, &line.OtherCatID, &line.CatName, &line.OtherCatName, &line.Text); err != nil {
			return nil, err
		}
		history = append(history, line)
	}
	return history, rows.Err()
}

func (r *ConversationMemoryRepo) Remember(ctx context.Context, scope string, speech ai.SpeechMemory) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serialize retention in the same scope, including concurrent human replies.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "cat-speech:"+scope); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
	 INSERT INTO cat_speech_memory(scope_key,cat_id,other_cat_id,cat_name,other_cat_name,speech)
	 VALUES($1,$2,NULLIF($3,0),$4,$5,$6)
	`, scope, speech.CatID, speech.OtherCatID, speech.CatName, speech.OtherCatName, speech.Text); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
	 DELETE FROM cat_speech_memory WHERE scope_key=$1 AND
	 (created_at < now()-interval '7 days' OR id IN
	  (SELECT id FROM cat_speech_memory WHERE scope_key=$1 ORDER BY id DESC OFFSET 30))
	`, scope); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
