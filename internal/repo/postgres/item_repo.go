package postgres

import (
	"context"
	"errors"

	"catforge/internal/app"
	"catforge/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ItemRepo struct{ pool *pgxpool.Pool }

func NewItemRepo(pool *pgxpool.Pool) *ItemRepo { return &ItemRepo{pool: pool} }

func (r *ItemRepo) ListOwned(ctx context.Context, userID int64) ([]domain.OwnedItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.item_id, i.item_level, i.fragments, e.item_id IS NOT NULL
		FROM cat_items i
		LEFT JOIN cat_equipment e ON e.user_id = i.user_id AND e.item_id = i.item_id
		WHERE i.user_id = $1
		ORDER BY i.discovered_at, i.item_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.OwnedItem, 0)
	for rows.Next() {
		var item domain.OwnedItem
		if err := rows.Scan(&item.ItemID, &item.Level, &item.Fragments, &item.Equipped); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ItemRepo) ListBestiary(ctx context.Context, userID int64) ([]domain.BestiaryEntry, error) {
	rows, err := r.pool.Query(ctx, `SELECT enemy_kind, encounters, victories FROM cat_bestiary WHERE user_id=$1 ORDER BY first_seen_at, enemy_kind`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]domain.BestiaryEntry, 0)
	for rows.Next() {
		var entry domain.BestiaryEntry
		if err := rows.Scan(&entry.EnemyKind, &entry.Encounters, &entry.Victories); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *ItemRepo) Equip(ctx context.Context, userID int64, slot domain.ItemSlot, itemID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT true FROM cat_items WHERE user_id=$1 AND item_id=$2`, userID, itemID).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrItemNotOwned
		}
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO cat_equipment (user_id, slot, item_id)
		VALUES ($1,$2,$3)
		ON CONFLICT (user_id, slot) DO UPDATE SET item_id=EXCLUDED.item_id, equipped_at=now()
	`, userID, string(slot), itemID)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE cats SET state_version=state_version+1, updated_at=now() WHERE user_id=$1`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *ItemRepo) Upgrade(ctx context.Context, userID int64, itemID string, expectedLevel, fragments int, coins int64) (domain.OwnedItem, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.OwnedItem{}, err
	}
	defer tx.Rollback(ctx)

	var balance int64
	if err := tx.QueryRow(ctx, `SELECT coins FROM cats WHERE user_id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
		return domain.OwnedItem{}, err
	}
	if balance < coins {
		return domain.OwnedItem{}, domain.ErrNotEnoughCoins
	}
	var item domain.OwnedItem
	err = tx.QueryRow(ctx, `
		UPDATE cat_items
		SET item_level=item_level+1, fragments=fragments-$4, updated_at=now()
		WHERE user_id=$1 AND item_id=$2 AND item_level=$3 AND item_level<5 AND fragments >= $4
		RETURNING item_id, item_level, fragments
	`, userID, itemID, expectedLevel, fragments).Scan(&item.ItemID, &item.Level, &item.Fragments)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OwnedItem{}, domain.ErrNotEnoughFragments
	}
	if err != nil {
		return domain.OwnedItem{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE cats SET coins=coins-$2, state_version=state_version+1, updated_at=now() WHERE user_id=$1`, userID, coins); err != nil {
		return domain.OwnedItem{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.OwnedItem{}, err
	}
	return item, nil
}

func (r *ItemRepo) SaveExpedition(ctx context.Context, userID, expectedVersion int64, cat domain.Cat, dropItemID string, enemyKind domain.EnemyKind, victory bool) (app.ExpeditionSaveResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.ExpeditionSaveResult{}, err
	}
	defer tx.Rollback(ctx)

	command, err := tx.Exec(ctx, `
		UPDATE cats
		SET level=$2, xp=$3, coins=$4, energy=$5,
			last_train_at=$6, energy_updated_at=$7,
			hp_base=$8, atk_base=$9, def_base=$10, spd_base=$11,
			state_version=state_version+1, updated_at=now()
		WHERE user_id=$1 AND state_version=$12
	`, userID, cat.Level, cat.XP, cat.Coins, cat.Energy, cat.LastTrainAt, cat.EnergyUpdatedAt,
		cat.HPBase, cat.ATKBase, cat.DEFBase, cat.SPDBase, expectedVersion)
	if err != nil {
		return app.ExpeditionSaveResult{}, err
	}
	if command.RowsAffected() == 0 {
		return app.ExpeditionSaveResult{Saved: false}, nil
	}

	result := app.ExpeditionSaveResult{Saved: true}
	if enemyKind != "" {
		_, err = tx.Exec(ctx, `
			INSERT INTO cat_bestiary (user_id, enemy_kind, encounters, victories)
			VALUES ($1,$2,1,$3)
			ON CONFLICT (user_id, enemy_kind) DO UPDATE
			SET encounters=cat_bestiary.encounters+1,
				victories=cat_bestiary.victories+EXCLUDED.victories,
				last_seen_at=now()
		`, userID, string(enemyKind), boolToInt(victory))
		if err != nil {
			return app.ExpeditionSaveResult{}, err
		}
	}
	if dropItemID != "" {
		err = tx.QueryRow(ctx, `
			SELECT item_id, item_level, fragments
			FROM cat_items
			WHERE user_id=$1 AND item_id=$2
			FOR UPDATE
		`, userID, dropItemID).Scan(&result.Item.ItemID, &result.Item.Level, &result.Item.Fragments)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			result.IsNew = true
			err = tx.QueryRow(ctx, `
				INSERT INTO cat_items (user_id, item_id)
				VALUES ($1,$2)
				RETURNING item_id, item_level, fragments
			`, userID, dropItemID).Scan(&result.Item.ItemID, &result.Item.Level, &result.Item.Fragments)
		case err == nil:
			err = tx.QueryRow(ctx, `
				UPDATE cat_items
				SET fragments=fragments+1, updated_at=now()
				WHERE user_id=$1 AND item_id=$2
				RETURNING item_id, item_level, fragments
			`, userID, dropItemID).Scan(&result.Item.ItemID, &result.Item.Level, &result.Item.Fragments)
		}
		if err != nil {
			return app.ExpeditionSaveResult{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return app.ExpeditionSaveResult{}, err
	}
	return result, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
