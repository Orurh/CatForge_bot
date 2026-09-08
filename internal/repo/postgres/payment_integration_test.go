package postgres

import (
	"catforge/internal/app"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"os"
	"sync"
	"testing"
	"time"
)

func paymentTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CATFORGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CATFORGE_TEST_DATABASE_URL")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	schema := pgx.Identifier{fmt.Sprintf("payment_test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg.ConnConfig)
	defer db.Close()
	if err = Migrate(db, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
func TestPaymentLedgerIntegration(t *testing.T) {
	pool := paymentTestPool(t)
	ctx := context.Background()
	repo := NewPaymentRepo(pool)
	uid, err := NewUserRepo(pool).EnsureUser(ctx, 123)
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewPaymentService(repo, app.SystemClock{}, "@support_test")
	invoice, err := service.CreateInvoice(ctx, uid, 123, 100, "request1")
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.CreateInvoice(ctx, uid, 123, 100, "request1")
	if err != nil || again.Payload != invoice.Payload {
		t.Fatalf("invoice retry: %+v %v", again, err)
	}
	p := app.PaymentReceipt{TelegramID: 123, Currency: "XTR", Stars: 100, Payload: invoice.Payload, ChargeID: "charge1"}
	for _, bad := range []app.PaymentReceipt{
		{TelegramID: 124, Currency: "XTR", Stars: 100, Payload: invoice.Payload},
		{TelegramID: 123, Currency: "USD", Stars: 100, Payload: invoice.Payload},
		{TelegramID: 123, Currency: "XTR", Stars: 250, Payload: invoice.Payload},
		{TelegramID: 123, Currency: "XTR", Stars: 100, Payload: "forged"},
	} {
		if ok, err := service.ApproveCheckout(ctx, "bad-query", bad); err != nil || ok {
			t.Fatalf("invalid checkout accepted: %v %v", ok, err)
		}
	}
	for i := 0; i < 2; i++ {
		if ok, err := service.ApproveCheckout(ctx, "checkout1", p); err != nil || !ok {
			t.Fatalf("valid checkout: %v %v", ok, err)
		}
	}
	if ok, _ := service.ApproveCheckout(ctx, "checkout2", p); ok {
		t.Fatal("second simultaneous checkout accepted")
	}
	var count int
	pool.QueryRow(ctx, "SELECT count(*) FROM payments").Scan(&count)
	if count != 0 {
		t.Fatal("checkout marked paid")
	}
	// Simultaneous duplicate successful-payment updates must create one record.
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := service.RecordReceipt(ctx, p); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM payments").Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate ledger entries: %d %v", count, err)
	}
	if ok, _ := service.ApproveCheckout(ctx, "checkout1", p); ok {
		t.Fatal("paid invoice accepted")
	}
	// Leased notification survives failure and process restart.
	now := time.Now().Add(time.Second)
	pending, err := repo.ClaimThankYou(ctx, now)
	if err != nil || pending == nil {
		t.Fatalf("pending thanks: %v %v", pending, err)
	}
	if repeat, _ := repo.ClaimThankYou(ctx, now); repeat != nil {
		t.Fatal("duplicate worker lease")
	}
	if retry, _ := repo.ClaimThankYou(ctx, now.Add(3*time.Minute)); retry == nil || retry.ID != pending.ID {
		t.Fatal("failed delivery not retried")
	}
	if err = repo.CompleteThankYou(ctx, pending.ID, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if repeat, _ := repo.ClaimThankYou(ctx, now.Add(6*time.Minute)); repeat != nil {
		t.Fatal("completed thanks replayed")
	}
	// Refund is idempotent and success arriving after it cannot resurrect payment.
	calls := 0
	refund := func(_ context.Context, tgID int64, charge string) error {
		calls++
		if tgID != 123 || charge != "charge1" {
			t.Fatal("wrong refund recipient")
		}
		return nil
	}
	if err = service.Refund(ctx, pending.ID, refund); err != nil {
		t.Fatal(err)
	}
	if err = service.Refund(ctx, pending.ID, refund); err != nil || calls != 1 {
		t.Fatalf("refund retry: %d %v", calls, err)
	}
	if err = service.RecordReceipt(ctx, p); err != nil {
		t.Fatal(err)
	}
	stored, _ := repo.GetPayment(ctx, pending.ID)
	if stored.Status != "refunded" {
		t.Fatal("late success undid refund")
	}
	bad := p
	bad.ChargeID = "unknowncharge"
	bad.Payload = "missing_invoice"
	if err = service.RecordReceipt(ctx, bad); !errors.Is(err, app.ErrPaymentMismatch) {
		t.Fatalf("mismatch: %v", err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM payments WHERE review_reason<>''").Scan(&count); err != nil || count != 1 {
		t.Fatal("mismatch was not durably recorded")
	}
	// Refund arriving before success stays refunded and can still be reconciled.
	second, err := service.CreateInvoice(ctx, uid, 123, 50, "request2")
	if err != nil {
		t.Fatal(err)
	}
	late := app.PaymentReceipt{TelegramID: 123, Currency: "XTR", Stars: 50, Payload: second.Payload, ChargeID: "charge2", Refunded: true}
	if err = service.RecordReceipt(ctx, late); err != nil {
		t.Fatal(err)
	}
	late.Refunded = false
	if err = service.RecordReceipt(ctx, late); err != nil {
		t.Fatal(err)
	}
	var status string
	var paidAt *time.Time
	if err = pool.QueryRow(ctx, "SELECT status,paid_at FROM payments WHERE telegram_payment_charge_id='charge2'").Scan(&status, &paidAt); err != nil || status != "refunded" || paidAt == nil {
		t.Fatalf("out of order: %s %v %v", status, paidAt, err)
	}
	// Invoice expiration blocks checkout but never drops a late success receipt.
	third, err := service.CreateInvoice(ctx, uid, 123, 250, "request3")
	if err != nil {
		t.Fatal(err)
	}
	pool.Exec(ctx, "UPDATE support_invoices SET expires_at=now()-interval '1 second' WHERE id=$1", third.ID)
	expired := app.PaymentReceipt{TelegramID: 123, Currency: "XTR", Stars: 250, Payload: third.Payload, ChargeID: "charge3"}
	if ok, _ := service.ApproveCheckout(ctx, "expired", expired); ok {
		t.Fatal("expired invoice accepted")
	}
	if err = service.RecordReceipt(ctx, expired); err != nil {
		t.Fatal("late payment lost", err)
	}
	// Canceled DB context must be retryable; it must not create an acknowledged record.
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err = service.RecordReceipt(canceled, expired); err == nil {
		t.Fatal("database failure swallowed")
	}
	// A lost refund response leaves durable intent. Retrying must not call API twice.
	var thirdID int64
	if err = pool.QueryRow(ctx, "SELECT id FROM payments WHERE telegram_payment_charge_id='charge3'").Scan(&thirdID); err != nil {
		t.Fatal(err)
	}
	attempts := 0
	uncertain := func(context.Context, int64, string) error { attempts++; return errors.New("response lost") }
	if err = service.Refund(ctx, thirdID, uncertain); err == nil {
		t.Fatal("uncertain refund reported success")
	}
	if err = service.Refund(ctx, thirdID, uncertain); err == nil || attempts != 1 {
		t.Fatal("uncertain refund reissued")
	}
	if err = service.ConfirmRefund(ctx, thirdID, func(context.Context, int64, string) (bool, error) { return false, nil }); err == nil {
		t.Fatal("unverified refund marked complete")
	}
	if err = service.ConfirmRefund(ctx, thirdID, func(context.Context, int64, string) (bool, error) { return true, nil }); err != nil {
		t.Fatal(err)
	}
	confirmed, _ := repo.GetPayment(ctx, thirdID)
	if confirmed.Status != "refunded" {
		t.Fatal("reconciliation not saved")
	}

}

func TestCustomSupportAmountsIntegration(t *testing.T) {
	pool := paymentTestPool(t)
	ctx := context.Background()
	repo := NewPaymentRepo(pool)
	uid, err := NewUserRepo(pool).EnsureUser(ctx, 123)
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewPaymentService(repo, app.SystemClock{}, "@support_test")
	for _, stars := range []int{1, 50, 137, 150, 250, 10000, app.MaxSupportStars} {
		key := fmt.Sprintf("custom-%d", stars)
		invoice, err := service.CreateInvoice(ctx, uid, 123, stars, key)
		if err != nil {
			t.Fatalf("create %d Stars: %v", stars, err)
		}
		receipt := app.PaymentReceipt{TelegramID: 123, Currency: "XTR", Stars: stars, Payload: invoice.Payload, ChargeID: key}
		if ok, err := service.ApproveCheckout(ctx, key, receipt); err != nil || !ok {
			t.Fatalf("checkout %d: %v %v", stars, ok, err)
		}
		for i := 0; i < 2; i++ {
			if err := service.RecordReceipt(ctx, receipt); err != nil {
				t.Fatal(err)
			}
		}
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM payments WHERE telegram_payment_charge_id=$1 AND stars=$2", key, stars).Scan(&count); err != nil || count != 1 {
			t.Fatalf("custom receipt changed or duplicated: %d %v", count, err)
		}
	}
	// The database must reject zero/negative amounts even if callers bypass Go.
	for _, stars := range []int{0, -1} {
		_, err := pool.Exec(ctx, "UPDATE support_invoices SET stars=$1 WHERE request_key='custom-137'", stars)
		if err == nil {
			t.Fatalf("database accepted %d Stars", stars)
		}
	}
}
