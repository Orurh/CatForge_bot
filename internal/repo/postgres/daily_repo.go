package postgres

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"time"

	"catforge/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DailyRepo struct {
	pool *pgxpool.Pool
}

func NewDailyRepo(pool *pgxpool.Pool) *DailyRepo {
	return &DailyRepo{pool: pool}
}

func (r *DailyRepo) GetState(ctx context.Context, userID int64) (domain.DailyState, error) {
	var last pgtype.Date
	var streak int
	err := r.pool.QueryRow(ctx, `
		SELECT daily_claim_date, daily_streak
		FROM users
		WHERE id = $1
	`, userID).Scan(&last, &streak)
	if err != nil {
		return domain.DailyState{}, err
	}
	var lastDay time.Time
	if last.Valid {
		lastDay = last.Time
	}
	return domain.DailyState{LastClaimDay: lastDay, Streak: streak}, nil
}

func (r *DailyRepo) Claim(ctx context.Context, userID int64, now time.Time) (*domain.Cat, domain.DailyClaimResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, domain.DailyClaimResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var last pgtype.Date
	var streak int
	err = tx.QueryRow(ctx, `
		SELECT daily_claim_date, daily_streak
		FROM users
		WHERE id = $1
		FOR UPDATE
	`, userID).Scan(&last, &streak)
	if err != nil {
		return nil, domain.DailyClaimResult{}, err
	}

	var c domain.Cat
	err = scanCatFull(tx.QueryRow(ctx, `
		SELECT id, user_id, name, breed, trait, level, xp, energy, last_train_at, energy_updated_at,
		       hp_base, atk_base, def_base, spd_base
		FROM cats
		WHERE user_id = $1
		FOR UPDATE
	`, userID), &c)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.DailyClaimResult{}, domain.ErrNoCat
		}
		return nil, domain.DailyClaimResult{}, err
	}

	claimDay := domain.DailyDay(now)
	var lastDay time.Time
	if last.Valid {
		lastDay = domain.DailyDay(last.Time)
	}

	if !lastDay.IsZero() && lastDay.Equal(claimDay) {
		if err := tx.Commit(ctx); err != nil {
			return nil, domain.DailyClaimResult{}, err
		}
		return &c, domain.DailyClaimResult{
			Outcome: domain.DailyClaimAlreadyClaimed,
			Streak:  streak,
			NextAt:  domain.NextDailyAt(now),
		}, nil
	}

	newStreak := 1
	if !lastDay.IsZero() && lastDay.Equal(claimDay.Add(-24*time.Hour)) {
		if streak > 0 {
			newStreak = streak + 1
		} else {
			newStreak = 1
		}
	}

	xpGain, energyGain := dailyReward(c.Breed, newStreak, userID, claimDay)

	energyNow := domain.RegenEnergy(c.Energy, c.EnergyUpdatedAt, now)
	energyAfter := energyNow + energyGain
	if energyAfter > domain.EnergyMax {
		energyAfter = domain.EnergyMax
	}

	_, err = tx.Exec(ctx, `
		UPDATE users
		SET daily_claim_date = $2::date,
		    daily_streak = $3
		WHERE id = $1
	`, userID, claimDay, newStreak)
	if err != nil {
		return nil, domain.DailyClaimResult{}, err
	}

	err = scanCatFull(tx.QueryRow(ctx, `
		UPDATE cats
		SET xp = xp + $2,
		    energy = $3,
		    energy_updated_at = $4,
		    updated_at = now()
		WHERE user_id = $1
		RETURNING id, user_id, name, breed, trait, level, xp, energy, last_train_at, energy_updated_at,
		          hp_base, atk_base, def_base, spd_base
	`, userID, xpGain, energyAfter, now), &c)
	if err != nil {
		return nil, domain.DailyClaimResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, domain.DailyClaimResult{}, err
	}

	return &c, domain.DailyClaimResult{
		Outcome:    domain.DailyClaimOK,
		Streak:     newStreak,
		XPGain:     xpGain,
		EnergyGain: energyGain,
		NextAt:     domain.NextDailyAt(now),
	}, nil
}

func dailyReward(breed domain.Breed, streak int, userID int64, claimDay time.Time) (xpGain int64, energyGain int) {
	y, m, d := claimDay.UTC().Date()
	key := fmt.Sprintf("%d:%04d%02d%02d", userID, y, int(m), d)
	roll := int(crc32.ChecksumIEEE([]byte(key)) % 9) 

	st := streak
	if st < 1 {
		st = 1
	}
	if st > 14 {
		st = 14
	}

	xp := 24 + int64(roll*2) + int64(minInt(st, 7)*4)     
	en := 18 + (roll%3)*4 + minInt(st, 7)*2                

	switch breed {
	case domain.BreedBengal:
		xp += 6
	case domain.BreedSiamese:
		xp += 3
	case domain.BreedMaineCoon:
		en += 6
	case domain.BreedBritish:
		en += 3
	}
	return xp, en
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}