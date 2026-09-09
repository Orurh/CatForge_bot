// Package observability exports bounded operational labels only: never tokens,
// chat/user/cat IDs, prompts, message text, or arbitrary request paths.
package observability

import (
	"context"
	"path"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var (
	PaymentReview        = promauto.NewGauge(prometheus.GaugeOpts{Name: "catforge_payments_review_required", Help: "Payment receipts requiring operator review."})
	PaymentThanksAge     = promauto.NewGauge(prometheus.GaugeOpts{Name: "catforge_payment_thanks_oldest_age_seconds", Help: "Age of oldest undelivered payment thank-you, zero when none."})
	PaymentRefundPending = promauto.NewGauge(prometheus.GaugeOpts{Name: "catforge_payment_refunds_pending", Help: "Requested refunds not yet confirmed in ledger."})
	PaymentProbe         = promauto.NewGauge(prometheus.GaugeOpts{Name: "catforge_payment_probe_up", Help: "Whether ledger monitoring query succeeded."})
	Operations           = promauto.NewCounterVec(prometheus.CounterOpts{Name: "catforge_operations_total", Help: "Completed operations by component, operation and outcome."}, []string{"component", "operation", "outcome"})
	Duration             = promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "catforge_operation_duration_seconds", Help: "Operation latency including failed operations.", Buckets: []float64{.01, .05, .1, .25, .5, 1, 2, 5, 10, 30, 60}}, []string{"component", "operation"})
	LastSuccess          = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "catforge_last_success_timestamp_seconds", Help: "Last successful polling or worker cycle, including idle cycles."}, []string{"component", "operation"})
	Dependencies         = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "catforge_dependency_up", Help: "Last dependency probe result: 1 healthy, 0 unhealthy."}, []string{"dependency"})
	ProbeTime            = promauto.NewGauge(prometheus.GaugeOpts{Name: "catforge_dependency_probe_timestamp_seconds", Help: "Time of last completed dependency probe cycle."})
	BackupTime           = promauto.NewGauge(prometheus.GaugeOpts{Name: "catforge_backup_last_success_timestamp_seconds", Help: "Newest nonempty .dump file mtime in the configured backup directory, zero if absent."})
	BackupReadable       = promauto.NewGauge(prometheus.GaugeOpts{Name: "catforge_backup_directory_readable", Help: "Whether the backup directory can be inspected."})
	Pool                 = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "catforge_db_pool_connections", Help: "PostgreSQL client pool connections."}, []string{"state"})
	AIMode               = promauto.NewGauge(prometheus.GaugeOpts{Name: "catforge_ai_primary_enabled", Help: "1 when an external AI provider is configured."})
	Polling              = promauto.NewGauge(prometheus.GaugeOpts{Name: "catforge_telegram_polling_enabled", Help: "1 in polling mode, 0 in webhook mode."})
	Logs                 = promauto.NewCounterVec(prometheus.CounterOpts{Name: "catforge_log_messages_total", Help: "Application log records by level."}, []string{"level"})
)

func Observe(component, operation string, started time.Time, err error) {
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	Operations.WithLabelValues(component, operation, outcome).Inc()
	Duration.WithLabelValues(component, operation).Observe(time.Since(started).Seconds())
	if err == nil {
		LastSuccess.WithLabelValues(component, operation).SetToCurrentTime()
	}
}

func InitializeWorker(operation string) {
	LastSuccess.WithLabelValues("worker", operation).Set(0)
}

func UnaryClientInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	started := time.Now()
	err := invoker(ctx, method, req, reply, cc, opts...)
	operation := path.Base(method)
	Operations.WithLabelValues("engine", operation, status.Code(err).String()).Inc()
	Duration.WithLabelValues("engine", operation).Observe(time.Since(started).Seconds())
	return err
}
