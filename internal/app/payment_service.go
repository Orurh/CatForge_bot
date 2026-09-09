package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math"
	"strings"
	"time"
)

const SupportTermsVersion = "2026-09-08-v2"
const SupportProductID = "catforge_support"

// Match the ledger's PostgreSQL INTEGER capacity; there is no preset-only limit.
// Telegram can apply its own limits when creating or paying an invoice.
const MaxSupportStars = math.MaxInt32

var ErrPaymentMismatch = errors.New("payment receipt requires review")

type SupportInvoice struct {
	ID           int64
	UserID       int64
	TelegramID   int64
	Stars        int
	Payload      string
	RequestKey   string
	TermsVersion string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}
type PaymentReceipt struct {
	TelegramID int64
	Currency   string
	Stars      int
	Payload    string
	ChargeID   string
	Refunded   bool
}
type SupportPayment struct {
	ID              int64
	TelegramID      int64
	Stars           int
	ChargeID        string
	Status          string
	RefundRequested bool
}
type PaymentRepository interface {
	CreateInvoice(context.Context, SupportInvoice) (SupportInvoice, error)
	ApproveCheckout(context.Context, string, PaymentReceipt, time.Time) (bool, error)
	RecordReceipt(context.Context, PaymentReceipt, time.Time) error
	ClaimThankYou(context.Context, time.Time) (*SupportPayment, error)
	CompleteThankYou(context.Context, int64, time.Time) error
	BeginRefund(context.Context, int64, time.Time) (bool, error)
	GetPayment(context.Context, int64) (SupportPayment, error)
	MarkRefunded(context.Context, int64, time.Time) error
}
type PaymentService struct {
	repo    PaymentRepository
	clock   Clock
	Contact string
}

func NewPaymentService(repo PaymentRepository, clock Clock, contact string) *PaymentService {
	return &PaymentService{repo: repo, clock: clock, Contact: strings.TrimSpace(contact)}
}
func (s *PaymentService) Enabled() bool { return s != nil && s.repo != nil && s.Contact != "" }
func ValidSupportAmount(stars int) bool { return stars >= 1 && stars <= MaxSupportStars }
func (s *PaymentService) CreateInvoice(ctx context.Context, userID, telegramID int64, stars int, requestKey string) (SupportInvoice, error) {
	if !s.Enabled() || !ValidSupportAmount(stars) || userID <= 0 || telegramID <= 0 || requestKey == "" {
		return SupportInvoice{}, errors.New("invalid support invoice")
	}
	var token [24]byte
	if _, err := rand.Read(token[:]); err != nil {
		return SupportInvoice{}, err
	}
	now := s.clock.Now()
	invoice, err := s.repo.CreateInvoice(ctx, SupportInvoice{UserID: userID, TelegramID: telegramID, Stars: stars, Payload: "support:" + hex.EncodeToString(token[:]), RequestKey: requestKey, TermsVersion: SupportTermsVersion, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	if err != nil {
		return SupportInvoice{}, err
	}
	if invoice.UserID != userID || invoice.TelegramID != telegramID || invoice.Stars != stars || invoice.TermsVersion != SupportTermsVersion {
		return SupportInvoice{}, errors.New("invoice request key mismatch")
	}
	return invoice, nil
}
func (s *PaymentService) ApproveCheckout(ctx context.Context, queryID string, receipt PaymentReceipt) (bool, error) {
	if !s.Enabled() || queryID == "" || receipt.Currency != "XTR" || !ValidSupportAmount(receipt.Stars) {
		return false, nil
	}
	return s.repo.ApproveCheckout(ctx, queryID, receipt, s.clock.Now())
}

// Receipt processing stays enabled even when issuing new invoices is disabled.
func (s *PaymentService) RecordReceipt(ctx context.Context, receipt PaymentReceipt) error {
	if s == nil || s.repo == nil {
		return errors.New("payment repository unavailable")
	}
	return s.repo.RecordReceipt(ctx, receipt, s.clock.Now())
}
func (s *PaymentService) DeliverThankYou(ctx context.Context, send func(context.Context, int64, string) error) error {
	p, err := s.repo.ClaimThankYou(ctx, s.clock.Now())
	if err != nil || p == nil {
		return err
	}
	if err = send(ctx, p.TelegramID, "🐈 Кот внимательно изучил финансовую отчётность.\n\n❤️ Спасибо за поддержку CatForge!"); err != nil {
		return err
	}
	return s.repo.CompleteThankYou(ctx, p.ID, s.clock.Now())
}

// Operator action: always resolve the recipient and charge from the ledger.
// On an ambiguous API result keep the row paid; reconcile with Telegram before retrying.
func (s *PaymentService) Refund(ctx context.Context, id int64, refund func(context.Context, int64, string) error) error {
	p, err := s.repo.GetPayment(ctx, id)
	if err != nil {
		return err
	}
	if p.Status == "refunded" {
		return nil
	}
	claimed, err := s.repo.BeginRefund(ctx, id, s.clock.Now())
	if err != nil {
		return err
	}
	if !claimed {
		return errors.New("refund already requested; reconcile Telegram transactions")
	}
	if err = refund(ctx, p.TelegramID, p.ChargeID); err != nil {
		return err
	}
	return s.repo.MarkRefunded(ctx, id, s.clock.Now())
}

// ConfirmRefund only changes the ledger after verifying Telegram transaction history.
func (s *PaymentService) ConfirmRefund(ctx context.Context, id int64, verify func(context.Context, int64, string) (bool, error)) error {
	p, err := s.repo.GetPayment(ctx, id)
	if err != nil {
		return err
	}
	if p.Status == "refunded" {
		return nil
	}
	ok, err := verify(ctx, p.TelegramID, p.ChargeID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("refund not found in Telegram transactions")
	}
	return s.repo.MarkRefunded(ctx, id, s.clock.Now())
}
