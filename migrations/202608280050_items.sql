-- +goose Up
CREATE TABLE IF NOT EXISTS cat_items (
  user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  item_id       TEXT NOT NULL,
  item_level    INT NOT NULL DEFAULT 1 CHECK (item_level BETWEEN 1 AND 5),
  fragments     INT NOT NULL DEFAULT 0 CHECK (fragments >= 0),
  discovered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, item_id)
);

CREATE TABLE IF NOT EXISTS cat_equipment (
  user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slot       TEXT NOT NULL CHECK (slot IN ('claws', 'collar', 'charm')),
  item_id    TEXT NOT NULL,
  equipped_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, slot),
  UNIQUE (user_id, item_id),
  FOREIGN KEY (user_id, item_id) REFERENCES cat_items(user_id, item_id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS cat_equipment;
DROP TABLE IF EXISTS cat_items;
