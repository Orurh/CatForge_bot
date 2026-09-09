-- +goose Up
ALTER TABLE support_invoices DROP CONSTRAINT support_invoices_stars_check;
ALTER TABLE support_invoices ADD CONSTRAINT support_invoices_stars_check CHECK (stars > 0);

-- +goose Down
-- Refuse rollback if custom amounts exist, preserving all financial records.
ALTER TABLE support_invoices DROP CONSTRAINT support_invoices_stars_check;
ALTER TABLE support_invoices ADD CONSTRAINT support_invoices_stars_check CHECK (stars IN (50,100,250));
