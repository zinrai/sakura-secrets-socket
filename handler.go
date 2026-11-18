package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

// namePattern is credstore-delivery's contract for names, checked before
// asking Secret Manager, not left to it: isNotFound can then take every 422
// that Unveil answers with for a missing name.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(/[A-Za-z0-9_-]+)*$`)

var errNotFound = errors.New("not found")

type secret struct {
	value   []byte
	version int
}

type store interface {
	Get(ctx context.Context, name string) (secret, error)
}

type handler struct {
	store  store
	logger *slog.Logger
}

func newHandler(s store, logger *slog.Logger) *handler {
	return &handler{store: s, logger: logger}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	status, s, err := h.respond(r, name)
	h.logRequest(name, status, s, err)
	if status != http.StatusOK {
		http.Error(w, http.StatusText(status), status)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(s.value)
}

func (h *handler) respond(r *http.Request, name string) (int, secret, error) {
	if r.Method != http.MethodGet {
		return http.StatusMethodNotAllowed, secret{}, nil
	}
	if !namePattern.MatchString(name) {
		return http.StatusBadRequest, secret{}, nil
	}
	s, err := h.store.Get(r.Context(), name)
	if errors.Is(err, errNotFound) {
		return http.StatusNotFound, secret{}, nil
	}
	if err != nil {
		return http.StatusBadGateway, secret{}, err
	}
	return http.StatusOK, s, nil
}

// logRequest writes a line for every request, not only the served ones: a
// refused name tells as much about who uses the socket as a served one.
// The error goes on the same line, not a separate one, so that a request
// stays one line.
func (h *handler) logRequest(name string, status int, s secret, err error) {
	attrs := []any{"name", name, "status", status}
	switch {
	case status == http.StatusOK:
		h.logger.Info("request", append(attrs, "version", s.version)...)
	case err != nil:
		h.logger.Error("request", append(attrs, "error", err.Error())...)
	default:
		h.logger.Info("request", attrs...)
	}
}
