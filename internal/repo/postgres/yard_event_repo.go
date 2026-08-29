package postgres

import (
	"context"
	"errors"
	"time"

	"catforge/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type YardEventRepo struct{ pool *pgxpool.Pool }

func NewYardEventRepo(pool *pgxpool.Pool) *YardEventRepo { return &YardEventRepo{pool: pool} }

func (r *YardEventRepo) StartOrGet(ctx context.Context, yardID int64, eventType domain.YardEventType, seed int64, startsAt, resolvesAt time.Time, contentVersion uint32) (*domain.YardEvent, bool, error) {
	event := &domain.YardEvent{}
	created := true
	err := scanYardEvent(r.pool.QueryRow(ctx, `
		INSERT INTO yard_events (yard_id, event_type, state, seed, starts_at, resolves_at, content_version)
		VALUES ($1, $2, 'active', $3, $4, $5, $6)
		ON CONFLICT (yard_id) WHERE state = 'active' DO NOTHING
		RETURNING id, yard_id, event_type, state, seed, starts_at, resolves_at, content_version, created_at
	`, yardID, string(eventType), seed, startsAt, resolvesAt, contentVersion), event)
	if errors.Is(err, pgx.ErrNoRows) {
		created = false
		err = scanYardEvent(r.pool.QueryRow(ctx, `
			SELECT id, yard_id, event_type, state, seed, starts_at, resolves_at, content_version, created_at
			FROM yard_events WHERE yard_id = $1 AND state = 'active'
		`, yardID), event)
	}
	if err != nil {
		return nil, false, err
	}
	return event, created, nil
}

func (r *YardEventRepo) SubmitChoice(ctx context.Context, telegramChatID, eventID, userID int64, choice domain.YardEventChoiceID, now time.Time) (domain.YardEventChoice, bool, error) {
	var saved domain.YardEventChoice
	var storedChoice string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO yard_event_choices (event_id, cat_id, choice_id, submitted_at)
		SELECT e.id, ym.cat_id, $4, $5
		FROM yard_events e
		JOIN yards y ON y.id = e.yard_id
		JOIN yard_members ym ON ym.yard_id = y.id AND ym.user_id = $3
		WHERE y.telegram_chat_id = $1 AND e.id = $2 AND e.state = 'active' AND e.resolves_at > $5
		ON CONFLICT (event_id, cat_id) DO NOTHING
		RETURNING event_id, cat_id, choice_id, submitted_at
	`, telegramChatID, eventID, userID, string(choice), now).Scan(
		&saved.EventID, &saved.CatID, &storedChoice, &saved.SubmittedAt,
	)
	first := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		err = r.pool.QueryRow(ctx, `
			UPDATE yard_event_choices c
			SET choice_id = $4, submitted_at = $5
			FROM yard_events e
			JOIN yards y ON y.id = e.yard_id
			JOIN yard_members ym ON ym.yard_id = y.id AND ym.user_id = $3
			WHERE c.event_id = e.id AND c.cat_id = ym.cat_id
			  AND y.telegram_chat_id = $1 AND e.id = $2
			  AND e.state = 'active' AND e.resolves_at > $5
			RETURNING c.event_id, c.cat_id, c.choice_id, c.submitted_at
		`, telegramChatID, eventID, userID, string(choice), now).Scan(
			&saved.EventID, &saved.CatID, &storedChoice, &saved.SubmittedAt,
		)
		first = false
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.YardEventChoice{}, false, domain.ErrYardEventUnavailable
	}
	if err != nil {
		return domain.YardEventChoice{}, false, err
	}
	saved.ChoiceID = domain.YardEventChoiceID(storedChoice)
	return saved, first, nil
}

func (r *YardEventRepo) ChoiceCounts(ctx context.Context, eventID int64) (map[domain.YardEventChoiceID]int, error) {
	counts := map[domain.YardEventChoiceID]int{
		domain.YardChoiceSteal: 0, domain.YardChoiceDistract: 0, domain.YardChoiceScout: 0,
	}
	rows, err := r.pool.Query(ctx, `
		SELECT choice_id, count(*) FROM yard_event_choices
		WHERE event_id = $1 GROUP BY choice_id
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var choice string
		var count int
		if err := rows.Scan(&choice, &count); err != nil {
			return nil, err
		}
		counts[domain.YardEventChoiceID(choice)] = count
	}
	return counts, rows.Err()
}

func (r *YardEventRepo) DueEventIDs(ctx context.Context, now time.Time, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id FROM yard_events
		WHERE state = 'active' AND resolves_at <= $1
		ORDER BY resolves_at, id LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *YardEventRepo) GetResolutionInput(ctx context.Context, eventID int64) (domain.YardEventResolutionInput, error) {
	var input domain.YardEventResolutionInput
	err := scanYardEvent(r.pool.QueryRow(ctx, `
		SELECT id, yard_id, event_type, state, seed, starts_at, resolves_at, content_version, created_at
		FROM yard_events WHERE id = $1 AND state = 'active'
	`, eventID), &input.Event)
	if errors.Is(err, pgx.ErrNoRows) {
		return input, domain.ErrYardEventUnavailable
	}
	if err != nil {
		return input, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT telegram_chat_id FROM yards WHERE id = $1`, input.Event.YardID).Scan(&input.TelegramChatID); err != nil {
		return input, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.name, c.breed, cp.trait, cp.speech_style, cp.auto_speak_enabled,
		       yec.choice_id, c.level, c.hp_base, c.atk_base, c.def_base, c.spd_base
		FROM yard_event_choices yec
		JOIN cats c ON c.id = yec.cat_id
		JOIN cat_personality cp ON cp.cat_id = c.id
		WHERE yec.event_id = $1 ORDER BY c.id
	`, eventID)
	if err != nil {
		return input, err
	}
	defer rows.Close()
	for rows.Next() {
		var participant domain.YardEventParticipant
		var choice, breed, trait string
		if err := rows.Scan(
			&participant.CatID, &participant.CatName, &breed, &trait, &participant.SpeechStyle, &participant.AutoSpeakEnabled,
			&choice, &participant.Level, &participant.HP,
			&participant.ATK, &participant.DEF, &participant.SPD); err != nil {
			return input, err
		}
		participant.Breed = domain.Breed(breed)
		participant.Trait = domain.Trait(trait)
		participant.Choice = domain.YardEventChoiceID(choice)
		input.Participants = append(input.Participants, participant)
	}
	return input, rows.Err()
}

func (r *YardEventRepo) SaveResolution(ctx context.Context, eventID int64, result domain.YardEventResult, rulesVersion, contentVersion uint32, resolvedAt time.Time) (bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `
		UPDATE yard_events SET state = 'resolved'
		WHERE id = $1 AND state = 'active'
	`, eventID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO yard_event_results (
			event_id, success, team_score, target_score, fish_total, secret_found,
			strategy_bonus, rules_version, content_version, resolved_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, eventID, result.Success, result.TeamScore, result.TargetScore, result.FishTotal,
		result.SecretFound, result.StrategyBonus, rulesVersion, contentVersion, resolvedAt)
	if err != nil {
		return false, err
	}
	for _, participant := range result.Participants {
		_, err = tx.Exec(ctx, `
			INSERT INTO yard_event_participant_results
			(event_id, cat_id, choice_id, contribution, fish_reward, mvp)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, eventID, participant.CatID, string(participant.Choice), participant.Contribution, participant.FishReward, participant.MVP)
		if err != nil {
			return false, err
		}
	}
	for _, effect := range result.RelationshipEffects {
		catAID, catBID := effect.CatAID, effect.CatBID
		if catAID > catBID {
			catAID, catBID = catBID, catAID
		}
		if catAID <= 0 || catAID == catBID || effect.FriendshipDelta < 0 || effect.RivalryDelta < 0 || effect.RespectDelta < 0 {
			return false, errors.New("invalid yard relationship effect")
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO yard_event_relationship_effects
			(event_id, cat_a_id, cat_b_id, friendship_delta, rivalry_delta, respect_delta)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, eventID, catAID, catBID, effect.FriendshipDelta, effect.RivalryDelta, effect.RespectDelta)
		if err != nil {
			return false, err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO cat_relationships
			(cat_a_id, cat_b_id, friendship, rivalry, respect, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (cat_a_id, cat_b_id) DO UPDATE SET
				friendship = cat_relationships.friendship + EXCLUDED.friendship,
				rivalry = cat_relationships.rivalry + EXCLUDED.rivalry,
				respect = cat_relationships.respect + EXCLUDED.respect,
				updated_at = EXCLUDED.updated_at
		`, catAID, catBID, effect.FriendshipDelta, effect.RivalryDelta, effect.RespectDelta, resolvedAt)
		if err != nil {
			return false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (r *YardEventRepo) WeeklySummary(ctx context.Context, yardID int64, periodStart, periodEnd time.Time) (domain.YardWeeklySummary, error) {
	summary := domain.YardWeeklySummary{
		YardID: yardID, PeriodStart: periodStart, PeriodEnd: periodEnd,
	}
	err := r.pool.QueryRow(ctx, `
		SELECT
			count(*),
			count(*) FILTER (WHERE r.success),
			COALESCE(sum(r.fish_total), 0),
			count(*) FILTER (WHERE r.secret_found)
		FROM yard_events e
		JOIN yard_event_results r ON r.event_id = e.id
		WHERE e.yard_id = $1 AND r.resolved_at >= $2 AND r.resolved_at < $3
	`, yardID, periodStart, periodEnd).Scan(
		&summary.EventsResolved, &summary.SuccessfulEvents, &summary.FishTotal, &summary.SecretsFound,
	)
	if err != nil {
		return domain.YardWeeklySummary{}, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			c.id,
			c.name,
			c.breed,
			c.trait,
			count(*),
			count(*) FILTER (WHERE pr.choice_id = 'steal'),
			count(*) FILTER (WHERE pr.choice_id = 'distract'),
			count(*) FILTER (WHERE pr.choice_id = 'scout'),
			COALESCE(sum(pr.contribution), 0),
			COALESCE(sum(pr.fish_reward), 0),
			count(*) FILTER (WHERE pr.mvp)
		FROM yard_event_participant_results pr
		JOIN yard_event_results r ON r.event_id = pr.event_id
		JOIN yard_events e ON e.id = r.event_id
		JOIN cats c ON c.id = pr.cat_id
		WHERE e.yard_id = $1 AND r.resolved_at >= $2 AND r.resolved_at < $3
		GROUP BY c.id, c.name, c.breed, c.trait
		ORDER BY c.id
	`, yardID, periodStart, periodEnd)
	if err != nil {
		return domain.YardWeeklySummary{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var cat domain.YardWeeklyCatStats
		var breed, trait string
		if err := rows.Scan(
			&cat.CatID, &cat.CatName, &breed, &trait, &cat.EventsParticipated,
			&cat.StealChoices, &cat.DistractChoices, &cat.ScoutChoices,
			&cat.Contribution, &cat.FishReward, &cat.MVPCount,
		); err != nil {
			return domain.YardWeeklySummary{}, err
		}
		cat.Breed = domain.Breed(breed)
		cat.Trait = domain.Trait(trait)
		summary.TotalChoices += cat.EventsParticipated
		summary.Cats = append(summary.Cats, cat)
	}
	if err := rows.Err(); err != nil {
		return domain.YardWeeklySummary{}, err
	}
	summary.UniqueParticipants = len(summary.Cats)
	return summary, nil
}

func scanYardEvent(row pgx.Row, event *domain.YardEvent) error {
	var eventType, state string
	err := row.Scan(
		&event.ID, &event.YardID, &eventType, &state, &event.Seed,
		&event.StartsAt, &event.ResolvesAt, &event.ContentVersion, &event.CreatedAt,
	)
	event.Type = domain.YardEventType(eventType)
	event.State = domain.YardEventState(state)
	return err
}
