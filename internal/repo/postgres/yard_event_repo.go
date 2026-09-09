package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"catforge/internal/domain"
	"catforge/internal/gameengine"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type YardEventRepo struct {
	pool   *pgxpool.Pool
	engine gameengine.ProgressionEngine
}

func NewYardEventRepo(pool *pgxpool.Pool, engines ...gameengine.ProgressionEngine) *YardEventRepo {
	r := &YardEventRepo{pool: pool}
	if len(engines) > 0 {
		r.engine = engines[0]
	}
	return r
}

func (r *YardEventRepo) NextEventType(ctx context.Context, yardID int64) (domain.YardEventType, error) {
	var previous string
	err := r.pool.QueryRow(ctx, `
		SELECT event_type FROM yard_events
		WHERE yard_id = $1
		ORDER BY starts_at DESC, id DESC
		LIMIT 1
	`, yardID).Scan(&previous)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.YardEventFishTruck, nil
	}
	if err != nil {
		return "", err
	}
	return domain.NextYardEventType(domain.YardEventType(previous)), nil
}

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

func (r *YardEventRepo) GetCurrentActive(ctx context.Context, telegramChatID int64) (*domain.YardEvent, error) {
	event := &domain.YardEvent{}
	err := scanYardEvent(r.pool.QueryRow(ctx, `
		SELECT e.id, e.yard_id, e.event_type, e.state, e.seed, e.starts_at, e.resolves_at, e.content_version, e.created_at
		FROM yard_events e
		JOIN yards y ON y.id = e.yard_id
		WHERE y.telegram_chat_id = $1 AND e.state = 'active'
	`, telegramChatID), event)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrYardEventUnavailable
	}
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (r *YardEventRepo) GetActive(ctx context.Context, telegramChatID, eventID int64) (*domain.YardEvent, error) {
	event := &domain.YardEvent{}
	err := scanYardEvent(r.pool.QueryRow(ctx, `
		SELECT e.id, e.yard_id, e.event_type, e.state, e.seed, e.starts_at, e.resolves_at, e.content_version, e.created_at
		FROM yard_events e
		JOIN yards y ON y.id = e.yard_id
		WHERE y.telegram_chat_id = $1 AND e.id = $2 AND e.state = 'active'
	`, telegramChatID, eventID), event)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrYardEventUnavailable
	}
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (r *YardEventRepo) ListStartCandidates(ctx context.Context, now, activeSince time.Time, limit int) ([]domain.Yard, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT y.id, y.telegram_chat_id, y.name, y.humor_mode, y.auto_messages_enabled,
		       y.max_auto_messages_day, y.cat_to_cat_banter, y.fights_enabled, y.quiet_until, y.created_at, y.updated_at
		FROM yards y
		LEFT JOIN LATERAL (
			SELECT e.seed, e.resolves_at
			FROM yard_events e
			WHERE e.yard_id = y.id
			ORDER BY e.starts_at DESC, e.id DESC
			LIMIT 1
		) previous ON true
		WHERE NOT EXISTS (
			SELECT 1 FROM yard_events active
			WHERE active.yard_id = y.id AND active.state = 'active'
		)
		AND (
			SELECT count(*) FROM yard_members member
			WHERE member.yard_id = y.id AND member.last_active_at >= $2
		) >= 2
		AND COALESCE(previous.resolves_at, y.created_at) +
			make_interval(mins => (1440 + mod(abs(COALESCE(previous.seed, y.id * 1103515245 + 12345)), 1441))::int) <= $1
		ORDER BY COALESCE(previous.resolves_at, y.created_at), y.id
		LIMIT $3
	`, now, activeSince, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	yards := make([]domain.Yard, 0)
	for rows.Next() {
		var yard domain.Yard
		var humorMode string
		var quietUntil *time.Time
		if err := rows.Scan(
			&yard.ID, &yard.TelegramChatID, &yard.Name, &humorMode,
			&yard.AutoMessagesEnabled, &yard.MaxAutoMessagesDay, &yard.CatToCatBanter,
			&yard.FightsEnabled, &quietUntil, &yard.CreatedAt, &yard.UpdatedAt,
		); err != nil {
			return nil, err
		}
		yard.HumorMode = domain.HumorMode(humorMode)
		if quietUntil != nil {
			yard.QuietUntil = *quietUntil
		}
		yards = append(yards, yard)
	}
	return yards, rows.Err()
}

func (r *YardEventRepo) SubmitChoice(ctx context.Context, telegramChatID, eventID, userID int64, choice domain.YardEventChoiceID, now time.Time) (domain.YardEventChoice, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.YardEventChoice{}, false, err
	}
	defer tx.Rollback(ctx)
	var catID int64
	var level int
	var eventType string
	var stats domain.FelineStats
	err = tx.QueryRow(ctx, `SELECT c.id,c.level,c.claws_tenth_mm,c.weight_grams,c.tail_mm,c.whisker_span_mm,e.event_type
 FROM yard_events e JOIN yards y ON y.id=e.yard_id JOIN yard_members ym ON ym.yard_id=y.id AND ym.user_id=$3 JOIN cats c ON c.id=ym.cat_id
 WHERE y.telegram_chat_id=$1 AND e.id=$2 AND e.state='active' AND e.resolves_at>$4 FOR UPDATE OF e,c`, telegramChatID, eventID, userID, now).Scan(&catID, &level, &stats.ClawsTenthMM, &stats.WeightGrams, &stats.TailMM, &stats.WhiskerSpanMM, &eventType)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.YardEventChoice{}, false, domain.ErrYardEventUnavailable
	}
	if err != nil {
		return domain.YardEventChoice{}, false, err
	}
	bonus, effects, err := physicalEquipment(ctx, tx, userID, level)
	if err != nil {
		return domain.YardEventChoice{}, false, err
	}
	stats.Add(bonus)
	special := ""
	if action, ok := domain.SpecialActionByID(string(choice)); ok {
		if string(action.Event) != eventType {
			return domain.YardEventChoice{}, false, domain.ErrInvalidYardChoice
		}
		if !action.Available(stats, effects) {
			return domain.YardEventChoice{}, false, fmt.Errorf("%w: нужно %s", domain.ErrFeatureLocked, action.Requirement)
		}
		special = action.ID
		choice = action.Choice
	} else if !domain.IsValidYardEventChoice(choice) {
		return domain.YardEventChoice{}, false, domain.ErrInvalidYardChoice
	}
	snapshot, _ := json.Marshal(struct {
		Feline  domain.FelineStats
		Effects []string
	}{stats, effects})
	var first bool
	err = tx.QueryRow(ctx, `INSERT INTO yard_event_choices(event_id,cat_id,choice_id,submitted_at,special_action,capability_snapshot) VALUES($1,$2,$3,$4,$5,$6)
 ON CONFLICT(event_id,cat_id) DO UPDATE SET choice_id=EXCLUDED.choice_id,submitted_at=EXCLUDED.submitted_at,special_action=EXCLUDED.special_action,capability_snapshot=EXCLUDED.capability_snapshot RETURNING (xmax=0)`, eventID, catID, string(choice), now, special, snapshot).Scan(&first)
	if err != nil {
		return domain.YardEventChoice{}, false, err
	}
	return domain.YardEventChoice{EventID: eventID, CatID: catID, ChoiceID: choice, SubmittedAt: now}, first, tx.Commit(ctx)
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
		SELECT c.id, c.name, c.breed, cp.trait, cp.speech_style, cp.humor_mode, cp.auto_speak_enabled,
		       yec.choice_id, c.level, c.hp_base, c.atk_base, c.def_base, c.spd_base, c.claws_tenth_mm,c.weight_grams,c.tail_mm,c.whisker_span_mm,yec.special_action,yec.capability_snapshot
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
		var choice, breed, trait, humor string
		var snapshot []byte
		if err := rows.Scan(
			&participant.CatID, &participant.CatName, &breed, &trait, &participant.SpeechStyle, &humor, &participant.AutoSpeakEnabled,
			&choice, &participant.Level, &participant.HP,
			&participant.ATK, &participant.DEF, &participant.SPD, &participant.Feline.ClawsTenthMM, &participant.Feline.WeightGrams, &participant.Feline.TailMM, &participant.Feline.WhiskerSpanMM, &participant.SpecialAction, &snapshot); err != nil {
			return input, err
		}
		participant.Breed = domain.Breed(breed)
		participant.Trait = domain.Trait(trait)
		participant.HumorMode = domain.HumorMode(humor)
		participant.Choice = domain.YardEventChoiceID(choice)
		var saved struct {
			Feline  domain.FelineStats
			Effects []string
		}
		if err = json.Unmarshal(snapshot, &saved); err != nil {
			return input, err
		}
		if saved.Feline.Valid() {
			participant.Feline = saved.Feline
			participant.Effects = saved.Effects
		}
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
			event_id, success, outcome_tier, team_score, target_score, yard_score,
			participant_xp, secret_found, strategy_bonus, rules_version,
			content_version, resolved_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, eventID, result.OutcomeTier.IsSuccess(), string(result.OutcomeTier), result.TeamScore,
		result.TargetScore, result.YardScore, result.XPGain, result.SecretFound,
		result.StrategyBonus, rulesVersion, contentVersion, resolvedAt)
	if err != nil {
		return false, err
	}
	for index, participant := range result.Participants {
		_, err = tx.Exec(ctx, `
			INSERT INTO yard_event_participant_results
			(event_id, cat_id, choice_id, contribution, mvp)
			VALUES ($1,$2,$3,$4,$5)
		`, eventID, participant.CatID, string(participant.Choice), participant.Contribution, participant.MVP)
		if err != nil {
			return false, err
		}
		reward, err := awardCatXP(ctx, tx, r.engine, participant.CatID, result.XPGain, "yard", uint64(eventID)^uint64(participant.CatID)<<32, string(result.OutcomeTier), result.SecretFound)
		if err != nil {
			return false, err
		}
		var previousFailure bool
		if result.OutcomeTier.IsSuccess() {
			err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT r.outcome_tier IN ('fail','partial') FROM yard_events old JOIN yard_event_results r ON r.event_id=old.id JOIN yard_event_participant_results p ON p.event_id=old.id JOIN yard_events current ON current.id=$1 WHERE old.yard_id=current.yard_id AND old.event_type=current.event_type AND old.id<>current.id AND p.cat_id=$2 ORDER BY r.resolved_at DESC,old.id DESC LIMIT 1),false)`, eventID, participant.CatID).Scan(&previousFailure)
			if err != nil {
				return false, err
			}
		}
		var special string
		if err = tx.QueryRow(ctx, `SELECT special_action FROM yard_event_choices WHERE event_id=$1 AND cat_id=$2`, eventID, participant.CatID).Scan(&special); err != nil {
			return false, err
		}
		if previousFailure {
			reward.ProgressionFacts = append(reward.ProgressionFacts, "previously_failed_event_now_won")
		}
		if participant.ItemEffectTriggered {
			fact := "item_effect_triggered"
			if special != "" {
				fact += "=" + special
			}
			reward.ProgressionFacts = append(reward.ProgressionFacts, fact)
		}
		if len(reward.ProgressionFacts) > 0 {
			data, _ := json.Marshal(reward.ProgressionFacts)
			if _, err = tx.Exec(ctx, `INSERT INTO cat_progression_facts(cat_id,state_version,facts) VALUES($1,$2,$3) ON CONFLICT(cat_id,state_version) DO UPDATE SET facts=EXCLUDED.facts`, reward.ID, reward.StateVersion, data); err != nil {
				return false, err
			}
		}
		result.Participants[index].LootItemID = reward.LootItemID
		result.Participants[index].ProgressionFacts = reward.ProgressionFacts
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
			count(*) FILTER (WHERE r.outcome_tier = 'fail'),
			count(*) FILTER (WHERE r.outcome_tier = 'partial'),
			count(*) FILTER (WHERE r.outcome_tier = 'success'),
			count(*) FILTER (WHERE r.outcome_tier = 'exceptional'),
			COALESCE(sum(r.yard_score), 0),
			count(*) FILTER (WHERE r.secret_found)
		FROM yard_events e
		JOIN yard_event_results r ON r.event_id = e.id
		WHERE e.yard_id = $1 AND r.resolved_at >= $2 AND r.resolved_at < $3
	`, yardID, periodStart, periodEnd).Scan(
		&summary.EventsResolved, &summary.FailedEvents, &summary.PartialEvents,
		&summary.SuccessfulEvents, &summary.ExceptionalEvents, &summary.YardScore,
		&summary.SecretsFound,
	)
	if err != nil {
		return domain.YardWeeklySummary{}, err
	}

	rows, err := r.pool.Query(ctx, `
		WITH event_stats AS (
			SELECT
				pr.cat_id,
				count(*) AS events_participated,
				count(*) FILTER (WHERE pr.choice_id = 'steal') AS steal_choices,
				count(*) FILTER (WHERE pr.choice_id = 'distract') AS distract_choices,
				count(*) FILTER (WHERE pr.choice_id = 'scout') AS scout_choices,
				COALESCE(sum(pr.contribution), 0) AS contribution,
				count(*) FILTER (WHERE pr.mvp) AS mvp_count,
				COALESCE(sum(r.participant_xp), 0) AS event_xp
			FROM yard_event_participant_results pr
			JOIN yard_event_results r ON r.event_id = pr.event_id
			JOIN yard_events e ON e.id = r.event_id
			WHERE e.yard_id = $1 AND r.resolved_at >= $2 AND r.resolved_at < $3
			GROUP BY pr.cat_id
		), training_stats AS (
			SELECT
				ge.cat_id,
				COALESCE(sum(COALESCE(
					NULLIF(ge.properties->'result'->>'energy_cost', '')::integer,
					NULLIF(ge.properties->'result'->>'EnergyCost', '')::integer,
					0
				)), 0) AS energy_spent,
				COALESCE(sum(COALESCE(
					NULLIF(ge.properties->'result'->>'xp_gain', '')::bigint,
					NULLIF(ge.properties->'result'->>'XPGain', '')::bigint,
					0
				)), 0) AS training_xp
			FROM game_events ge
			WHERE ge.event_type = 'cat_trained'
			  AND ge.occurred_at >= $2 AND ge.occurred_at < $3
			  AND ge.cat_id IS NOT NULL
			GROUP BY ge.cat_id
		), fight_entries AS (
			SELECT cat_a_id AS cat_id, winner_cat_id = cat_a_id AS won,
			       rivalry_delta, cat_a_xp_gain AS xp_gain
			FROM yard_fights
			WHERE yard_id = $1 AND created_at >= $2 AND created_at < $3
			UNION ALL
			SELECT cat_b_id AS cat_id, winner_cat_id = cat_b_id AS won,
			       rivalry_delta, cat_b_xp_gain AS xp_gain
			FROM yard_fights
			WHERE yard_id = $1 AND created_at >= $2 AND created_at < $3
		), fight_stats AS (
			SELECT cat_id, count(*) AS fights,
			       count(*) FILTER (WHERE won) AS wins,
			       count(*) FILTER (WHERE NOT won) AS losses,
			       COALESCE(sum(rivalry_delta), 0) AS rivalry_gained
			FROM fight_entries
			WHERE cat_id IS NOT NULL
			GROUP BY cat_id
		)
		SELECT
			c.id,
			c.name,
			c.breed,
			c.trait,
			COALESCE(es.events_participated, 0),
			COALESCE(es.steal_choices, 0),
			COALESCE(es.distract_choices, 0),
			COALESCE(es.scout_choices, 0),
			COALESCE(es.contribution, 0),
			COALESCE(es.mvp_count, 0),
			COALESCE(es.event_xp, 0),
			COALESCE(ts.energy_spent, 0),
			COALESCE(ts.training_xp, 0),
			COALESCE(fs.fights, 0),
			COALESCE(fs.wins, 0),
			COALESCE(fs.losses, 0),
			COALESCE(fs.rivalry_gained, 0),
			(SELECT count(*) FROM yard_events ae JOIN yard_event_results ar ON ar.event_id = ae.id
			 WHERE ae.yard_id = $1 AND ar.resolved_at >= $2 AND ar.resolved_at < $3
			 AND ae.resolves_at > ym.joined_at)
		FROM yard_members ym
		JOIN cats c ON c.id = ym.cat_id
		LEFT JOIN event_stats es ON es.cat_id = c.id
		LEFT JOIN training_stats ts ON ts.cat_id = c.id
		LEFT JOIN fight_stats fs ON fs.cat_id = c.id
		WHERE ym.yard_id = $1 AND ym.joined_at < $3
		  AND (COALESCE(es.events_participated, 0) > 0
		    OR COALESCE(ts.energy_spent, 0) > 0
		    OR COALESCE(fs.fights, 0) > 0)
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
			&cat.Contribution, &cat.MVPCount, &cat.EventXP,
			&cat.TrainingEnergySpent, &cat.TrainingXP,
			&cat.Fights, &cat.Wins, &cat.Losses, &cat.RivalryGained, &cat.AvailableEvents,
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
	rows.Close()
	summary.UniqueParticipants = len(summary.Cats)
	streakRows, err := r.pool.Query(ctx, `
		SELECT participant.cat_id, participant.won
		FROM yard_fights f
		CROSS JOIN LATERAL (VALUES
			(f.cat_a_id, f.winner_cat_id = f.cat_a_id),
			(f.cat_b_id, f.winner_cat_id = f.cat_b_id)
		) AS participant(cat_id, won)
		WHERE f.yard_id = $1 AND f.created_at >= $2 AND f.created_at < $3
		  AND participant.cat_id IS NOT NULL
		ORDER BY participant.cat_id, f.created_at DESC, f.id DESC
	`, yardID, periodStart, periodEnd)
	if err != nil {
		return domain.YardWeeklySummary{}, err
	}
	defer streakRows.Close()
	type streakState struct {
		initialized bool
		wins        bool
		stopped     bool
		count       int
	}
	streaks := make(map[int64]streakState)
	for streakRows.Next() {
		var catID int64
		var won bool
		if err := streakRows.Scan(&catID, &won); err != nil {
			return domain.YardWeeklySummary{}, err
		}
		state := streaks[catID]
		if !state.initialized {
			state.initialized, state.wins = true, won
		}
		if state.wins != won {
			state.stopped = true
		}
		if !state.stopped && won {
			state.count++
		}
		streaks[catID] = state
	}
	if err := streakRows.Err(); err != nil {
		return domain.YardWeeklySummary{}, err
	}
	for index := range summary.Cats {
		summary.Cats[index].WinStreak = streaks[summary.Cats[index].CatID].count
	}
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
