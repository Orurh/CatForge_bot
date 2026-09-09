// payment-refund is an operator-only tool; it has no Telegram command handler.
package main

import (
	"catforge/internal/app"
	"catforge/internal/repo/postgres"
	tg "catforge/internal/transport/telegram"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	id := flag.Int64("payment-id", 0, "payment ledger ID")
	reconcile := flag.Bool("reconcile", false, "verify a prior refund in Telegram history and repair ledger")
	execute := flag.Bool("execute", false, "issue the refund; otherwise inspect only")
	flag.Parse()
	if *id <= 0 {
		log.Fatal("positive -payment-id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := postgres.NewPool(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("database connection failed")
	}
	defer pool.Close()
	repo := postgres.NewPaymentRepo(pool)
	p, err := repo.GetPayment(ctx, *id)
	if err != nil {
		log.Fatal("payment not found")
	}
	fmt.Printf("Payment %d: %d Stars, status %s\n", p.ID, p.Stars, p.Status)
	if !*execute && !*reconcile {
		fmt.Printf("Refund previously requested: %v\n", p.RefundRequested)
		fmt.Println("Inspect only. Add -execute after reviewing the support request.")
		return
	}
	if os.Getenv("TELEGRAM_TOKEN") == "" {
		log.Fatal("TELEGRAM_TOKEN required")
	}
	service := app.NewPaymentService(repo, app.SystemClock{}, "")
	sender := tg.NewSender(os.Getenv("TELEGRAM_TOKEN"), nil)
	if *reconcile {
		err = service.ConfirmRefund(ctx, *id, sender.VerifyStarRefund)
	} else {
		err = service.Refund(ctx, *id, sender.RefundStarPayment)
	}
	if err != nil {
		log.Fatal("refund not confirmed; inspect Telegram transactions before retrying")
	}
	fmt.Println("Refund confirmed and saved.")
}
