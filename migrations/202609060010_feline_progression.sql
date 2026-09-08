-- +goose Up
ALTER TABLE cats
 ADD COLUMN claws_tenth_mm INTEGER NOT NULL DEFAULT 100 CHECK (claws_tenth_mm > 0),
 ADD COLUMN weight_grams INTEGER NOT NULL DEFAULT 5000 CHECK (weight_grams > 0),
 ADD COLUMN tail_mm INTEGER NOT NULL DEFAULT 300 CHECK (tail_mm > 0),
 ADD COLUMN whisker_span_mm INTEGER NOT NULL DEFAULT 300 CHECK (whisker_span_mm > 0),
 ADD COLUMN first_item_granted BOOLEAN NOT NULL DEFAULT FALSE;
-- Preserve every earned legacy stat increment; do not reset level, XP, or items.
UPDATE cats SET
 claws_tenth_mm = CASE breed WHEN 'bengal' THEN 150 ELSE 100 END + GREATEST(0,atk_base-CASE breed WHEN 'siamese' THEN 18 WHEN 'bengal' THEN 22 ELSE 20 END)*5,
 weight_grams = CASE breed WHEN 'maine_coon' THEN 6000 WHEN 'siamese' THEN 4000 WHEN 'british' THEN 5500 WHEN 'bengal' THEN 4500 ELSE 5000 END + GREATEST(0,hp_base-CASE breed WHEN 'maine_coon' THEN 46 WHEN 'siamese' THEN 42 WHEN 'british' THEN 42 ELSE 40 END)*100,
 tail_mm = CASE breed WHEN 'maine_coon' THEN 320 WHEN 'siamese' THEN 400 WHEN 'british' THEN 250 WHEN 'bengal' THEN 350 ELSE 300 END + GREATEST(0,spd_base-CASE breed WHEN 'maine_coon' THEN 14 WHEN 'siamese' THEN 26 WHEN 'british' THEN 16 ELSE 20 END)*10,
 whisker_span_mm = CASE breed WHEN 'maine_coon' THEN 180 WHEN 'bengal' THEN 200 ELSE 300 END + GREATEST(0,def_base-CASE breed WHEN 'maine_coon' THEN 18 WHEN 'siamese' THEN 18 WHEN 'british' THEN 22 WHEN 'bengal' THEN 16 ELSE 20 END)*10,
 first_item_granted = EXISTS (SELECT 1 FROM cat_items i WHERE i.user_id=cats.user_id),
 state_version = state_version+1;
ALTER TABLE yard_event_choices ADD COLUMN special_action TEXT NOT NULL DEFAULT '', ADD COLUMN capability_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;
CREATE TABLE cat_progression_facts (
 cat_id BIGINT NOT NULL REFERENCES cats(id) ON DELETE CASCADE,
 state_version BIGINT NOT NULL,
 facts JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(cat_id,state_version)
);
ALTER TABLE yard_event_results DROP CONSTRAINT yard_event_results_outcome_tier_check;
UPDATE yard_event_results SET outcome_tier='fail' WHERE outcome_tier='failure';
ALTER TABLE yard_event_results ADD CONSTRAINT yard_event_results_outcome_tier_check CHECK(outcome_tier IN ('fail','partial','success','exceptional'));

-- +goose Down
ALTER TABLE yard_event_results DROP CONSTRAINT yard_event_results_outcome_tier_check;
UPDATE yard_event_results SET outcome_tier='failure' WHERE outcome_tier='fail';
ALTER TABLE yard_event_results ADD CONSTRAINT yard_event_results_outcome_tier_check CHECK(outcome_tier IN ('failure','partial','success','exceptional'));

DROP TABLE cat_progression_facts;
ALTER TABLE yard_event_choices DROP COLUMN special_action, DROP COLUMN capability_snapshot;
ALTER TABLE cats DROP COLUMN claws_tenth_mm, DROP COLUMN weight_grams, DROP COLUMN tail_mm, DROP COLUMN whisker_span_mm, DROP COLUMN first_item_granted;
