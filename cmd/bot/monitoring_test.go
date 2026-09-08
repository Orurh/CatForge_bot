package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewestBackup(t *testing.T) {
	dir := t.TempDir()
	if got, ok := newestBackup(dir); !ok || got != 0 {
		t.Fatal(got, ok)
	}
	if _, ok := newestBackup(filepath.Join(dir, "missing")); ok {
		t.Fatal("missing directory reported readable")
	}
	for _, name := range []string{"old.dump", "new.dump", "partial.dump.tmp", "empty.dump"} {
		data := []byte("archive")
		if name == "empty.dump" {
			data = nil
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Unix(100, 0)
	latest := time.Unix(200, 0)
	if err := os.Chtimes(filepath.Join(dir, "old.dump"), old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(dir, "new.dump"), latest, latest); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "partial.dump.tmp"), filepath.Join(dir, "link.dump")); err != nil {
		t.Fatal(err)
	}
	if got, ok := newestBackup(dir); !ok || got != 200 {
		t.Fatal(got, ok)
	}
}
