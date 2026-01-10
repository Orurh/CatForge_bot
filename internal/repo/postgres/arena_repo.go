package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"catforge/internal/domain"
)

type ArenaRepo struct{ pool *pgxpool.Pool }

func NewArenaRepo(pool *pgxpool.Pool) *ArenaRepo { return &ArenaRepo{pool: pool} }

func (r *ArenaRepo) GetState(ctx context.Context, userID int64) (domain.ArenaState, error) {
	var st domain.ArenaState
	err := r.pool.QueryRow(ctx, `
		SELECT tickets, tickets_updated_at, rating, season_points
		FROM arena_state
		WHERE user_id = $1
	`, userID).Scan(&st.Tickets, &st.TicketsUpdatedAt, &st.Rating, &st.SeasonPoints)

	if errors.Is(err, pgx.ErrNoRows) {
		// лениво 
		st = domain.ArenaState{
			Tickets:          domain.ArenaTicketsCap,
			TicketsUpdatedAt: time.Now().UTC(),
			Rating:           domain.ArenaBaseRating,
		}
		_ = r.SaveState(ctx, userID, st)
		return st, nil
	}
	return st, err
}

func (r *ArenaRepo) SaveState(ctx context.Context, userID int64, st domain.ArenaState) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO arena_state (user_id, tickets, tickets_updated_at, rating, season_points)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id) DO UPDATE
		SET tickets = EXCLUDED.tickets,
		    tickets_updated_at = EXCLUDED.tickets_updated_at,
		    rating = EXCLUDED.rating,
		    season_points = EXCLUDED.season_points
	`, userID, st.Tickets, st.TicketsUpdatedAt, st.Rating, st.SeasonPoints)
	return err
}

func (r *ArenaRepo) FindOpponents(ctx context.Context, userID int64, attackerPower int, seed string) ([]domain.ArenaOpponent, error) {
	// 3 окна: слабее / равный / сильнее
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
	for i, w := range windows {
		op, ok, err := r.findOneOpponent(ctx, userID, w.min, w.max, fmt.Sprintf("%s:%d", seed, i))
		if err != nil {
			return nil, err
		}
		if ok {
			op.Kind = w.kind
			out = append(out, op)
		}
	}

	// если совсем пусто (мало игроков), попробуем “широкий” поиск 1-2 целей
	if len(out) == 0 {
		op, ok, err := r.findOneOpponent(ctx, userID, attackerPower-99999, attackerPower+99999, seed+":wide")
		if err != nil {
			return nil, err
		}
		if ok {
			op.Kind = domain.ArenaOppEven
			out = append(out, op)
		}
	}
	return out, nil
}

func (r *ArenaRepo) findOneOpponent(ctx context.Context, userID int64, minPower, maxPower int, seed string) (domain.ArenaOpponent, bool, error) {
	const q = `
	WITH candidates AS (
		SELECT
			c.user_id,
			c.name,
			c.breed,
			c.level,
			((c.atk_base * 3) + (c.def_base * 2) + (c.hp_base / 2) + (c.spd_base * 1) + (c.level * 5)) AS power
		FROM cats c
		WHERE c.user_id <> $1
		  AND ((c.atk_base * 3) + (c.def_base * 2) + (c.hp_base / 2) + (c.spd_base * 1) + (c.level * 5)) BETWEEN $2 AND $3
	)
	SELECT user_id, name, breed, level, power
	FROM candidates
	ORDER BY md5((user_id::text) || $4)
	LIMIT 1
	`
	var op domain.ArenaOpponent
	var breed string
	err := r.pool.QueryRow(ctx, q, userID, minPower, maxPower, seed).Scan(
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

func (r *ArenaRepo) Fight(ctx context.Context, userID int64, opponentUserID int64, seed string) (domain.ArenaState, domain.ArenaFightResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1) state + regen + consume ticket
	st, err := r.getStateTx(ctx, tx, userID)
	if err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, err
	}
	st = domain.RegenArenaTickets(st, time.Now().UTC())
	if st.Tickets <= 0 {
		return st, domain.ArenaFightResult{}, errors.New("arena: no tickets")
	}
	st.Tickets--

	// 2) load powers (SQL = domain.Power)
	ap, err := catPowerTx(ctx, tx, userID)
	if err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, err
	}
	dp, err := catPowerTx(ctx, tx, opponentUserID)
	if err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, err
	}

	res := domain.ArenaResolveWithRage(seed, ap, dp, st.Rage)
	res.NewRating = st.Rating + res.RatingDelta
	if res.NewRating < 0 {
		res.NewRating = 0
	}
	st.Rating = res.NewRating
	if res.AttackerWon {
		st.SeasonPoints += 10
		st.Rage = 0
	} else {
		st.SeasonPoints += 3
		st.Rage++
		if st.Rage > domain.ArenaRageCap{
			st.Rage = domain.ArenaRageCap
		}
	}

    // 2.5) reward XP to attacker (more for win)
    xpGain := int64(12)
    if res.AttackerWon {
        xpGain = 28
    }

    // Load attacker cat (FOR UPDATE) to apply XP + possible levelups.
    var c domain.Cat
    err = scanCatFull(tx.QueryRow(ctx, `
        SELECT id, user_id, name, breed, trait, level, xp, energy, last_train_at, energy_updated_at,
               hp_base, atk_base, def_base, spd_base
        FROM cats WHERE user_id=$1
        FOR UPDATE
    `, userID), &c)
    if err != nil { return domain.ArenaState{}, domain.ArenaFightResult{}, err }

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
    if err != nil { return domain.ArenaState{}, domain.ArenaFightResult{}, err }


	// 3) persist state
	if err := r.saveStateTx(ctx, tx, userID, st); err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, err
	}

	// 4) match log
	_, err = tx.Exec(ctx, `
		INSERT INTO arena_matches(attacker_id, defender_id, seed, attacker_power, defender_power, attacker_won, rating_delta)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, userID, opponentUserID, seed, ap, dp, res.AttackerWon, res.RatingDelta)
	if err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ArenaState{}, domain.ArenaFightResult{}, err
	}
	return st, res, nil
}

func (r *ArenaRepo) getStateTx(ctx context.Context, tx pgx.Tx, userID int64) (domain.ArenaState, error) {
	var st domain.ArenaState
	err := tx.QueryRow(ctx, `
		SELECT tickets, tickets_updated_at, rating, season_points
		FROM arena_state WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&st.Tickets, &st.TicketsUpdatedAt, &st.Rating, &st.SeasonPoints)

	if errors.Is(err, pgx.ErrNoRows) {
		st = domain.ArenaState{
			Tickets:          domain.ArenaTicketsCap,
			TicketsUpdatedAt: time.Now().UTC(),
			Rating:           domain.ArenaBaseRating,
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
		INSERT INTO arena_state (user_id, tickets, tickets_updated_at, rating, season_points)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id) DO UPDATE
		SET tickets = EXCLUDED.tickets,
		    tickets_updated_at = EXCLUDED.tickets_updated_at,
		    rating = EXCLUDED.rating,
		    season_points = EXCLUDED.season_points
	`, userID, st.Tickets, st.TicketsUpdatedAt, st.Rating, st.SeasonPoints)
	return err
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
