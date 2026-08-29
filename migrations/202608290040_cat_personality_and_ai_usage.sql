-- +goose Up
CREATE TABLE IF NOT EXISTS cat_personality (
  cat_id             BIGINT PRIMARY KEY REFERENCES cats(id) ON DELETE CASCADE,
  trait              TEXT NOT NULL,
  speech_style       TEXT NOT NULL,
  auto_speak_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO cat_personality (cat_id, trait, speech_style)
SELECT
  id,
  trait,
  CASE trait
    WHEN 'lazy' THEN 'сухой юмор, минимум энтузиазма, избегает лишней работы'
    WHEN 'bully' THEN 'самоуверенно поддевает, спорит и редко признаёт вину'
    WHEN 'philosopher' THEN 'видит великий смысл в бытовых мелочах и говорит с невозмутимым пафосом'
    WHEN 'neat' THEN 'аккуратен, придирчив к порядку и слегка осуждает чужой бардак'
    WHEN 'sleepy' THEN 'сонный, медленный и любую тему сводит к отдыху'
    ELSE 'коротко, по-кошачьи и с добродушной иронией'
  END
FROM cats
ON CONFLICT (cat_id) DO NOTHING;

-- Personality is created in the same database statement lifecycle as every new cat.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION create_default_cat_personality()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO cat_personality (cat_id, trait, speech_style)
  VALUES (
    NEW.id,
    NEW.trait,
    CASE NEW.trait
      WHEN 'lazy' THEN 'сухой юмор, минимум энтузиазма, избегает лишней работы'
      WHEN 'bully' THEN 'самоуверенно поддевает, спорит и редко признаёт вину'
      WHEN 'philosopher' THEN 'видит великий смысл в бытовых мелочах и говорит с невозмутимым пафосом'
      WHEN 'neat' THEN 'аккуратен, придирчив к порядку и слегка осуждает чужой бардак'
      WHEN 'sleepy' THEN 'сонный, медленный и любую тему сводит к отдыху'
      ELSE 'коротко, по-кошачьи и с добродушной иронией'
    END
  )
  ON CONFLICT (cat_id) DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_create_default_cat_personality ON cats;
CREATE TRIGGER trg_create_default_cat_personality
AFTER INSERT ON cats
FOR EACH ROW EXECUTE FUNCTION create_default_cat_personality();

CREATE TABLE IF NOT EXISTS ai_usage_log (
  id              BIGSERIAL PRIMARY KEY,
  generation_type TEXT NOT NULL,
  cat_id          BIGINT REFERENCES cats(id) ON DELETE SET NULL,
  yard_id         BIGINT,
  provider        TEXT NOT NULL,
  model           TEXT NOT NULL,
  tokens_in       INTEGER NOT NULL DEFAULT 0,
  tokens_out      INTEGER NOT NULL DEFAULT 0,
  latency_ms      INTEGER NOT NULL DEFAULT 0,
  success         BOOLEAN NOT NULL,
  blocked         BOOLEAN NOT NULL DEFAULT FALSE,
  fallback        BOOLEAN NOT NULL DEFAULT FALSE,
  error_code      TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_usage_log_created_at
  ON ai_usage_log (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_usage_log_cat_created_at
  ON ai_usage_log (cat_id, created_at DESC) WHERE cat_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS ai_usage_log;
DROP TRIGGER IF EXISTS trg_create_default_cat_personality ON cats;
DROP FUNCTION IF EXISTS create_default_cat_personality();
DROP TABLE IF EXISTS cat_personality;
