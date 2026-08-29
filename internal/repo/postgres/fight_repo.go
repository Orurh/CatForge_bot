package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"catforge/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FightRepo struct{ pool *pgxpool.Pool }

func NewFightRepo(pool *pgxpool.Pool) *FightRepo { return &FightRepo{pool: pool} }

func (r *FightRepo) ToggleQueue(ctx context.Context, yardID, userID, catID int64, now, expiresAt time.Time) (domain.FightQueueToggle, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.FightQueueToggle{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Serialize arena toggles per yard. Without this lock, two simultaneous
	// empty-arena requests could both insert a waiter instead of matching.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, yardID); err != nil {
		return domain.FightQueueToggle{}, err
	}

	var member bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM yard_members
			WHERE yard_id = $1 AND user_id = $2 AND cat_id = $3
		)
	`, yardID, userID, catID).Scan(&member); err != nil {
		return domain.FightQueueToggle{}, err
	}
	if !member {
		return domain.FightQueueToggle{}, domain.ErrNotYardMember
	}
	if _, err := tx.Exec(ctx, `DELETE FROM yard_fight_queue WHERE expires_at <= $1`, now); err != nil {
		return domain.FightQueueToggle{}, err
	}

	var removed int64
	err = tx.QueryRow(ctx, `
		DELETE FROM yard_fight_queue
		WHERE yard_id = $1 AND cat_id = $2
		RETURNING cat_id
	`, yardID, catID).Scan(&removed)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return domain.FightQueueToggle{}, err
		}
		return domain.FightQueueToggle{Status: domain.FightQueueCanceled}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.FightQueueToggle{}, err
	}

	var opponent domain.FightQueueEntry
	err = tx.QueryRow(ctx, `
		SELECT yard_id, user_id, cat_id, queued_at, expires_at
		FROM yard_fight_queue
		WHERE yard_id = $1 AND cat_id <> $2 AND expires_at > $3
		ORDER BY queued_at, cat_id
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, yardID, catID, now).Scan(
		&opponent.YardID, &opponent.UserID, &opponent.CatID, &opponent.QueuedAt, &opponent.ExpiresAt,
	)
	if err == nil {
		if _, err := tx.Exec(ctx, `
			DELETE FROM yard_fight_queue WHERE yard_id = $1 AND cat_id = $2
		`, yardID, opponent.CatID); err != nil {
			return domain.FightQueueToggle{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.FightQueueToggle{}, err
		}
		return domain.FightQueueToggle{Status: domain.FightQueueMatched, Opponent: opponent}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.FightQueueToggle{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO yard_fight_queue (yard_id, user_id, cat_id, queued_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, yardID, userID, catID, now, expiresAt); err != nil {
		return domain.FightQueueToggle{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.FightQueueToggle{}, err
	}
	return domain.FightQueueToggle{Status: domain.FightQueueWaiting}, nil
}

func (r *FightRepo) SaveFight(ctx context.Context, record domain.FightRecord, result domain.FightResult, catAName, catBName string) (int64, int, error) {
	turns, err := json.Marshal(result.Turns)
	if err != nil {
		return 0, 0, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	winnerName, loserName := catAName, catBName
	if result.WinnerCatID == record.CatBID {
		winnerName, loserName = catBName, catAName
	}
	var fightID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO yard_fights (
			yard_id, cat_a_id, cat_b_id, cat_a_name, cat_b_name,
			winner_cat_id, loser_cat_id, winner_name, loser_name, seed,
			rounds, final_hp_a, final_hp_b, turns, rivalry_delta,
			rules_version, content_version, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18
		) RETURNING id
	`, record.YardID, record.CatAID, record.CatBID, catAName, catBName,
		result.WinnerCatID, result.LoserCatID, winnerName, loserName, record.Seed,
		result.Rounds, result.FinalHPA, result.FinalHPB, turns, record.RivalryDelta,
		record.RulesVersion, record.ContentVersion, record.CreatedAt).Scan(&fightID)
	if err != nil {
		return 0, 0, err
	}

	catAID, catBID := record.CatAID, record.CatBID
	if catAID > catBID {
		catAID, catBID = catBID, catAID
	}
	var rivalry int
	err = tx.QueryRow(ctx, `
		INSERT INTO cat_relationships (cat_a_id, cat_b_id, rivalry, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (cat_a_id, cat_b_id) DO UPDATE
		SET rivalry = cat_relationships.rivalry + EXCLUDED.rivalry,
		    updated_at = EXCLUDED.updated_at
		RETURNING rivalry
	`, catAID, catBID, record.RivalryDelta, record.CreatedAt).Scan(&rivalry)
	if err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return fightID, rivalry, nil
}
