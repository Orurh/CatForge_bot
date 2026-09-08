-- +goose Up
CREATE TABLE support_invoices (
 id BIGSERIAL PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 telegram_id BIGINT NOT NULL CHECK (telegram_id > 0),
 product_id TEXT NOT NULL DEFAULT 'catforge_support' CHECK (product_id = 'catforge_support'),
 invoice_payload TEXT NOT NULL UNIQUE,
 request_key TEXT NOT NULL UNIQUE,
 stars INTEGER NOT NULL CHECK (stars IN (50,100,250)),
 terms_version TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 pre_checkout_query_id TEXT UNIQUE
);
-- Financial receipts are independent of the cat and retained even for mismatches.
CREATE TABLE payments (
 id BIGSERIAL PRIMARY KEY,
 user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
 telegram_id BIGINT NOT NULL,
 product_id TEXT NOT NULL DEFAULT 'catforge_support',
 invoice_payload TEXT NOT NULL,
 currency TEXT NOT NULL,
 stars INTEGER NOT NULL,
 status TEXT NOT NULL CHECK (status IN ('paid','refunded')),
 telegram_payment_charge_id TEXT NOT NULL UNIQUE,
 review_reason TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL,
 paid_at TIMESTAMPTZ,
 refunded_at TIMESTAMPTZ,
 refund_requested_at TIMESTAMPTZ,
 thank_you_sent_at TIMESTAMPTZ,
 thank_you_retry_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX payments_invoice ON payments(invoice_payload);
CREATE INDEX payments_pending_thanks ON payments(thank_you_retry_at) WHERE status='paid' AND thank_you_sent_at IS NULL AND review_reason='';

-- +goose Down
DROP TABLE payments;
DROP TABLE support_invoices;
