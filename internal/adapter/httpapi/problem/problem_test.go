// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package problem_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
)

func TestCatalogIsWellFormed(t *testing.T) {
	kebab := regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	seen := map[problem.Code]bool{}
	for _, s := range problem.Catalog() {
		if seen[s.Code] {
			t.Errorf("duplicate code %s", s.Code)
		}
		seen[s.Code] = true
		if !kebab.MatchString(string(s.Code)) {
			t.Errorf("%s is not kebab-case", s.Code)
		}
		if s.Status < 400 || s.Status > 599 {
			t.Errorf("%s has status %d", s.Code, s.Status)
		}
		if s.Title == "" || s.When == "" {
			t.Errorf("%s needs a title and a condition", s.Code)
		}
		if got, ok := problem.Lookup(s.Code); !ok || got != s {
			t.Errorf("Lookup(%s) = %+v, %v", s.Code, got, ok)
		}
	}
	if len(seen) != 25 {
		t.Errorf("the catalog has %d codes, ERR-6, Amendment 8 and BL-090 give 25", len(seen))
	}
	if _, ok := problem.Lookup("no-such-code"); ok {
		t.Error("Lookup found a code that is not in the catalog")
	}
}

func TestCatalogIsACopy(t *testing.T) {
	c := problem.Catalog()
	c[0].Status = 999
	if problem.Catalog()[0].Status == 999 {
		t.Error("Catalog shares its table with the caller")
	}
}

func write(t *testing.T, method string, code problem.Code, opts ...problem.Option) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	rec.Header().Set(problem.RequestIDHeader, "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a62")
	req := httptest.NewRequestWithContext(t.Context(), method, "/api/v1/flows/42?secret=1", nil)
	problem.Write(rec, req, code, opts...)
	return rec
}

func TestWriteDocument(t *testing.T) {
	rec := write(t, http.MethodPost, problem.ValidationFailed,
		problem.Detail("Two fields are invalid."),
		problem.Fields(
			problem.FieldError{Pointer: "/name", Code: "too-long", Message: "The name is longer than 200 characters."},
			problem.FieldError{Pointer: "/note", Code: "required", Message: "The note is required."},
		))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != problem.MediaType {
		t.Errorf("Content-Type = %q", ct)
	}
	var doc problem.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("body: %v", err)
	}
	if doc.Type != "urn:orcon:problem:validation-failed" || doc.Code != problem.ValidationFailed || doc.Title != "Validation failed" {
		t.Errorf("document = %+v", doc)
	}
	if doc.RequestID != "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a62" {
		t.Errorf("requestId = %q, want the one of the response", doc.RequestID)
	}
	if doc.Instance != "/api/v1/flows/42" || strings.Contains(rec.Body.String(), "secret=1") {
		t.Errorf("instance = %q: it must be the path without the query", doc.Instance)
	}
	if len(doc.Errors) != 2 {
		t.Errorf("errors = %+v, want both fields in one response", doc.Errors)
	}
}

func TestWriteOmitsEmptyMembers(t *testing.T) {
	rec := write(t, http.MethodGet, problem.NotFound)
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body: %v", err)
	}
	for _, absent := range []string{"detail", "errors"} {
		if _, ok := raw[absent]; ok {
			t.Errorf("member %q should be omitted when empty", absent)
		}
	}
}

func TestWriteHeadHasNoBody(t *testing.T) {
	rec := write(t, http.MethodHead, problem.NotFound)
	if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
		t.Errorf("HEAD = %d with %d body bytes", rec.Code, rec.Body.Len())
	}
	if rec.Header().Get("Content-Length") == "" {
		t.Error("HEAD should still announce the length")
	}
}

func TestWriteUnknownCodeBecomesInternalError(t *testing.T) {
	rec := write(t, http.MethodGet, "made-up")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"internal-error"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestRetryAfterRoundsUp(t *testing.T) {
	for d, want := range map[time.Duration]string{
		time.Millisecond: "1", time.Second: "1", 1100 * time.Millisecond: "2", 30 * time.Second: "30", 0: "1",
	} {
		rec := write(t, http.MethodGet, problem.RateLimited, problem.RetryAfter(d))
		if got := rec.Header().Get("Retry-After"); got != want {
			t.Errorf("RetryAfter(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestHeaderOption(t *testing.T) {
	rec := write(t, http.MethodGet, problem.Unauthenticated, problem.Header("WWW-Authenticate", "Bearer"))
	if rec.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Error("the header option was dropped")
	}
}

func TestConfirmationMembers(t *testing.T) {
	rec := write(t, http.MethodPost, problem.ConfirmationRequired,
		problem.Confirmation("0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a99", time.Date(2026, 10, 14, 9, 30, 0, 0, time.UTC)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	var doc problem.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("body: %v", err)
	}
	if doc.ConfirmationID == nil || *doc.ConfirmationID != "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a99" || doc.ExpiresAt == nil || *doc.ExpiresAt != "2026-10-14T09:30:00Z" {
		t.Errorf("document = %+v", doc)
	}
	for code, status := range map[problem.Code]int{problem.MethodNotAllowed: 405, problem.FederatedTokenRejected: 400} {
		if s, ok := problem.Lookup(code); !ok || s.Status != status || s.Retryable {
			t.Errorf("%s = %+v, %v; want status %d, not retryable", code, s, ok, status)
		}
	}
}

func TestNewWithoutRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	p, header := problem.New(rec, nil, problem.Conflict)
	if p.Status != http.StatusConflict || p.Instance != "" || len(header) != 0 {
		t.Errorf("New = %+v, %v", p, header)
	}
}
