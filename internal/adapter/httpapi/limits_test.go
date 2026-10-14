// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

// TestBodyLimitsDocumented: every operation with a request body ends its
// description with the sentence the table dictates (API-5.2, Amendment 8 8-5).
func TestBodyLimitsDocumented(t *testing.T) {
	if bad := bodyLimitProblems(newReal(t).Spec()); len(bad) > 0 {
		t.Errorf("the contract and bodylimits.go differ: %v", bad)
	}
	spec, err := ParseSpec([]byte(syntheticSpec))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if bad := bodyLimitProblems(spec); len(bad) != 0 {
		t.Errorf("synthetic contract: %v", bad)
	}
	wrong := strings.Replace(syntheticSpec, "Request body limit: 8 MiB.", "Request body limit: 1 MiB.", 1)
	spec, err = ParseSpec([]byte(wrong))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if bad := bodyLimitProblems(spec); len(bad) != 1 || !strings.Contains(bad[0], "replaceDraft") {
		t.Errorf("a wrong limit in a description was not reported: %v", bad)
	}
	missing := strings.Replace(syntheticSpec, " Request body limit: 1 MiB.", "", 1)
	spec, _ = ParseSpec([]byte(missing))
	if bad := bodyLimitProblems(spec); len(bad) != 1 || !strings.Contains(bad[0], "createThing") {
		t.Errorf("a missing sentence was not reported: %v", bad)
	}
}

func TestBodyLimitTable(t *testing.T) {
	for id, want := range map[string]int64{
		"getThing": 1 << 20, "createThing": 1 << 20, "replaceDraft": 8 << 20, "updateDraft": 8 << 20,
		"evaluateN8nConverter": 2 << 20, "createPin": 2 << 20, "replacePin": 2 << 20, "unknown": 1 << 20,
	} {
		if got := BodyLimitFor(id); got != want {
			t.Errorf("BodyLimitFor(%s) = %d, want %d", id, got, want)
		}
	}
}

// TestBodyLimitPerOperation: the limit of the operation applies, before
// authentication, and the default applies to the others.
func TestBodyLimitPerOperation(t *testing.T) {
	h := newHarness(t, nil)
	withLength := func(n int64) func(*http.Request) {
		return func(r *http.Request) {
			r.ContentLength = n
			r.Header.Set("If-Match", `"1"`)
		}
	}
	if rec := h.do(http.MethodPut, "/api/v1/drafts", "token-a", withLength(2<<20)); rec.Code != http.StatusOK {
		t.Errorf("2 MiB to a draft = %d, want 200", rec.Code)
	}
	if rec := h.do(http.MethodPut, "/api/v1/drafts", "token-a", withLength(9<<20)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("9 MiB to a draft = %d, want 413", rec.Code)
	}
	if rec := h.do(http.MethodPost, "/api/v1/things", "token-a", withLength(2<<20)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("2 MiB to an operation with the default limit = %d, want 413", rec.Code)
	}
	if rec := h.do(http.MethodPut, "/api/v1/drafts", "", withLength(9<<20)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("an oversize body without credentials = %d, want 413 before authentication", rec.Code)
	}
}

func TestMethodNotAllowedListsTheAllowedMethods(t *testing.T) {
	h := newHarness(t, nil)
	rec := h.do(http.MethodDelete, "/api/v1/things/1", "token-a")
	if rec.Code != http.StatusMethodNotAllowed || problemOf(t, rec)["code"] != "method-not-allowed" {
		t.Fatalf("DELETE = %d %s", rec.Code, rec.Body.String())
	}
	allow := rec.Header().Get("Allow")
	for _, m := range []string{"GET", "HEAD", "PUT"} {
		if !strings.Contains(allow, m) {
			t.Errorf("Allow = %q, missing %s", allow, m)
		}
	}
	if strings.Contains(allow, "DELETE") {
		t.Errorf("Allow = %q lists the refused method", allow)
	}
	nowhere := h.do(http.MethodGet, "/api/v1/nowhere", "token-a")
	if nowhere.Code != http.StatusNotFound || nowhere.Header().Get("Allow") != "" {
		t.Errorf("an unknown path = %d with Allow %q, want 404 and none", nowhere.Code, nowhere.Header().Get("Allow"))
	}
	// A refused method is answered even without credentials: it names no resource.
	if rec := h.do(http.MethodDelete, "/api/v1/things/1", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("unauthenticated DELETE = %d, want 405", rec.Code)
	}
}
