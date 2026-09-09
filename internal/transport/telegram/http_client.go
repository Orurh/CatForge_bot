package telegram

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// newTelegramHTTPClient forces IPv4 because some Docker hosts publish an AAAA
// record for api.telegram.org while having no usable IPv6 route.
func newTelegramHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, _ string, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp4", address)
		},
		ForceAttemptHTTP2: true,
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}

func telegramRequestError(action string, err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("telegram %s request: %w", action, urlErr.Err)
	}
	return fmt.Errorf("telegram %s request failed", action)
}
