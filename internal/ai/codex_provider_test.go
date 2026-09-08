package ai

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
)

func TestCodexProviderUsesUnixSocket(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "bridge.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-5.6-luna","output":[{"type":"message","content":[{"type":"output_text","text":"мяу"}]}],"usage":{"input_tokens":6,"output_tokens":2}}`))
	})}
	go server.Serve(listener)
	t.Cleanup(func() { _ = server.Close() })
	p, err := NewCodexProvider(socket, "gpt-5.6-luna")
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.Generate(context.Background(), Prompt{System: "cat", User: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "codex" || r.Text != "мяу" || r.TokensIn != 6 {
		t.Fatalf("unexpected result: %+v", r)
	}
}
