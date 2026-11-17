package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeStore map[string]string

func (s fakeStore) Get(_ context.Context, name string) (secret, error) {
	if name == "broken" {
		return secret{}, errors.New("secret manager is down")
	}
	value, ok := s[name]
	if !ok {
		return secret{}, errNotFound
	}
	return secret{value: []byte(value), version: 3}, nil
}

func get(t *testing.T, h http.Handler, method, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, "http://socket"+path, nil))
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	return rec.Code, string(body)
}

func testHandler(store fakeStore) (*handler, *bytes.Buffer) {
	var log bytes.Buffer
	return newHandler(store, slog.New(slog.NewJSONHandler(&log, nil))), &log
}

// logLines decodes the JSON log, one object per line
func logLines(t *testing.T, log *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(log.String()), "\n") {
		var fields map[string]any
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			t.Fatalf("%q is not JSON: %v", line, err)
		}
		lines = append(lines, fields)
	}
	return lines
}

func TestValueIsServedByteForByte(t *testing.T) {
	value := "line1\nline2\n\x00\xff"
	h, _ := testHandler(fakeStore{"web/token": value})
	code, body := get(t, h, http.MethodGet, "/web/token")
	if code != http.StatusOK || body != value {
		t.Fatalf("got %d %q, want 200 %q", code, body, value)
	}
}

func TestNameOutsideTheContractIsRejected(t *testing.T) {
	h, _ := testHandler(fakeStore{"a.b": "v", "": "v"})
	for _, path := range []string{"/", "/a.b", "//a", "/a/", "/a//b", "/../a", "/a%2F..%2Fb"} {
		if code, _ := get(t, h, http.MethodGet, path); code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", path, code)
		}
	}
}

func TestOnlyGetIsServed(t *testing.T) {
	h, _ := testHandler(fakeStore{"web/token": "v"})
	if code, _ := get(t, h, http.MethodPost, "/web/token"); code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", code)
	}
}

func TestEveryRequestIsLoggedWithoutTheValue(t *testing.T) {
	h, log := testHandler(fakeStore{"web/token": "s3cr3t-value"})
	get(t, h, http.MethodGet, "/web/token")
	get(t, h, http.MethodGet, "/web/missing")
	get(t, h, http.MethodGet, "/a.b")
	get(t, h, http.MethodPost, "/web/token")

	want := []struct {
		name   string
		status float64
	}{{"web/token", 200}, {"web/missing", 404}, {"a.b", 400}, {"web/token", 405}}
	lines := logLines(t, log)
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(want), log)
	}
	for i, w := range want {
		if lines[i]["msg"] != "request" || lines[i]["name"] != w.name || lines[i]["status"] != w.status {
			t.Errorf("line %d: got %v, want request %s %v", i, lines[i], w.name, w.status)
		}
	}
	if lines[0]["version"] != float64(3) {
		t.Errorf("served line has no version: %v", lines[0])
	}
	if strings.Contains(log.String(), "s3cr3t-value") {
		t.Errorf("the value is in the log:\n%s", log)
	}
}

func TestStoreFailureIsLoggedWithItsCause(t *testing.T) {
	h, log := testHandler(fakeStore{})
	if code, _ := get(t, h, http.MethodGet, "/broken"); code != http.StatusBadGateway {
		t.Fatalf("got %d, want 502", code)
	}
	line := logLines(t, log)[0]
	if line["level"] != "ERROR" || line["status"] != float64(502) || line["error"] != "secret manager is down" {
		t.Fatalf("got %v", line)
	}
}
