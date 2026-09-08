package ai

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"
)

// CodexProvider talks only to the local subscription bridge over a Unix socket.
// ChatGPT credentials stay on the host and never enter the bot container.
type CodexProvider struct{ *OpenAIProvider }

func (*CodexProvider) Name() string { return "codex" }

func NewCodexProvider(socket, model string) (*CodexProvider, error) {
	if !filepath.IsAbs(socket) {
		return nil, fmt.Errorf("CODEX_BRIDGE_SOCKET must be an absolute path")
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		}},
	}
	provider, err := NewOpenAIProvider("local-unix-socket", "http://localhost/v1", model, client)
	if err != nil {
		return nil, err
	}
	return &CodexProvider{provider}, nil
}
