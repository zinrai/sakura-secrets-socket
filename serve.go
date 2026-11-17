package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	socket := flags.String("socket", "", "Path of the Unix socket to listen on (required)")
	flags.Parse(args)
	if *socket == "" {
		flags.Usage()
		return fmt.Errorf("-socket is required")
	}

	vaultID, err := loadVaultID()
	if err != nil {
		return err
	}
	store, err := newSakuraStore(vaultID)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	listener, err := listenOwner(*socket, logger)
	if err != nil {
		return err
	}
	return serve(listener, vaultID, newHandler(store, logger), logger)
}

func loadVaultID() (string, error) {
	if v := os.Getenv("SAKURA_SECRETS_ID"); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("required environment variable not set: SAKURA_SECRETS_ID")
}

func serve(listener *ownerListener, vaultID string, h http.Handler, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          log.New(httpErrorWriter{logger}, "", 0),
	}
	logger.Info("start", "socket", listener.Addr().String(), "vault", vaultID)
	errc := make(chan error, 1)
	go func() { errc <- server.Serve(listener) }()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		logger.Info("stop")
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

// httpErrorWriter turns net/http's own log lines into the JSON log, not
// leaving them in the log package's format among the JSON lines.
type httpErrorWriter struct {
	logger *slog.Logger
}

func (w httpErrorWriter) Write(p []byte) (int, error) {
	w.logger.Error("http", "error", strings.TrimSpace(string(p)))
	return len(p), nil
}
