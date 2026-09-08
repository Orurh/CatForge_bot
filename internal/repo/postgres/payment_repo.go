package postgres

import (
	"catforge/internal/app"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type PaymentRepo struct{ pool *pgxpool.Pool }

func NewPaymentRepo(pool *pgxpool.Pool) *PaymentRepo { return &PaymentRepo{pool: pool} }
func (r *PaymentRepo) CreateInvoice(ctx context.Context, p app.SupportInvoice) (app.SupportInvoice, error) {
	err := r.pool.QueryRow(ctx, `INSERT INTO support_invoices(user_id,telegram_id,invoice_payload,request_key,stars,terms_version,created_at,expires_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(request_key) DO UPDATE SET request_key=EXCLUDED.request_key
 RETURNING id,user_id,telegram_id,invoice_payload,stars,terms_version,created_at,expires_at`, p.UserID, p.TelegramID, p.Payload, p.RequestKey, p.Stars, p.TermsVersion, p.CreatedAt, p.ExpiresAt).Scan(&p.ID, &p.UserID, &p.TelegramID, &p.Payload, &p.Stars, &p.TermsVersion, &p.CreatedAt, &p.ExpiresAt)
	return p, err
}
func (r *PaymentRepo) ApproveCheckout(ctx context.Context, queryID string, p app.PaymentReceipt, now time.Time) (bool, error) {
	ct, err := r.pool.Exec(ctx, `UPDATE support_invoices i SET pre_checkout_query_id=$1 WHERE invoice_payload=$2 AND telegram_id=$3 AND stars=$4 AND expires_at>$5
 AND (pre_checkout_query_id IS NULL OR pre_checkout_query_id=$1)
 AND NOT EXISTS(SELECT 1 FROM payments p WHERE p.invoice_payload=i.invoice_payload)`, queryID, p.Payload, p.TelegramID, p.Stars, now)
	return ct.RowsAffected() == 1, err
}
func (r *PaymentRepo) RecordReceipt(ctx context.Context, p app.PaymentReceipt, now time.Time) error {
	if p.ChargeID == "" || p.TelegramID <= 0 {
		return errors.New("invalid payment receipt identity")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var userID *int64
	var invoiceTG int64
	var stars int
	reason := ""
	err = tx.QueryRow(ctx, `SELECT user_id,telegram_id,stars FROM support_invoices WHERE invoice_payload=$1 FOR UPDATE`, p.Payload).Scan(&userID, &invoiceTG, &stars)
	if errors.Is(err, pgx.ErrNoRows) {
		reason = "unknown_invoice"
		userID = nil
	} else if err != nil {
		return err
	}
	if reason == "" && (invoiceTG != p.TelegramID || stars != p.Stars || p.Currency != "XTR") {
		reason = "invoice_mismatch"
	}
	var other bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payments WHERE invoice_payload=$1 AND telegram_payment_charge_id<>$2)`, p.Payload, p.ChargeID).Scan(&other); err != nil {
		return err
	}
	if other {
		reason = "multiple_charges_for_invoice"
	}
	status := "paid"
	var paidAt, refundedAt *time.Time
	if p.Refunded {
		status = "refunded"
		refundedAt = &now
	} else {
		paidAt = &now
	}
	// The receipt and the state transition commit together. Unknown/mismatched
	// receipts remain in the ledger for manual review instead of disappearing.
	_, err = tx.Exec(ctx, `INSERT INTO payments(user_id,telegram_id,invoice_payload,currency,stars,status,telegram_payment_charge_id,review_reason,created_at,paid_at,refunded_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(telegram_payment_charge_id) DO NOTHING`, userID, p.TelegramID, p.Payload, p.Currency, p.Stars, status, p.ChargeID, reason, now, paidAt, refundedAt)
	if err != nil {
		return err
	}
	var id int64
	var storedTG int64
	var storedStars int
	var storedPayload, storedCurrency, storedReason string
	if err = tx.QueryRow(ctx, `SELECT id,telegram_id,stars,invoice_payload,currency,review_reason FROM payments WHERE telegram_payment_charge_id=$1 FOR UPDATE`, p.ChargeID).Scan(&id, &storedTG, &storedStars, &storedPayload, &storedCurrency, &storedReason); err != nil {
		return err
	}
	if storedTG != p.TelegramID || storedStars != p.Stars || storedPayload != p.Payload || storedCurrency != p.Currency {
		reason = "conflicting_receipt"
		_, err = tx.Exec(ctx, `UPDATE payments SET review_reason=$2 WHERE id=$1`, id, reason)
	} else if p.Refunded {
		_, err = tx.Exec(ctx, `UPDATE payments SET status='refunded',refunded_at=COALESCE(refunded_at,$2) WHERE id=$1`, id, now)
	} else {
		// An out-of-order success must never undo a refund.
		_, err = tx.Exec(ctx, `UPDATE payments SET paid_at=COALESCE(paid_at,$2) WHERE id=$1`, id, now)
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if reason != "" || storedReason != "" {
		return app.ErrPaymentMismatch
	}
	return nil
}
func (r *PaymentRepo) ClaimThankYou(ctx context.Context, now time.Time) (*app.SupportPayment, error) {
	var p app.SupportPayment
	err := r.pool.QueryRow(ctx, `UPDATE payments SET thank_you_retry_at=$1+interval '2 minutes' WHERE id=(SELECT id FROM payments
 WHERE status='paid' AND review_reason='' AND thank_you_sent_at IS NULL AND thank_you_retry_at<=$1
 ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED) RETURNING id,telegram_id,stars,telegram_payment_charge_id,status`, now).Scan(&p.ID, &p.TelegramID, &p.Stars, &p.ChargeID, &p.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &p, err
}
func (r *PaymentRepo) CompleteThankYou(ctx context.Context, id int64, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE payments SET thank_you_sent_at=$2 WHERE id=$1`, id, now)
	return err
}
func (r *PaymentRepo) GetPayment(ctx context.Context, id int64) (app.SupportPayment, error) {
	var p app.SupportPayment
	err := r.pool.QueryRow(ctx, `SELECT id,telegram_id,stars,telegram_payment_charge_id,status,refund_requested_at IS NOT NULL FROM payments WHERE id=$1`, id).Scan(&p.ID, &p.TelegramID, &p.Stars, &p.ChargeID, &p.Status, &p.RefundRequested)
	return p, err
}
func (r *PaymentRepo) MarkRefunded(ctx context.Context, id int64, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE payments SET status='refunded',refunded_at=COALESCE(refunded_at,$2) WHERE id=$1`, id, now)
	return err
}

func (r *PaymentRepo) BeginRefund(ctx context.Context, id int64, now time.Time) (bool, error) {
	ct, err := r.pool.Exec(ctx, `UPDATE payments SET refund_requested_at=$2 WHERE id=$1 AND status='paid' AND refund_requested_at IS NULL`, id, now)
	return ct.RowsAffected() == 1, err
}
