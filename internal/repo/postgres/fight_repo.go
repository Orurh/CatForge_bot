package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"catforge/internal/domain"
	"catforge/internal/gameengine"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FightRepo struct {
	pool   *pgxpool.Pool
	engine gameengine.ProgressionEngine
}

func NewFightRepo(pool *pgxpool.Pool, engines ...gameengine.ProgressionEngine) *FightRepo {
	r := &FightRepo{pool: pool}
	if len(engines) > 0 {
		r.engine = engines[0]
	}
	return r
}

func (r *FightRepo) CatStats(ctx context.Context, catID int64) (domain.ArenaStats, error) {
	var stats domain.ArenaStats
	err := r.pool.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE winner_cat_id = $1),
		       count(*) FILTER (WHERE loser_cat_id = $1)
		FROM yard_fights
		WHERE cat_a_id = $1 OR cat_b_id = $1
	`, catID).Scan(&stats.Fights, &stats.Wins, &stats.Losses)
	if err != nil {
		return domain.ArenaStats{}, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT winner_cat_id = $1
		FROM yard_fights
		WHERE cat_a_id = $1 OR cat_b_id = $1
		ORDER BY created_at DESC, id DESC
	`, catID)
	if err != nil {
		return domain.ArenaStats{}, err
	}
	defer rows.Close()
	first := true
	for rows.Next() {
		var won bool
		if err := rows.Scan(&won); err != nil {
			return domain.ArenaStats{}, err
		}
		if first {
			stats.StreakWins = won
			first = false
		}
		if won != stats.StreakWins {
			break
		}
		stats.CurrentStreak++
	}
	return stats, rows.Err()
}

func (r *FightRepo) ToggleQueue(ctx context.Context, yardID, userID, catID int64, now, expiresAt time.Time, limits domain.FightLimits) (domain.FightQueueToggle, error) {
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
	// A cat can belong to several yards. Use a separate negative lock key so
	// the global daily limit is serialized across all of them.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, -catID); err != nil {
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
	var dailyFights int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM yard_fights
		WHERE created_at >= $1 AND (cat_a_id = $2 OR cat_b_id = $2)
	`, limits.DailySince, catID).Scan(&dailyFights); err != nil {
		return domain.FightQueueToggle{}, err
	}
	if dailyFights >= limits.DailyLimit {
		return domain.FightQueueToggle{}, domain.ErrFightDailyLimit
	}

	var opponent domain.FightQueueEntry
	err = tx.QueryRow(ctx, `
		SELECT q.yard_id, q.user_id, q.cat_id, q.queued_at, q.expires_at
		FROM yard_fight_queue q
 JOIN cats opponent_cat ON opponent_cat.id=q.cat_id AND opponent_cat.level>=3
		WHERE q.yard_id = $1 AND q.cat_id <> $2 AND q.expires_at > $3
		  AND (
			SELECT count(*) FROM yard_fights daily
			WHERE daily.created_at >= $4
			  AND (daily.cat_a_id = q.cat_id OR daily.cat_b_id = q.cat_id)
		  ) < $5
		  AND (
			SELECT count(*) FROM yard_fights pair_fight
			WHERE pair_fight.yard_id = q.yard_id AND pair_fight.created_at >= $6
			  AND ((pair_fight.cat_a_id = $2 AND pair_fight.cat_b_id = q.cat_id)
			    OR (pair_fight.cat_a_id = q.cat_id AND pair_fight.cat_b_id = $2))
		  ) < $7
		ORDER BY q.queued_at, q.cat_id
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, yardID, catID, now, limits.DailySince, limits.DailyLimit, limits.PairSince, limits.PairLimit).Scan(
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
	var pairBlocked bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM yard_fight_queue q
 JOIN cats opponent_cat ON opponent_cat.id=q.cat_id AND opponent_cat.level>=3
			WHERE q.yard_id = $1 AND q.cat_id <> $2 AND q.expires_at > $3
			  AND (
				SELECT count(*) FROM yard_fights pair_fight
				WHERE pair_fight.yard_id = q.yard_id AND pair_fight.created_at >= $4
				  AND ((pair_fight.cat_a_id = $2 AND pair_fight.cat_b_id = q.cat_id)
				    OR (pair_fight.cat_a_id = q.cat_id AND pair_fight.cat_b_id = $2))
			  ) >= $5
		)
	`, yardID, catID, now, limits.PairSince, limits.PairLimit).Scan(&pairBlocked); err != nil {
		return domain.FightQueueToggle{}, err
	}
	if pairBlocked {
		return domain.FightQueueToggle{}, domain.ErrFightPairCooldown
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM yard_fight_queue q
		WHERE q.yard_id = $1 AND (
			SELECT count(*) FROM yard_fights daily
			WHERE daily.created_at >= $2
			  AND (daily.cat_a_id = q.cat_id OR daily.cat_b_id = q.cat_id)
		) >= $3
	`, yardID, limits.DailySince, limits.DailyLimit); err != nil {
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

func (r *FightRepo) GetRevenge(ctx context.Context, telegramChatID, sourceFightID, loserUserID int64, _ time.Time, limits domain.FightLimits) (domain.FightRevenge, error) {
	var revenge domain.FightRevenge
	err := r.pool.QueryRow(ctx, `
		SELECT source.id, source.yard_id, loser_member.user_id, source.loser_cat_id,
		       winner_member.user_id, source.winner_cat_id
		FROM yard_fights source
		JOIN yards y ON y.id = source.yard_id
		JOIN yard_members loser_member
		  ON loser_member.yard_id = source.yard_id AND loser_member.cat_id = source.loser_cat_id
		JOIN yard_members winner_member
		  ON winner_member.yard_id = source.yard_id AND winner_member.cat_id = source.winner_cat_id
		WHERE source.id = $1 AND y.telegram_chat_id = $2
		  AND loser_member.user_id = $3 AND source.created_at >= $4
		  AND NOT EXISTS (
			SELECT 1 FROM yard_fights child WHERE child.parent_fight_id = source.id
		  )
	`, sourceFightID, telegramChatID, loserUserID, limits.RevengeSince).Scan(
		&revenge.SourceFightID, &revenge.YardID, &revenge.LoserUserID, &revenge.LoserCatID,
		&revenge.WinnerUserID, &revenge.WinnerCatID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.FightRevenge{}, domain.ErrFightRevengeExpired
	}
	if err != nil {
		return domain.FightRevenge{}, err
	}
	for _, catID := range []int64{revenge.LoserCatID, revenge.WinnerCatID} {
		var count int
		if err := r.pool.QueryRow(ctx, `
			SELECT count(*) FROM yard_fights
			WHERE created_at >= $1 AND (cat_a_id = $2 OR cat_b_id = $2)
		`, limits.DailySince, catID).Scan(&count); err != nil {
			return domain.FightRevenge{}, err
		}
		if count >= limits.DailyLimit {
			return domain.FightRevenge{}, domain.ErrFightDailyLimit
		}
	}
	var pairFights int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM yard_fights
		WHERE yard_id = $1 AND created_at >= $2
		  AND ((cat_a_id = $3 AND cat_b_id = $4) OR (cat_a_id = $4 AND cat_b_id = $3))
	`, revenge.YardID, limits.PairSince, revenge.LoserCatID, revenge.WinnerCatID).Scan(&pairFights); err != nil {
		return domain.FightRevenge{}, err
	}
	if pairFights >= limits.PairLimit {
		return domain.FightRevenge{}, domain.ErrFightPairCooldown
	}
	return revenge, nil
}

func (r *FightRepo) SaveFight(ctx context.Context, record domain.FightRecord, result domain.FightResult, catAName, catBName string, limits domain.FightLimits) (domain.FightSaveResult, error) {
	turns, err := json.Marshal(result.Turns)
	if err != nil {
		return domain.FightSaveResult{}, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.FightSaveResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// ToggleQueue deliberately releases its yard lock before the engine runs.
	// Lock both cats in stable order and re-check here so concurrent matches,
	// including matches in different yards, cannot both persist after observing
	// the same pre-fight daily counters. Negative keys do not overlap yard locks.
	lockedCatAID, lockedCatBID := record.CatAID, record.CatBID
	if lockedCatAID > lockedCatBID {
		lockedCatAID, lockedCatBID = lockedCatBID, lockedCatAID
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, -lockedCatAID); err != nil {
		return domain.FightSaveResult{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, -lockedCatBID); err != nil {
		return domain.FightSaveResult{}, err
	}
	for _, id := range []int64{lockedCatAID, lockedCatBID} {
		var version int64
		var level int
		if err = tx.QueryRow(ctx, `SELECT state_version,level FROM cats WHERE id=$1 FOR UPDATE`, id).Scan(&version, &level); err != nil {
			return domain.FightSaveResult{}, err
		}
		expected := record.CatAVersion
		if id == record.CatBID {
			expected = record.CatBVersion
		}
		if version != expected {
			return domain.FightSaveResult{}, domain.ErrConcurrentUpdate
		}
		if level < 3 {
			return domain.FightSaveResult{}, domain.ErrFeatureLocked
		}
	}
	var catAFights, catBFights int
	if err := tx.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE cat_a_id = $2 OR cat_b_id = $2),
			count(*) FILTER (WHERE cat_a_id = $3 OR cat_b_id = $3)
		FROM yard_fights
		WHERE created_at >= $1
	`, limits.DailySince, record.CatAID, record.CatBID).Scan(&catAFights, &catBFights); err != nil {
		return domain.FightSaveResult{}, err
	}
	if catAFights >= limits.DailyLimit || catBFights >= limits.DailyLimit {
		return domain.FightSaveResult{}, domain.ErrFightDailyLimit
	}

	winnerName, loserName := catAName, catBName
	if result.WinnerCatID == record.CatBID {
		winnerName, loserName = catBName, catAName
	}
	if record.Kind == "" {
		record.Kind = domain.FightKindRegular
	}
	var parentFightID any
	if record.ParentFightID > 0 {
		parentFightID = record.ParentFightID
	}
	var fightID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO yard_fights (
			yard_id, cat_a_id, cat_b_id, cat_a_name, cat_b_name,
			winner_cat_id, loser_cat_id, winner_name, loser_name, seed,
			rounds, final_hp_a, final_hp_b, turns, rivalry_delta,
			fight_kind, parent_fight_id, cat_a_xp_gain, cat_b_xp_gain,
			rules_version, content_version, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22
		) RETURNING id
	`, record.YardID, record.CatAID, record.CatBID, catAName, catBName,
		result.WinnerCatID, result.LoserCatID, winnerName, loserName, record.Seed,
		result.Rounds, result.FinalHPA, result.FinalHPB, turns, record.RivalryDelta,
		string(record.Kind), parentFightID, record.CatAXPGain, record.CatBXPGain,
		record.RulesVersion, record.ContentVersion, record.CreatedAt).Scan(&fightID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "yard_fights_parent_unique" {
			return domain.FightSaveResult{}, domain.ErrFightRevengeExpired
		}
		return domain.FightSaveResult{}, err
	}
	tierA, tierB := "loss", "win"
	if record.WinnerCatID == record.CatAID {
		tierA, tierB = "win", "loss"
	}
	rewardA, err := awardCatXP(ctx, tx, r.engine, record.CatAID, record.CatAXPGain, "arena", uint64(record.Seed), tierA, false)
	if err != nil {
		return domain.FightSaveResult{}, err
	}
	rewardB, err := awardCatXP(ctx, tx, r.engine, record.CatBID, record.CatBXPGain, "arena", uint64(record.Seed), tierB, false)
	if err != nil {
		return domain.FightSaveResult{}, err
	}

	catAID, catBID := record.CatAID, record.CatBID
	if catAID > catBID {
		catAID, catBID = catBID, catAID
	}
	var friendship, rivalry, respect int
	err = tx.QueryRow(ctx, `
		INSERT INTO cat_relationships (cat_a_id, cat_b_id, rivalry, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (cat_a_id, cat_b_id) DO UPDATE
		SET rivalry = cat_relationships.rivalry + EXCLUDED.rivalry,
		    updated_at = EXCLUDED.updated_at
		RETURNING friendship, rivalry, respect
	`, catAID, catBID, record.RivalryDelta, record.CreatedAt).Scan(&friendship, &rivalry, &respect)
	if err != nil {
		return domain.FightSaveResult{}, err
	}
	var stats domain.FightStats
	if err := tx.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE winner_cat_id = $2),
			count(*) FILTER (WHERE loser_cat_id = $2),
			count(*) FILTER (WHERE winner_cat_id = $3),
			count(*) FILTER (WHERE loser_cat_id = $3)
		FROM yard_fights
		WHERE yard_id = $1 AND (cat_a_id IN ($2, $3) OR cat_b_id IN ($2, $3))
	`, record.YardID, record.CatAID, record.CatBID).Scan(
		&stats.CatAWins, &stats.CatALosses, &stats.CatBWins, &stats.CatBLosses,
	); err != nil {
		return domain.FightSaveResult{}, err
	}
	if err := tx.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE winner_cat_id = $2),
			count(*) FILTER (WHERE winner_cat_id = $3)
		FROM yard_fights
		WHERE yard_id = $1
		  AND ((cat_a_id = $2 AND cat_b_id = $3) OR (cat_a_id = $3 AND cat_b_id = $2))
	`, record.YardID, record.CatAID, record.CatBID).Scan(
		&stats.PairCatAWins, &stats.PairCatBWins,
	); err != nil {
		return domain.FightSaveResult{}, err
	}
	rows, err := tx.Query(ctx, `
		SELECT winner_cat_id
		FROM yard_fights
		WHERE yard_id = $1
		  AND ((cat_a_id = $2 AND cat_b_id = $3) OR (cat_a_id = $3 AND cat_b_id = $2))
		ORDER BY created_at DESC, id DESC
		LIMIT 20
	`, record.YardID, record.CatAID, record.CatBID)
	if err != nil {
		return domain.FightSaveResult{}, err
	}
	for rows.Next() {
		var winnerCatID int64
		if err := rows.Scan(&winnerCatID); err != nil {
			rows.Close()
			return domain.FightSaveResult{}, err
		}
		if winnerCatID != result.WinnerCatID {
			break
		}
		stats.PairWinnerStreak++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return domain.FightSaveResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.FightSaveResult{}, err
	}
	return domain.FightSaveResult{
		FightID: fightID, Friendship: friendship, Rivalry: rivalry, Respect: respect, Stats: stats, CatAFacts: rewardA.ProgressionFacts, CatBFacts: rewardB.ProgressionFacts,
	}, nil
}
