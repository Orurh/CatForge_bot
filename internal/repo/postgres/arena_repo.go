package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"catforge/internal/domain"
)

type ArenaRepo struct{ pool *pgxpool.Pool }

func NewArenaRepo(pool *pgxpool.Pool) *ArenaRepo { return &ArenaRepo{pool: pool} }

func (r *ArenaRepo) GetState(ctx context.Context, userID int64, now time.Time) (domain.ArenaState, error) {
	var st domain.ArenaState
	err := r.pool.QueryRow(ctx, `
	SELECT tickets, tickets_updated_at, rating, season_points, rage,
       COALESCE(reroll_day, CURRENT_DATE)                AS reroll_day,
       COALESCE(rerolls_today, 0)                        AS rerolls_today,
       COALESCE(reroll_ready_at, 'epoch'::timestamptz)   AS reroll_ready_at
		FROM arena_state
		WHERE user_id = $1
	`, userID).Scan(&st.Tickets, &st.TicketsUpdatedAt, &st.Rating, &st.SeasonPoints, &st.Rage,
		&st.RerollDay, &st.RerollsToday, &st.RerollReadyAt)

	if errors.Is(err, pgx.ErrNoRows) {
		st = domain.ArenaState{
			Tickets:          domain.ArenaTicketsCap,
			TicketsUpdatedAt: now.UTC(),
			Rating:           domain.ArenaBaseRating,
			Rage:             0,
			RerollDay:        domain.ArenaDay(now),
			RerollsToday:     0,
			RerollReadyAt:    time.Unix(0, 0).UTC(),
		}
		_ = r.SaveState(ctx, userID, st)
		return st, nil
	}
	return st, err
}

func (r *ArenaRepo) SaveState(ctx context.Context, userID int64, st domain.ArenaState) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO arena_state (user_id, tickets, tickets_updated_at, rating, season_points, rage, reroll_day, rerolls_today, reroll_ready_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (user_id) DO UPDATE
		SET tickets = EXCLUDED.tickets,
		    tickets_updated_at = EXCLUDED.tickets_updated_at,
		    rating = EXCLUDED.rating,
		    season_points = EXCLUDED.season_points,
		    rage = EXCLUDED.rage,
		    reroll_day = EXCLUDED.reroll_day,
		    rerolls_today = EXCLUDED.rerolls_today,
		    reroll_ready_at = EXCLUDED.reroll_ready_at
	`, userID, st.Tickets, st.TicketsUpdatedAt, st.Rating, st.SeasonPoints, st.Rage, st.RerollDay, st.RerollsToday, st.RerollReadyAt)
 	return err
}

func (r *ArenaRepo) ResetState(ctx context.Context, userID int64, now time.Time) error {
	st := domain.ArenaState{
		Tickets:          domain.ArenaTicketsCap,
		TicketsUpdatedAt: now.UTC(),
		Rating:           domain.ArenaBaseRating,
		SeasonPoints:     0,
		Rage:             0,
		RerollDay:        domain.ArenaDay(now),
		RerollsToday:     0,
		RerollReadyAt:    time.Unix(0, 0).UTC(),
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO arena_state (user_id, tickets, tickets_updated_at, rating, season_points, rage, reroll_day, rerolls_today, reroll_ready_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (user_id) DO UPDATE
		SET tickets = EXCLUDED.tickets,
		    tickets_updated_at = EXCLUDED.tickets_updated_at,
		    rating = EXCLUDED.rating,
		    season_points = EXCLUDED.season_points,
		    rage = EXCLUDED.rage,
		    reroll_day = EXCLUDED.reroll_day,
		    rerolls_today = EXCLUDED.rerolls_today,
		    reroll_ready_at = EXCLUDED.reroll_ready_at
	`, userID, st.Tickets, st.TicketsUpdatedAt, st.Rating, st.SeasonPoints, st.Rage, st.RerollDay, st.RerollsToday, st.RerollReadyAt)
	return err
}

func (r *ArenaRepo) FindOpponents(ctx context.Context, userID int64, attackerPower int, scopeChatID int64, scopeChatType string, seed string) ([]domain.ArenaOpponent, error) {
	seen := make(map[int64]struct{}, 4)
	add := func(op domain.ArenaOpponent) bool {
		if _, ok := seen[op.UserID]; ok {
			return false
		}
		seen[op.UserID] = struct{}{}
		return true
	}
	type win struct {
		kind domain.ArenaOpponentKind
		min  int
		max  int
	}
	windows := []win{
		{kind: domain.ArenaOppWeaker, min: attackerPower - 260, max: attackerPower - 60},
		{kind: domain.ArenaOppEven, min: attackerPower - 80, max: attackerPower + 80},
		{kind: domain.ArenaOppStronger, min: attackerPower + 60, max: attackerPower + 260},
	}

	out := make([]domain.ArenaOpponent, 0, 3)

	// 1) scoped windows
	for i, w := range windows {
		op, ok, err := r.findOneOpponent(ctx, userID, w.min, w.max, scopeChatID, scopeChatType, fmt.Sprintf("%s:%d", seed, i))
		if err != nil {
			return nil, err
		}
		if ok && add(op) {
			op.Kind = w.kind
			out = append(out, op)
		}
	}

	// 2) wide (scoped, then global fallback)
	if len(out) == 0 {
		op, ok, err := r.findOneOpponent(ctx, userID, attackerPower-99999, attackerPower+99999, scopeChatID, scopeChatType, seed+":wide")
		if err != nil {
			return nil, err
		}
		if !ok && scopeChatID != 0 {
			op, ok, err = r.findOneOpponent(ctx, userID, attackerPower-99999, attackerPower+99999, 0, "", seed+":wide:global")
			if err != nil {
				return nil, err
			}
		}
		if ok && add(op) {
			op.Kind = domain.ArenaOppEven
			out = append(out, op)
		}
	}

	// 3) fallback to global if we have too few targets in scoped pool
	if scopeChatID != 0 && len(out) < 3 {
		for i, w := range windows {
			op, ok, err := r.findOneOpponent(ctx, userID, w.min, w.max, 0, "", fmt.Sprintf("%s:global:%d", seed, i))
			if err != nil {
				return nil, err
			}
			if ok && add(op) {
				op.Kind = w.kind
				out = append(out, op)
				if len(out) >= 3 {
					break
				}
			}
		}
	}

	return out, nil
}


func (r *ArenaRepo) findOneOpponent(ctx context.Context, userID int64, minPower, maxPower int, scopeChatID int64, scopeChatType string, seed string) (domain.ArenaOpponent, bool, error) {
	const q = `
	WITH candidates AS (
		SELECT
			c.user_id,
			c.name,
			c.breed,
			c.level,
			((c.atk_base * 3) + (c.def_base * 2) + (c.hp_base / 2) + (c.spd_base * 1) + (c.level * 5)) AS power
		FROM cats c
		JOIN users u ON u.id = c.user_id
		WHERE c.user_id <> $1
		  AND ((c.atk_base * 3) + (c.def_base * 2) + (c.hp_base / 2) + (c.spd_base * 1) + (c.level * 5)) BETWEEN $2 AND $3
		  AND ($5::bigint = 0 OR (u.home_chat_id = $5 AND u.home_chat_type = $6))
	)
	SELECT user_id, name, breed, level, power
	FROM candidates
	ORDER BY md5((user_id::text) || $4)
	LIMIT 1
	`
	var op domain.ArenaOpponent
	var breed string
	err := r.pool.QueryRow(ctx, q, userID, minPower, maxPower, seed, scopeChatID, scopeChatType).Scan(
		&op.UserID, &op.Name, &breed, &op.Level, &op.Power,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ArenaOpponent{}, false, nil
	}
	if err != nil {
		return domain.ArenaOpponent{}, false, err
	}
	op.Breed = domain.Breed(breed)
	return op, true, nil
}

func (r *ArenaRepo) Fight(ctx context.Context, userID int64, opponentUserID int64, now time.Time, seed string) (domain.ArenaState, domain.ArenaFightResult, int64, int, int, int, int, int, error) {

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, 0, 0, 0, 0, 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1) state + regen + consume ticket
	st, err := r.getStateTx(ctx, tx, userID, now)
	if err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, 0, 0, 0, 0, 0, 0, err
	}
	st = domain.RegenArenaTickets(st, now)
	if st.Tickets <= 0 {
		return st, domain.ArenaFightResult{}, 0, 0, 0, 0, st.Rage, st.Rage, errors.New("arena: no tickets")

	}
	st.Tickets--
	rageBefore := st.Rage

	// 2) load powers (SQL = domain.Power)
	ap, err := catPowerTx(ctx, tx, userID)
	if err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, 0, 0, 0, 0, 0, 0, err
	}
	dp, err := catPowerTx(ctx, tx, opponentUserID)
	if err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, 0, 0, 0, 0, 0, 0, err
	}

	res := domain.ArenaResolveWithRage(seed, ap, dp, st.Rage)
	res.NewRating = st.Rating + res.RatingDelta
	if res.NewRating < 0 {
		res.NewRating = 0
	}
	st.Rating = res.NewRating
	diff := ap - dp
	riskMulPct := 100
	switch {
	case diff <= -80:
		riskMulPct = 130
	case diff >= 80:
		riskMulPct = 80
	default:
		riskMulPct = 100
	}

	// season points (scaled by risk)
	seasonBase := 3
	if res.AttackerWon {
		seasonBase = 10
	}
	seasonDelta := int(math.Round(float64(seasonBase) * float64(riskMulPct) / 100.0))
	if seasonDelta < 0 {
		seasonDelta = 0
	}
	st.SeasonPoints += seasonDelta

	// rage update
	if res.AttackerWon {
		st.Rage = 0
	} else {
		st.Rage++
		if st.Rage > domain.ArenaRageCap {
			st.Rage = domain.ArenaRageCap
		}
	}
	rageAfter := st.Rage

	// 2.5) reward XP to attacker (more for win)
	xpGain := int64(12)
	if res.AttackerWon {
		xpGain = 28
	}

	xpGain = int64(math.Round(float64(xpGain) * float64(riskMulPct) / 100.0))
	if xpGain < 1 {
		xpGain = 1
	}

	// Load attacker cat (FOR UPDATE) to apply XP + possible levelups.
	var c domain.Cat
	err = scanCatFull(tx.QueryRow(ctx, `
        SELECT id, user_id, name, breed, trait, level, xp, energy, last_train_at, energy_updated_at,
               hp_base, atk_base, def_base, spd_base
        FROM cats WHERE user_id=$1
        FOR UPDATE
    `, userID), &c)
	if err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, 0, 0, 0, 0, 0, 0, err
	}

	c.XP += xpGain
	leveled := 0
	var gained domain.StatDelta
	for c.XP >= int64(c.Level)*100 {
		c.XP -= int64(c.Level) * 100
		c.Level++
		leveled++
		d := domain.ApplyLevelUps(&c, 1)
		gained.Add(d)
	}

	// persist cat xp/level/stats
	_, err = tx.Exec(ctx, `
        UPDATE cats
        SET level=$2, xp=$3,
            hp_base=$4, atk_base=$5, def_base=$6, spd_base=$7,
            updated_at=now()
        WHERE user_id=$1
    `, userID, c.Level, c.XP, c.HPBase, c.ATKBase, c.DEFBase, c.SPDBase)
	if err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, 0, 0, 0, 0, 0, 0, err
	}

	// 3) persist state
	if err := r.saveStateTx(ctx, tx, userID, st); err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, 0, 0, 0, 0, 0, 0, err
	}

	// 4) match log
	_, err = tx.Exec(ctx, `
		INSERT INTO arena_matches(attacker_id, defender_id, seed, attacker_power, defender_power, attacker_won, rating_delta)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, userID, opponentUserID, seed, ap, dp, res.AttackerWon, res.RatingDelta)
	if err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, 0, 0, 0, 0, 0, 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, 0, 0, 0, 0, 0, 0, err
	}
	return st, res, xpGain, leveled, riskMulPct, seasonDelta, rageBefore, rageAfter, nil

}

func (r *ArenaRepo) getStateTx(ctx context.Context, tx pgx.Tx, userID int64, now time.Time) (domain.ArenaState, error) {
	var st domain.ArenaState
	err := tx.QueryRow(ctx, `
	SELECT tickets, tickets_updated_at, rating, season_points, rage,
       COALESCE(reroll_day, CURRENT_DATE)                AS reroll_day,
       COALESCE(rerolls_today, 0)                        AS rerolls_today,
       COALESCE(reroll_ready_at, 'epoch'::timestamptz)   AS reroll_ready_at
		FROM arena_state WHERE user_id = $1
		FOR UPDATE
		`, userID).Scan(&st.Tickets, &st.TicketsUpdatedAt, &st.Rating, &st.SeasonPoints, &st.Rage,
		&st.RerollDay, &st.RerollsToday, &st.RerollReadyAt)

	if errors.Is(err, pgx.ErrNoRows) {
		st = domain.ArenaState{
			Tickets:          domain.ArenaTicketsCap,
			TicketsUpdatedAt: now.UTC(),
			Rating:           domain.ArenaBaseRating,
			Rage:             0,
			RerollDay:        domain.ArenaDay(now),
			RerollsToday:     0,
			RerollReadyAt:    time.Unix(0, 0).UTC(),
		}
		if err := r.saveStateTx(ctx, tx, userID, st); err != nil {
			return domain.ArenaState{}, err
		}
		return st, nil
	}
	return st, err
}

func (r *ArenaRepo) saveStateTx(ctx context.Context, tx pgx.Tx, userID int64, st domain.ArenaState) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO arena_state (user_id, tickets, tickets_updated_at, rating, season_points, rage, reroll_day, rerolls_today, reroll_ready_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (user_id) DO UPDATE
		SET tickets = EXCLUDED.tickets,
		    tickets_updated_at = EXCLUDED.tickets_updated_at,
		    rating = EXCLUDED.rating,
		    season_points = EXCLUDED.season_points,
		    rage = EXCLUDED.rage,
		    reroll_day = EXCLUDED.reroll_day,
		    rerolls_today = EXCLUDED.rerolls_today,
		    reroll_ready_at = EXCLUDED.reroll_ready_at
	`, userID, st.Tickets, st.TicketsUpdatedAt, st.Rating, st.SeasonPoints, st.Rage, st.RerollDay, st.RerollsToday, st.RerollReadyAt)
 	return err
}

func (r *ArenaRepo) Reroll(ctx context.Context, userID int64, now time.Time, pay bool, energyCost int) (domain.ArenaState, int, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.ArenaState{}, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	st, err := r.getStateTx(ctx, tx, userID, now)
	if err != nil {
		return domain.ArenaState{}, 0, err
	}

	day := domain.ArenaDay(now)
	if st.RerollDay.IsZero() || !domain.ArenaDay(st.RerollDay).Equal(day) {
		st.RerollDay = day
		st.RerollsToday = 0
		st.RerollReadyAt = time.Time{}
	}

	freeOK := (st.RerollsToday == 0) || !now.Before(st.RerollReadyAt)
	if !pay && !freeOK {
		return st, 0, domain.ErrArenaRerollCooldown
	}

	energyNow := 0
	if pay {
		var energy int
		var updatedAt time.Time
		err = tx.QueryRow(ctx, `
			SELECT energy, energy_updated_at
			FROM cats
			WHERE user_id=$1
			FOR UPDATE
		`, userID).Scan(&energy, &updatedAt)
		if err != nil {
			return domain.ArenaState{}, 0, err
		}
		energyNow = domain.RegenEnergy(energy, updatedAt, now)
		if energyNow < energyCost {
			return st, energyNow, domain.ErrArenaNotEnoughEnergy
		}
		energyNow -= energyCost
		_, err = tx.Exec(ctx, `
			UPDATE cats
			SET energy=$2, energy_updated_at=$3, updated_at=now()
			WHERE user_id=$1
		`, userID, energyNow, now)
		if err != nil {
			return domain.ArenaState{}, 0, err
		}
	}

	st.RerollsToday++
	cd := domain.ArenaRerollCooldown(st.RerollsToday)
	st.RerollReadyAt = now.Add(cd)
	if err := r.saveStateTx(ctx, tx, userID, st); err != nil {
		return domain.ArenaState{}, 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ArenaState{}, 0, err
	}
	return st, energyNow, nil
}


func catPowerTx(ctx context.Context, tx pgx.Tx, userID int64) (int, error) {
	var power int
	err := tx.QueryRow(ctx, `
		SELECT ((atk_base * 3) + (def_base * 2) + (hp_base / 2) + (spd_base * 1) + (level * 5)) AS power
		FROM cats
		WHERE user_id = $1
	`, userID).Scan(&power)
	return power, err
}
