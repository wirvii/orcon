// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package conditional_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/conditional"
	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
)

func request(t *testing.T, ifMatch string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/flows/1/draft", nil)
	if ifMatch != "" {
		r.Header.Set("If-Match", ifMatch)
	}
	return r
}

func TestETagRoundTrip(t *testing.T) {
	for _, n := range []int64{0, 1, 7, 1 << 40} {
		got, err := conditional.Parse(conditional.ETag(n))
		if err != nil || got != n {
			t.Errorf("round trip of %d = %d, %v", n, got, err)
		}
	}
	if conditional.ETag(7) != `"7"` {
		t.Errorf("ETag(7) = %s", conditional.ETag(7))
	}
}

func TestParseRejectsWhatIsNotAStrongTag(t *testing.T) {
	for _, tag := range []string{`7`, `W/"7"`, `"abc"`, `""`, `"-1"`, `*`, `"7`, `'7'`} {
		if _, err := conditional.Parse(tag); !errors.Is(err, conditional.ErrMalformed) {
			t.Errorf("Parse(%s) err = %v, want ErrMalformed", tag, err)
		}
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		ifMatch string
		want    error
	}{
		{"current", `"7"`, nil},
		{"current with spaces", ` "7" `, nil},
		{"missing", ``, conditional.ErrRequired},
		{"old", `"6"`, conditional.ErrMismatch},
		{"new", `"8"`, conditional.ErrMismatch},
		{"wildcard", `*`, conditional.ErrMismatch},
		{"weak", `W/"7"`, conditional.ErrMismatch},
	}
	for _, tt := range tests {
		if err := conditional.Check(request(t, tt.ifMatch), 7); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestWrite(t *testing.T) {
	revision := 8
	rec := httptest.NewRecorder()
	if conditional.Write(rec, request(t, ""), conditional.Check(request(t, ""), 8), 8) != true || rec.Code != http.StatusPreconditionRequired {
		t.Errorf("missing If-Match = %d, want 428", rec.Code)
	}
	rec = httptest.NewRecorder()
	r := request(t, `"6"`)
	failed := conditional.Write(rec, r, conditional.Check(r, 8), 8, problem.CurrentDraft(revision, "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a61"))
	if !failed || rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match = %d, want 412", rec.Code)
	}
	if rec.Header().Get("ETag") != `"8"` {
		t.Errorf("ETag = %q, want the current one", rec.Header().Get("ETag"))
	}
	var doc problem.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("body: %v", err)
	}
	if doc.CurrentDraftRevision == nil || *doc.CurrentDraftRevision != 8 || doc.UpdatedBy == nil || *doc.UpdatedBy == "" {
		t.Errorf("the 412 of a draft lacks currentDraftRevision and updatedBy: %+v", doc)
	}
	rec = httptest.NewRecorder()
	if conditional.Write(rec, request(t, `"8"`), nil, 8) || rec.Body.Len() != 0 {
		t.Error("Write answered a check that passed")
	}
}
