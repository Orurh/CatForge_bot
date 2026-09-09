package postgres

import (
	"context"
	"errors"
	"time"

	"catforge/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type YardRepo struct{ pool *pgxpool.Pool }

func NewYardRepo(pool *pgxpool.Pool) *YardRepo { return &YardRepo{pool: pool} }

func (r *YardRepo) EnsureAndJoin(ctx context.Context, telegramChatID int64, name string, userID, catID int64, now time.Time) (*domain.Yard, bool, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, false, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	yard := &domain.Yard{}
	created := true
	err = scanYard(tx.QueryRow(ctx, `
		INSERT INTO yards (telegram_chat_id, name, created_at, updated_at)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (telegram_chat_id) DO NOTHING
		RETURNING id, telegram_chat_id, name, humor_mode, auto_messages_enabled,
		          max_auto_messages_day, cat_to_cat_banter, fights_enabled, quiet_until, created_at, updated_at
	`, telegramChatID, name, now), yard)
	if errors.Is(err, pgx.ErrNoRows) {
		created = false
		err = scanYard(tx.QueryRow(ctx, `
			UPDATE yards SET name = $2, updated_at = $3
			WHERE telegram_chat_id = $1
			RETURNING id, telegram_chat_id, name, humor_mode, auto_messages_enabled,
			          max_auto_messages_day, cat_to_cat_banter, fights_enabled, quiet_until, created_at, updated_at
		`, telegramChatID, name, now), yard)
	}
	if err != nil {
		return nil, false, false, err
	}

	joined := true
	var inserted int
	err = tx.QueryRow(ctx, `
		INSERT INTO yard_members (yard_id, user_id, cat_id, joined_at, last_active_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (yard_id, user_id) DO NOTHING
		RETURNING 1
	`, yard.ID, userID, catID, now).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		joined = false
		_, err = tx.Exec(ctx, `
			UPDATE yard_members SET last_active_at = $3
			WHERE yard_id = $1 AND user_id = $2
		`, yard.ID, userID, now)
	}
	if err != nil {
		return nil, false, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, false, err
	}
	return yard, created, joined, nil
}

func (r *YardRepo) GetByTelegramChatID(ctx context.Context, telegramChatID int64) (*domain.Yard, error) {
	yard := &domain.Yard{}
	err := scanYard(r.pool.QueryRow(ctx, `
		SELECT id, telegram_chat_id, name, humor_mode, auto_messages_enabled,
		       max_auto_messages_day, cat_to_cat_banter, fights_enabled, quiet_until, created_at, updated_at
		FROM yards WHERE telegram_chat_id = $1
	`, telegramChatID), yard)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNoYard
	}
	if err != nil {
		return nil, err
	}
	return yard, nil
}

func (r *YardRepo) GetByID(ctx context.Context, yardID int64) (*domain.Yard, error) {
	yard := &domain.Yard{}
	err := scanYard(r.pool.QueryRow(ctx, `
		SELECT id, telegram_chat_id, name, humor_mode, auto_messages_enabled,
		       max_auto_messages_day, cat_to_cat_banter, fights_enabled, quiet_until, created_at, updated_at
		FROM yards WHERE id = $1
	`, yardID), yard)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNoYard
	}
	return yard, err
}

func (r *YardRepo) SaveSettings(ctx context.Context, yardID int64, settings domain.YardSettings, now time.Time) (*domain.Yard, error) {
	yard := &domain.Yard{}
	var quietUntil any
	if !settings.QuietUntil.IsZero() {
		quietUntil = settings.QuietUntil
	}
	err := scanYard(r.pool.QueryRow(ctx, `
		UPDATE yards SET humor_mode = $2, auto_messages_enabled = $3,
		       max_auto_messages_day = $4, cat_to_cat_banter = $5,
		       fights_enabled = $6, quiet_until = $7, updated_at = $8
		WHERE id = $1
		RETURNING id, telegram_chat_id, name, humor_mode, auto_messages_enabled,
		          max_auto_messages_day, cat_to_cat_banter, fights_enabled, quiet_until, created_at, updated_at
	`, yardID, string(settings.HumorMode), settings.AutoMessagesEnabled,
		settings.MaxAutoMessagesDay, settings.CatToCatBanter, settings.FightsEnabled, quietUntil, now), yard)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNoYard
	}
	return yard, err
}

func (r *YardRepo) ClaimAutoMessageSlot(ctx context.Context, yardID int64, now time.Time, limit int, kind domain.AutoMessageKind) (bool, error) {
	if limit < 1 || limit > 2 {
		return false, nil
	}
	singleDelta, banterDelta := 0, 0
	switch kind {
	case domain.AutoMessageSingle:
		singleDelta = 1
	case domain.AutoMessageBanter:
		banterDelta = 1
	default:
		return false, nil
	}
	var used int
	err := r.pool.QueryRow(ctx, `
		INSERT INTO yard_auto_message_usage (
			yard_id, usage_day, used, single_used, banter_used, updated_at
		)
		SELECT id, $2::date, 1, $5, $6, $3 FROM yards
		WHERE id = $1 AND auto_messages_enabled
		  AND (quiet_until IS NULL OR quiet_until <= $3)
		ON CONFLICT (yard_id, usage_day) DO UPDATE
		SET used = yard_auto_message_usage.used + 1,
		    single_used = yard_auto_message_usage.single_used + $5,
		    banter_used = yard_auto_message_usage.banter_used + $6,
		    updated_at = EXCLUDED.updated_at
		WHERE yard_auto_message_usage.used < $4
		  AND yard_auto_message_usage.single_used + $5 <= 1
		  AND yard_auto_message_usage.banter_used + $6 <= 1
		RETURNING used
	`, yardID, now.UTC().Format("2006-01-02"), now, limit, singleDelta, banterDelta).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *YardRepo) ListIdleBanterCandidates(ctx context.Context, now, activeSince time.Time, limit int) ([]domain.Yard, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT y.id, y.telegram_chat_id, y.name, y.humor_mode, y.auto_messages_enabled,
		       y.max_auto_messages_day, y.cat_to_cat_banter, y.fights_enabled,
		       y.quiet_until, y.created_at, y.updated_at
		FROM yards y
		LEFT JOIN LATERAL (
			SELECT ge.id, ge.occurred_at
			FROM game_events ge
			WHERE ge.yard_id = y.id
			  AND (
				ge.event_type = 'cat_banter'
				OR (ge.event_type = 'fight_finished' AND ge.properties->'banter' IS NOT NULL AND ge.properties->'banter' <> 'null'::jsonb)
			  )
			ORDER BY ge.occurred_at DESC, ge.id DESC
			LIMIT 1
		) previous ON true
		WHERE y.auto_messages_enabled
		  AND y.cat_to_cat_banter
		  AND y.max_auto_messages_day > 0
		  AND (y.quiet_until IS NULL OR y.quiet_until <= $1)
		  AND (
			SELECT count(*)
			FROM yard_members member
			JOIN cat_personality personality ON personality.cat_id = member.cat_id
			WHERE member.yard_id = y.id
			  AND member.last_active_at >= $2
			  AND personality.auto_speak_enabled
		  ) >= 2
		  AND COALESCE(previous.occurred_at, y.created_at) +
			make_interval(mins => (1440 + mod(abs(COALESCE(previous.id, y.id * 1103515245 + 12345)), 1441))::int) <= $1
		  AND NOT EXISTS (
			SELECT 1 FROM yard_auto_message_usage usage
			WHERE usage.yard_id = y.id AND usage.usage_day = $3::date AND usage.banter_used >= 1
		  )
		ORDER BY COALESCE(previous.occurred_at, y.created_at), y.id
		LIMIT $4
	`, now, activeSince, now.UTC().Format("2006-01-02"), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	yards := make([]domain.Yard, 0)
	for rows.Next() {
		var yard domain.Yard
		if err := scanYard(rows, &yard); err != nil {
			return nil, err
		}
		yards = append(yards, yard)
	}
	return yards, rows.Err()
}

func (r *YardRepo) ListMembers(ctx context.Context, yardID int64) ([]domain.YardMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ym.yard_id, ym.user_id, ym.cat_id, c.name, c.breed, cp.trait, c.level,
		       ym.joined_at, ym.last_active_at
		FROM yard_members ym
		JOIN cats c ON c.id = ym.cat_id
		JOIN cat_personality cp ON cp.cat_id = c.id
		WHERE ym.yard_id = $1
		ORDER BY ym.joined_at, ym.cat_id
	`, yardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := make([]domain.YardMember, 0)
	for rows.Next() {
		var member domain.YardMember
		var breed, trait string
		if err := rows.Scan(
			&member.YardID, &member.UserID, &member.CatID, &member.CatName,
			&breed, &trait, &member.Level, &member.JoinedAt, &member.LastActiveAt,
		); err != nil {
			return nil, err
		}
		member.Breed = domain.Breed(breed)
		member.Trait = domain.Trait(trait)
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *YardRepo) ListRelationships(ctx context.Context, yardID int64) ([]domain.CatRelationship, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cr.cat_a_id, cr.cat_b_id, ca.name, cb.name,
		       cr.friendship, cr.rivalry, cr.respect, cr.updated_at
		FROM cat_relationships cr
		JOIN yard_members yma ON yma.yard_id = $1 AND yma.cat_id = cr.cat_a_id
		JOIN yard_members ymb ON ymb.yard_id = $1 AND ymb.cat_id = cr.cat_b_id
		JOIN cats ca ON ca.id = cr.cat_a_id
		JOIN cats cb ON cb.id = cr.cat_b_id
		ORDER BY (cr.friendship + cr.rivalry + cr.respect) DESC, cr.updated_at DESC
		LIMIT 20
	`, yardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	relationships := make([]domain.CatRelationship, 0)
	for rows.Next() {
		var relationship domain.CatRelationship
		if err := rows.Scan(
			&relationship.CatAID, &relationship.CatBID, &relationship.CatAName, &relationship.CatBName,
			&relationship.Friendship, &relationship.Rivalry, &relationship.Respect, &relationship.UpdatedAt,
		); err != nil {
			return nil, err
		}
		relationships = append(relationships, relationship)
	}
	return relationships, rows.Err()
}

func scanYard(row pgx.Row, yard *domain.Yard) error {
	var humorMode string
	var quietUntil *time.Time
	err := row.Scan(
		&yard.ID, &yard.TelegramChatID, &yard.Name, &humorMode,
		&yard.AutoMessagesEnabled, &yard.MaxAutoMessagesDay, &yard.CatToCatBanter,
		&yard.FightsEnabled, &quietUntil, &yard.CreatedAt, &yard.UpdatedAt,
	)
	yard.HumorMode = domain.HumorMode(humorMode)
	if quietUntil != nil {
		yard.QuietUntil = *quietUntil
	}
	return err
}
