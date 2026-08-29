package postgres

import (
	"context"
	"errors"

	"catforge/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PersonalityRepo struct{ pool *pgxpool.Pool }

func NewPersonalityRepo(pool *pgxpool.Pool) *PersonalityRepo { return &PersonalityRepo{pool: pool} }

func (r *PersonalityRepo) GetByCatID(ctx context.Context, catID int64) (*domain.CatPersonality, error) {
	var personality domain.CatPersonality
	var trait, humorMode string
	err := r.pool.QueryRow(ctx, `
		SELECT cat_id, trait, speech_style, humor_mode, auto_speak_enabled, created_at, updated_at
		FROM cat_personality
		WHERE cat_id = $1
	`, catID).Scan(&personality.CatID, &trait, &personality.SpeechStyle, &humorMode, &personality.AutoSpeakEnabled, &personality.CreatedAt, &personality.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNoCat
		}
		return nil, err
	}
	personality.Trait = domain.Trait(trait)
	personality.HumorMode = domain.HumorMode(humorMode)
	return &personality, nil
}

func (r *PersonalityRepo) SetTrait(ctx context.Context, catID int64, trait domain.Trait, speechStyle string) (*domain.CatPersonality, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE cats SET trait = $2, updated_at = now() WHERE id = $1`, catID, string(trait)); err != nil {
		return nil, err
	}
	var personality domain.CatPersonality
	var storedTrait, humorMode string
	err = tx.QueryRow(ctx, `
		UPDATE cat_personality
		SET trait = $2, speech_style = $3, updated_at = now()
		WHERE cat_id = $1
		RETURNING cat_id, trait, speech_style, humor_mode, auto_speak_enabled, created_at, updated_at
	`, catID, string(trait), speechStyle).Scan(
		&personality.CatID, &storedTrait, &personality.SpeechStyle, &humorMode,
		&personality.AutoSpeakEnabled, &personality.CreatedAt, &personality.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNoCat
		}
		return nil, err
	}
	personality.Trait = domain.Trait(storedTrait)
	personality.HumorMode = domain.HumorMode(humorMode)
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &personality, nil
}

func (r *PersonalityRepo) SetHumorMode(ctx context.Context, catID int64, mode domain.HumorMode) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE cat_personality
		SET humor_mode = $2, updated_at = now()
		WHERE cat_id = $1
	`, catID, string(mode))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNoCat
	}
	return nil
}

func (r *PersonalityRepo) SetAutoSpeak(ctx context.Context, catID int64, enabled bool) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE cat_personality
		SET auto_speak_enabled = $2, updated_at = now()
		WHERE cat_id = $1
	`, catID, enabled)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNoCat
	}
	return nil
}
