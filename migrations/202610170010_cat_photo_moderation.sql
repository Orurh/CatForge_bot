-- +goose Up
-- Photo reviews and Stars purchases are intentionally separate from support payments.
-- Requests retain cat_id even after cat deletion; free entitlement belongs to the user.
CREATE TABLE cat_photo_state (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    free_used BOOLEAN NOT NULL DEFAULT FALSE,
    current_cat_id BIGINT,
    current_file_id TEXT,
    current_request_id BIGINT,
    CONSTRAINT cat_photo_current_pair CHECK (
        (current_cat_id IS NULL AND current_file_id IS NULL AND current_request_id IS NULL)
        OR (current_cat_id IS NOT NULL AND current_file_id IS NOT NULL AND current_request_id IS NOT NULL)
    )
);
CREATE TABLE cat_photo_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cat_id BIGINT NOT NULL,
    file_id TEXT NOT NULL CHECK (length(file_id) BETWEEN 1 AND 512),
    status TEXT NOT NULL CHECK (status IN ('pending', 'awaiting_payment', 'applied', 'rejected', 'superseded', 'refunded')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at TIMESTAMPTZ,
    reviewer_telegram_id BIGINT
);
CREATE UNIQUE INDEX cat_photo_one_unfinished ON cat_photo_requests(user_id)
    WHERE status IN ('pending', 'awaiting_payment');
CREATE INDEX cat_photo_pending ON cat_photo_requests(id) WHERE status = 'pending';

CREATE TABLE cat_photo_invoices (
    id BIGSERIAL PRIMARY KEY,
    request_id BIGINT NOT NULL UNIQUE REFERENCES cat_photo_requests(id) ON DELETE RESTRICT,
    telegram_id BIGINT NOT NULL CHECK (telegram_id > 0),
    invoice_payload TEXT NOT NULL UNIQUE,
    stars INTEGER NOT NULL CHECK (stars = 10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    pre_checkout_query_id TEXT UNIQUE
);
CREATE TABLE cat_photo_payments (
    id BIGSERIAL PRIMARY KEY,
    request_id BIGINT REFERENCES cat_photo_requests(id) ON DELETE SET NULL,
    telegram_id BIGINT NOT NULL,
    invoice_payload TEXT NOT NULL,
    currency TEXT NOT NULL,
    stars INTEGER NOT NULL,
    telegram_payment_charge_id TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL CHECK (status IN ('paid', 'refunded')),
    review_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    refunded_at TIMESTAMPTZ
);
CREATE INDEX cat_photo_payment_review ON cat_photo_payments(id)
    WHERE review_reason <> '';

-- +goose Down
DROP TABLE cat_photo_payments;
DROP TABLE cat_photo_invoices;
DROP TABLE cat_photo_requests;
DROP TABLE cat_photo_state;
