// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/platform/config"
)

func TestMountAPIServesTheReferenceAndRefusesUnknownPaths(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	for _, role := range []config.Role{config.RoleAPI, config.RoleStandalone} {
		mux := http.NewServeMux()
		if err := mountAPI(mux, role, logger); err != nil {
			t.Fatalf("%s: mountAPI: %v", role, err)
		}
		for target, want := range map[string]int{
			"/api/reference/":     http.StatusOK,
			"/api/reference":      http.StatusMovedPermanently,
			"/api/v1/anything":    http.StatusNotFound,
			"/api/reference/nope": http.StatusNotFound,
		} {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
			req.RemoteAddr = "192.0.2.1:1"
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != want {
				t.Errorf("%s GET %s = %d, want %d", role, target, rec.Code, want)
			}
		}
	}
}

func TestMountAPIRefusesRolesThatDoNotServeIt(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	for _, role := range []config.Role{config.RoleScheduler, config.RoleWorker} {
		err := mountAPI(http.NewServeMux(), role, logger)
		if err == nil || !strings.Contains(err.Error(), "does not serve the API") {
			t.Errorf("%s: err = %v", role, err)
		}
	}
}

func TestServeHandlerMountsTheAPIBeforeTheWebUI(t *testing.T) {
	h, err := newHandler(config.RoleStandalone, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newHandler: %v", err)
	}
	get := func(target string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
		req.RemoteAddr = "192.0.2.1:1"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := get("/api/reference/"); rec.Code != http.StatusOK {
		t.Errorf("GET /api/reference/ = %d, want 200", rec.Code)
	}
	rec := get("/api/v1/unknown")
	if rec.Code != http.StatusNotFound || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/problem+json") {
		t.Errorf("GET /api/v1/unknown = %d %q, want 404 problem+json", rec.Code, rec.Header().Get("Content-Type"))
	}
}
