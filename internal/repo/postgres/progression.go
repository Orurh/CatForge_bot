package postgres

import (
	"catforge/internal/domain"
	"catforge/internal/gamedata"
	"catforge/internal/gameengine"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

// The cat row is locked throughout calculation and reward persistence. Failure
// of the engine or any SQL statement rolls back the entire social action.
func awardCatXP(ctx context.Context, tx pgx.Tx, engine gameengine.ProgressionEngine, catID, gain int64, source string, seed uint64, tier string, secret bool) (domain.Cat, error) {
	if engine == nil {
		return domain.Cat{}, errors.New("progression engine is required")
	}
	var cat domain.Cat
	err := scanCatFull(tx.QueryRow(ctx, `SELECT id,user_id,state_version,name,breed,trait,level,xp,coins,energy,last_train_at,energy_updated_at,hp_base,atk_base,def_base,spd_base,claws_tenth_mm,weight_grams,tail_mm,whisker_span_mm,first_item_granted FROM cats WHERE id=$1 FOR UPDATE`, catID), &cat)
	if err != nil {
		return cat, err
	}
	next, err := engine.Progress(ctx, gameengine.ProgressInput{Cat: cat, XPGain: gain, Source: source, Seed: seed, OutcomeTier: tier, SecretFound: secret})
	if err != nil {
		return cat, err
	}
	next.StateVersion = cat.StateVersion + 1
	_, err = tx.Exec(ctx, `UPDATE cats SET level=$2,xp=$3,hp_base=$4,atk_base=$5,def_base=$6,spd_base=$7,claws_tenth_mm=$8,weight_grams=$9,tail_mm=$10,whisker_span_mm=$11,first_item_granted=$12,state_version=$13,updated_at=now() WHERE id=$1`, catID, next.Level, next.XP, next.HPBase, next.ATKBase, next.DEFBase, next.SPDBase, next.Feline.ClawsTenthMM, next.Feline.WeightGrams, next.Feline.TailMM, next.Feline.WhiskerSpanMM, next.FirstItemGranted, next.StateVersion)
	if err != nil {
		return cat, err
	}
	if source == "yard" && tier == "exceptional" {
		next.ProgressionFacts = append(next.ProgressionFacts, "exceptional_event")
	}
	err = persistProgressionReward(ctx, tx, &next)
	return next, err
}

func persistProgressionReward(ctx context.Context, tx pgx.Tx, cat *domain.Cat) error {
	if cat.LootItemID != "" {
		definition, ok := gamedata.ItemByID(cat.LootItemID)
		if !ok {
			return errors.New("unknown engine loot")
		}
		var level, fragments int
		var isNew bool
		err := tx.QueryRow(ctx, `INSERT INTO cat_items(user_id,item_id) VALUES($1,$2) ON CONFLICT(user_id,item_id) DO UPDATE SET fragments=cat_items.fragments+1,updated_at=now() RETURNING item_level,fragments,(xmax=0)`, cat.UserID, cat.LootItemID).Scan(&level, &fragments, &isNew)
		if err != nil {
			return err
		}
		beforeLevel := level
		for level < domain.ItemMaxLevel {
			cost, _, ok := domain.UpgradeCost(level)
			if !ok || fragments < cost {
				break
			}
			fragments -= cost
			level++
		}
		if !isNew {
			kind := "item_fragment="
			if level > beforeLevel {
				kind = "item_upgraded="
			}
			for i, fact := range cat.ProgressionFacts {
				if fact == "new_item="+cat.LootItemID {
					cat.ProgressionFacts[i] = kind + cat.LootItemID
				}
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE cat_items SET item_level=$3,fragments=$4 WHERE user_id=$1 AND item_id=$2`, cat.UserID, cat.LootItemID, level, fragments); err != nil {
			return err
		}
		if domain.SlotUnlocked(definition.Slot, cat.Level) {
			if _, err = tx.Exec(ctx, `INSERT INTO cat_equipment(user_id,slot,item_id) VALUES($1,$2,$3) ON CONFLICT(user_id,slot) DO NOTHING`, cat.UserID, string(definition.Slot), cat.LootItemID); err != nil {
				return err
			}
		}
	}
	if len(cat.ProgressionFacts) > 0 {
		data, err := json.Marshal(cat.ProgressionFacts)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO cat_progression_facts(cat_id,state_version,facts) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, cat.ID, cat.StateVersion, data)
		return err
	}
	return nil
}
