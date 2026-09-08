package main

import (
	"catforge/internal/app"
	"catforge/internal/logx"
	"catforge/internal/observability"
	tg "catforge/internal/transport/telegram"
	"context"
	"time"
)

func runPaymentThanks(ctx context.Context, service *app.PaymentService, sender *tg.Sender, logger logx.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attempt, cancel := context.WithTimeout(ctx, 15*time.Second)
			started := time.Now()
			err := service.DeliverThankYou(attempt, sender.Text)
			cancel()
			observability.Observe("payments", "thank_you", started, err)
			if err != nil {
				logger.Error("payment thank-you delivery failed; retry scheduled", logx.Any("err", err))
			}
		}
	}
}
