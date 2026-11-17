package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSocketIsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.sock")
	listener, err := listenOwner(path, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("mode %o, want 600", mode)
	}
}

func TestTooLongSocketPathNamesTheLimit(t *testing.T) {
	_, err := listenOwner("/tmp/"+strings.Repeat("x", 200), slog.New(slog.DiscardHandler))
	if err == nil || !strings.Contains(err.Error(), "longer than") {
		t.Fatalf("got %v", err)
	}
}
