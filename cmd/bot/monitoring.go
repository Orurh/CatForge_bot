package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"catforge/internal/observability"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newestBackup(directory string) (float64, bool) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0, false
	}
	var newest int64
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".dump") {
			continue
		}
		info, err := os.Stat(filepath.Join(directory, entry.Name()))
		if err != nil {
			return 0, false
		}
		if info.Mode().IsRegular() && info.Size() > 0 && info.ModTime().Unix() > newest {
			newest = info.ModTime().Unix()
		}
	}
	return float64(newest), true
}

func runDependencyMonitor(ctx context.Context, pool *pgxpool.Pool, engineCheck func(context.Context) error, backupDir string) {
	lastLog := time.Time{}
	check := func() {
		for _, dependency := range []struct {
			name  string
			probe func(context.Context) error
		}{{"postgres", pool.Ping}, {"engine", engineCheck}} {
			probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err := dependency.probe(probeCtx)
			cancel()
			value := 0.0
			if err == nil {
				value = 1
			}
			observability.Dependencies.WithLabelValues(dependency.name).Set(value)
		}
		observability.ProbeTime.SetToCurrentTime()
		if time.Since(lastLog) >= 5*time.Minute {
			slog.Info("monitoring heartbeat", "component", "monitoring")
			lastLog = time.Now()
		}
		paymentCtx, cancelPayment := context.WithTimeout(ctx, 2*time.Second)
		var review, refunds float64
		var thanksAge float64
		paymentErr := pool.QueryRow(paymentCtx, `SELECT count(*) FILTER(WHERE review_reason<>''),
          count(*) FILTER(WHERE refund_requested_at IS NOT NULL AND status<>'refunded'),
          COALESCE(EXTRACT(EPOCH FROM now()-min(created_at) FILTER(WHERE status='paid' AND review_reason='' AND thank_you_sent_at IS NULL)),0) FROM payments`).Scan(&review, &refunds, &thanksAge)
		cancelPayment()
		if paymentErr == nil {
			observability.PaymentProbe.Set(1)
			observability.PaymentReview.Set(review)
			observability.PaymentRefundPending.Set(refunds)
			observability.PaymentThanksAge.Set(thanksAge)
		} else {
			observability.PaymentProbe.Set(0)
		}
		stat := pool.Stat()
		observability.Pool.WithLabelValues("acquired").Set(float64(stat.AcquiredConns()))
		observability.Pool.WithLabelValues("idle").Set(float64(stat.IdleConns()))
		observability.Pool.WithLabelValues("max").Set(float64(stat.MaxConns()))
		timestamp, readable := newestBackup(backupDir)
		observability.BackupTime.Set(timestamp)
		value := 0.0
		if readable {
			value = 1
		}
		observability.BackupReadable.Set(value)
	}
	check()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}
