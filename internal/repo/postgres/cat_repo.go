package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"catforge/internal/domain"
)

// var ErrCatAlreadyExists = errors.New("cat already exists")
// var ErrNoCat = errors.New("cat not found")

type CatRepo struct{ pool *pgxpool.Pool }

func NewCatRepo(pool *pgxpool.Pool) *CatRepo { return &CatRepo{pool: pool} }

func (r *CatRepo) GetByUserID(ctx context.Context, userID int64) (*domain.Cat, error) {
	var c domain.Cat
	err := scanCatFull(r.pool.QueryRow(ctx, `
		SELECT id, user_id, state_version, name, breed, trait, level, xp, coins, energy, last_train_at, energy_updated_at,
		       hp_base, atk_base, def_base, spd_base, claws_tenth_mm, weight_grams, tail_mm, whisker_span_mm, first_item_granted
		FROM cats
		WHERE user_id = $1
		`, userID), &c)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNoCat
		}
		return nil, err
	}
	c.TrainingCritBonusPercent, err = trainingCritBonus(ctx, r.pool, userID, c.Level)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CatRepo) Create(ctx context.Context, userID int64, name string, breed domain.Breed, trait domain.Trait, hp, atk, def, spd int) (*domain.Cat, error) {
	var c domain.Cat
	physical := domain.BaseFelineStats(breed)
	err := scanCatBrief(r.pool.QueryRow(ctx, `
		INSERT INTO cats (user_id, name, breed, trait, hp_base, atk_base, def_base, spd_base, claws_tenth_mm, weight_grams, tail_mm, whisker_span_mm)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING id, user_id, state_version, name, breed, trait, level, xp, coins, energy, hp_base, atk_base, def_base, spd_base, claws_tenth_mm, weight_grams, tail_mm, whisker_span_mm, first_item_granted
	`, userID, name, string(breed), string(trait), hp, atk, def, spd, physical.ClawsTenthMM, physical.WeightGrams, physical.TailMM, physical.WhiskerSpanMM), &c)
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
		SET name = $2, state_version = state_version + 1, updated_at = now()
		WHERE user_id = $1
		RETURNING id, user_id, state_version, name, breed, trait, level, xp, coins, energy, hp_base, atk_base, def_base, spd_base, claws_tenth_mm, weight_grams, tail_mm, whisker_span_mm, first_item_granted
	`, userID, name), &c)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNoCat
		}
		return nil, err
	}
	return &c, nil
}

// SaveProgress uses optimistic concurrency: a state calculated from an old
// snapshot cannot overwrite a newer game action.
func (r *CatRepo) SaveProgress(ctx context.Context, userID, expectedVersion int64, c domain.Cat) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	c.Feline = c.PhysicalStats()
	ct, err := tx.Exec(ctx, `
		UPDATE cats
		SET level = $2, xp = $3, coins = $4, energy = $5,
			last_train_at = $6, energy_updated_at = $7,
			hp_base = $8, atk_base = $9, def_base = $10, spd_base = $11,
			claws_tenth_mm=$13, weight_grams=$14, tail_mm=$15, whisker_span_mm=$16, first_item_granted=$17,
			state_version = state_version + 1,
			updated_at = now()
		WHERE user_id = $1 AND state_version = $12
	`, userID, c.Level, c.XP, c.Coins, c.Energy, c.LastTrainAt, c.EnergyUpdatedAt,
		c.HPBase, c.ATKBase, c.DEFBase, c.SPDBase, expectedVersion,
		c.Feline.ClawsTenthMM, c.Feline.WeightGrams, c.Feline.TailMM, c.Feline.WhiskerSpanMM, c.FirstItemGranted,
	)
	if err != nil {
		return false, err
	}
	if ct.RowsAffected() != 1 {
		return false, nil
	}
	c.StateVersion = expectedVersion + 1
	if err = persistProgressionReward(ctx, tx, &c); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// scanCatFull scans a full cat row (including timestamps)
func scanCatFull(row pgx.Row, c *domain.Cat) error {
	var breed, trait string
	if err := row.Scan(
		&c.ID, &c.UserID, &c.StateVersion, &c.Name, &breed, &trait, &c.Level, &c.XP, &c.Coins, &c.Energy,
		&c.LastTrainAt, &c.EnergyUpdatedAt,
		&c.HPBase, &c.ATKBase, &c.DEFBase, &c.SPDBase,
		&c.Feline.ClawsTenthMM, &c.Feline.WeightGrams, &c.Feline.TailMM, &c.Feline.WhiskerSpanMM, &c.FirstItemGranted,
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
		&c.ID, &c.UserID, &c.StateVersion, &c.Name, &breed, &trait, &c.Level, &c.XP, &c.Coins, &c.Energy,
		&c.HPBase, &c.ATKBase, &c.DEFBase, &c.SPDBase,
		&c.Feline.ClawsTenthMM, &c.Feline.WeightGrams, &c.Feline.TailMM, &c.Feline.WhiskerSpanMM, &c.FirstItemGranted,
	); err != nil {
		return err
	}
	c.Breed = domain.Breed(breed)
	c.Trait = domain.Trait(trait)
	return nil
}

func (r *CatRepo) EquippedStats(ctx context.Context, userID int64, level int) (domain.FelineStats, error) {
	bonus, _, err := physicalEquipment(ctx, r.pool, userID, level)
	return bonus, err
}

func (r *CatRepo) RecentProgressionFacts(ctx context.Context, catID int64) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT facts FROM cat_progression_facts WHERE cat_id=$1 ORDER BY state_version DESC LIMIT 5`, catID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var facts []string
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		var batch []string
		if err = json.Unmarshal(data, &batch); err != nil {
			return nil, err
		}
		facts = append(facts, batch...)
	}
	return facts, rows.Err()
}
