-- +goose Up
-- Keep familiar traits; retire philosopher/neat/sleepy deterministically.
UPDATE cats SET trait = CASE trait
 WHEN 'lazy' THEN 'lazy'
 WHEN 'bully' THEN 'bully'
 WHEN 'philosopher' THEN 'veteran'
 WHEN 'neat' THEN 'cunning'
 WHEN 'sleepy' THEN CASE WHEN id % 2 = 0 THEN 'arrogant' ELSE 'angry' END
 WHEN 'veteran' THEN 'veteran'
 WHEN 'cunning' THEN 'cunning'
 WHEN 'arrogant' THEN 'arrogant'
 WHEN 'angry' THEN 'angry'
 ELSE (ARRAY['lazy','bully','veteran','cunning','arrogant','angry'])[1 + ((id % 6 + 6) % 6)::int]
 END,
 state_version = state_version + 1, updated_at = now();

INSERT INTO cat_personality (cat_id, trait, speech_style)
SELECT c.id, c.trait, CASE c.trait
WHEN 'lazy' THEN 'всё делает с минимальными усилиями; любое действие — лишняя работа; отвечает коротко и лениво'
WHEN 'bully' THEN 'ищет повод подколоть и устроить перепалку; провоцирует и первым лезет спорить; ему интересна реакция собеседника'
WHEN 'veteran' THEN 'бывалый дворовый кот, переживший десять зим; говорит спокойно, коротко и по делу; дешёвым понтом не удивить; без философии, метафор о жизни и пафоса'
WHEN 'cunning' THEN 'выкручивается и поворачивает ситуацию в свою пользу; хитрит, торгуется и прикидывается невинным'
WHEN 'arrogant' THEN 'считает, что ему все должны; требует обслуживания и считает чужое своим; не ищет перепалку, просто ждёт исполнения'
WHEN 'angry' THEN 'раздражительный и недоверчивый; огрызается, потому что всё действительно бесит; не подкалывает ради игры и не выступает на публику'
END FROM cats c
ON CONFLICT (cat_id) DO UPDATE
SET trait = EXCLUDED.trait, speech_style = EXCLUDED.speech_style, updated_at = now();

ALTER TABLE cats ADD CONSTRAINT cats_trait_valid
 CHECK (trait IN ('lazy','bully','veteran','cunning','arrogant','angry'));
ALTER TABLE cat_personality ADD CONSTRAINT cat_personality_trait_valid
 CHECK (trait IN ('lazy','bully','veteran','cunning','arrogant','angry'));

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION create_default_cat_personality()
RETURNS TRIGGER AS $$
BEGIN
 INSERT INTO cat_personality (cat_id, trait, speech_style)
 VALUES (NEW.id, NEW.trait, CASE NEW.trait
WHEN 'lazy' THEN 'всё делает с минимальными усилиями; любое действие — лишняя работа; отвечает коротко и лениво'
WHEN 'bully' THEN 'ищет повод подколоть и устроить перепалку; провоцирует и первым лезет спорить; ему интересна реакция собеседника'
WHEN 'veteran' THEN 'бывалый дворовый кот, переживший десять зим; говорит спокойно, коротко и по делу; дешёвым понтом не удивить; без философии, метафор о жизни и пафоса'
WHEN 'cunning' THEN 'выкручивается и поворачивает ситуацию в свою пользу; хитрит, торгуется и прикидывается невинным'
WHEN 'arrogant' THEN 'считает, что ему все должны; требует обслуживания и считает чужое своим; не ищет перепалку, просто ждёт исполнения'
WHEN 'angry' THEN 'раздражительный и недоверчивый; огрызается, потому что всё действительно бесит; не подкалывает ради игры и не выступает на публику'
END)
 ON CONFLICT (cat_id) DO NOTHING;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- A lossy trait conversion must not silently rewrite existing cats again.
-- Restore the pre-deploy backup into a separate database for rollback.
-- +goose StatementBegin
DO $$ BEGIN
 RAISE EXCEPTION 'Trait migration is irreversible; restore a pre-deploy backup into a separate database';
END $$;
-- +goose StatementEnd
