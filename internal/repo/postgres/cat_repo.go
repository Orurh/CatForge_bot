package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"catforge/internal/domain"
	"catforge/internal/pkg/randx"
)

// var ErrCatAlreadyExists = errors.New("cat already exists")
// var ErrNoCat = errors.New("cat not found")

type CatRepo struct {
	pool *pgxpool.Pool
	rng  randx.RNG
}

func NewCatRepo(pool *pgxpool.Pool, rng randx.RNG) *CatRepo { return &CatRepo{pool: pool, rng: rng} }

func (r *CatRepo) GetByUserID(ctx context.Context, userID int64) (*domain.Cat, error) {
	var c domain.Cat
	err := scanCatFull(r.pool.QueryRow(ctx, `
		SELECT id, user_id, name, breed, trait, level, xp, energy, last_train_at, energy_updated_at,
		       hp_base, atk_base, def_base, spd_base
		FROM cats
		WHERE user_id = $1
		`, userID), &c)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNoCat
		}
		return nil, err
	}
	return &c, nil
}

func (r *CatRepo) Create(ctx context.Context, userID int64, name string, breed domain.Breed, trait domain.Trait, hp, atk, def, spd int) (*domain.Cat, error) {
	var c domain.Cat
	err := scanCatBrief(r.pool.QueryRow(ctx, `
		INSERT INTO cats (user_id, name, breed, trait, hp_base, atk_base, def_base, spd_base)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING id, user_id, name, breed, trait, level, xp, energy, hp_base, atk_base, def_base, spd_base
	`, userID, name, string(breed), string(trait), hp, atk, def, spd), &c)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrCatAlreadyExists
		}
		return nil, err
	}

	return &c, nil
}

func (r *CatRepo) DeleteByUserID(ctx context.Context, userID int64) error {
	ct, err := r.pool.Exec(ctx, `
		DELETE FROM cats
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNoCat
	}
	return nil
}

func (r *CatRepo) SetName(ctx context.Context, userID int64, name string) (*domain.Cat, error) {
	var c domain.Cat
	err := scanCatBrief(r.pool.QueryRow(ctx, `
		UPDATE cats
		SET name = $2, updated_at = now()
		WHERE user_id = $1
		RETURNING id, user_id, name, breed, trait, level, xp, energy, hp_base, atk_base, def_base, spd_base
	`, userID, name), &c)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNoCat
		}
		return nil, err
	}
	return &c, nil
}

// Train атомарно: обновляет энергию (по timestamps), проверяет cooldown, тратит энергию, добавляет XP и (возможно) повышает level.
// Возвращает обновлённого кота и текст результата (для UI).
func (r *CatRepo) Train(ctx context.Context, userID int64, now time.Time) (*domain.Cat, domain.TrainResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, domain.TrainResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

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
			return nil, domain.TrainResult{}, domain.ErrNoCat
		}
		return nil, domain.TrainResult{}, err
	}

	// 1) Реген энергии: 1 единица / 20 секунд, максимум 100
	c.Energy = domain.RegenEnergy(c.Energy, c.EnergyUpdatedAt, now)
	c.EnergyUpdatedAt = now

	// 2) Стоимость энергии
	if c.Energy < domain.TrainingMinEnergy {
		_ = tx.QueryRow(ctx, `
			UPDATE cats SET energy = $2, energy_updated_at = $3, updated_at = now()
			WHERE user_id = $1
			RETURNING id
		`, userID, c.Energy, c.EnergyUpdatedAt).Scan(new(int64))
		if err := tx.Commit(ctx); err != nil {
			return nil, domain.TrainResult{}, err
		}
		return &c, domain.TrainResult{
			Outcome:    domain.TrainingNotEnoughEnergy,
			EffPercent: 100,
		}, nil
	}

	// 3) Рандом: расход энергии и XP (контролируемый)
	// energyCost: 18..30, но не больше текущей энергии
	cost := 18 + r.rng.Intn(13) // 18..30
	if cost > c.Energy {
		cost = c.Energy
	}
	// xpBase: 12..28 + небольшой бонус за уровень
	gainBase := float64(12+r.rng.Intn(17)) + float64(c.Level/2)

	eff, effPercent := domain.TrainingEfficiency(c.LastTrainAt, now)

	roll := r.rng.Intn(100)
	enc := domain.EncounterMicePack
	switch {
	case roll < 10:
		enc = domain.EncounterBigRat
	case roll < 45:
		enc = domain.EncounterMicePack
	case roll < 70:
		enc = domain.EncounterPigeon
	default:
		enc = domain.EncounterLizard
	}
	flavor := uint16(r.rng.Intn(1 << 16))

	crit := (enc == domain.EncounterBigRat)
	if crit {
		gainBase *= 2
	}

	gain := int64(gainBase * eff)
	if gain < 1 {
		gain = 1
	}

	c.Energy -= cost
	c.XP += gain

	// 4) Level up
	leveled := 0
	var gained domain.StatDelta

	for c.XP >= int64(c.Level)*100 {
		c.XP -= int64(c.Level) * 100
		c.Level++
		leveled++

		// растим базовые статы (и копим дельту)
		d := domain.ApplyLevelUps(&c, 1)
		gained.Add(d)
	}

	// 5) persist
	c.LastTrainAt = now
	err = tx.QueryRow(ctx, `
		UPDATE cats
		SET level = $2, xp = $3, energy = $4,
			last_train_at = $5, energy_updated_at = $6,
			hp_base = $7, atk_base = $8, def_base = $9, spd_base = $10,
			updated_at = now()
		WHERE user_id = $1
		RETURNING id
	`, userID, c.Level, c.XP, c.Energy, c.LastTrainAt, c.EnergyUpdatedAt,
		c.HPBase, c.ATKBase, c.DEFBase, c.SPDBase,
	).Scan(new(int64))
	if err != nil {
		return nil, domain.TrainResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, domain.TrainResult{}, err
	}

	return &c, domain.TrainResult{
		Outcome:     domain.TrainingOK,
		XPGain:      gain,
		EnergyCost:  cost,
		Crit:        crit,
		EffPercent:  effPercent,
		LeveledUp:   leveled,
		StatsGained: gained,
		Encounter:   enc,
		Flavor:      flavor,
	}, nil
}

// scanCatFull scans a full cat row (including timestamps)
func scanCatFull(row pgx.Row, c *domain.Cat) error {
	var breed, trait string
	if err := row.Scan(
		&c.ID, &c.UserID, &c.Name, &breed, &trait, &c.Level, &c.XP, &c.Energy,
		&c.LastTrainAt, &c.EnergyUpdatedAt,
		&c.HPBase, &c.ATKBase, &c.DEFBase, &c.SPDBase,
	); err != nil {
		return err
	}
	c.Breed = domain.Breed(breed)
	c.Trait = domain.Trait(trait)
	return nil
}

// scanCatBrief scans a cat row without timestamps (used by Create RETURNING)
func scanCatBrief(row pgx.Row, c *domain.Cat) error {
	var breed, trait string
	if err := row.Scan(
		&c.ID, &c.UserID, &c.Name, &breed, &trait, &c.Level, &c.XP, &c.Energy,
		&c.HPBase, &c.ATKBase, &c.DEFBase, &c.SPDBase,
	); err != nil {
		return err
	}
	c.Breed = domain.Breed(breed)
	c.Trait = domain.Trait(trait)
	return nil
}
