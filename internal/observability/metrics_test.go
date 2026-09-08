package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func value(m prometheus.Metric) float64 {
	var out dto.Metric
	_ = m.Write(&out)
	if out.Counter != nil {
		return out.Counter.GetValue()
	}
	return out.Gauge.GetValue()
}
func TestObserveFailureDoesNotAdvanceHeartbeat(t *testing.T) {
	LastSuccess.WithLabelValues("worker", "test").Set(123)
	before := value(Operations.WithLabelValues("worker", "test", "error"))
	Observe("worker", "test", time.Now(), errors.New("test"))
	if value(LastSuccess.WithLabelValues("worker", "test")) != 123 {
		t.Fatal("failure advanced heartbeat")
	}
	if value(Operations.WithLabelValues("worker", "test", "error")) != before+1 {
		t.Fatal("missing failure")
	}
	Observe("worker", "test", time.Now(), nil)
	if value(LastSuccess.WithLabelValues("worker", "test")) <= 123 {
		t.Fatal("success did not advance heartbeat")
	}
}
func TestRPCInterceptorPreservesError(t *testing.T) {
	want := status.Error(codes.Unavailable, "engine down")
	before := value(Operations.WithLabelValues("engine", "Train", "Unavailable"))
	got := UnaryClientInterceptor(context.Background(), "/catforge.GameEngine/Train", nil, nil, nil, func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error { return want })
	if got != want || value(Operations.WithLabelValues("engine", "Train", "Unavailable")) != before+1 {
		t.Fatal("RPC result/metric mismatch")
	}
}
