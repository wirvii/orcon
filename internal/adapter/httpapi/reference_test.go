// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/wirvii/orcon/internal/adapter/httpapi/reference"
	"github.com/wirvii/orcon/internal/platform/config"
)

// siteFS stands for the site the build embeds.
func siteFS() fs.FS {
	return fstest.MapFS{
		"index.html":           {Data: []byte(`<!doctype html><html><head><script src="reference-config.js"></script><script src="scalar.standalone.js"></script></head><body></body></html>`)},
		"reference-config.js":  {Data: []byte(`window.__ORCON_REFERENCE__ = { url: "openapi.json" };`)},
		"scalar.standalone.js": {Data: []byte(`console.log("scalar");`)},
		"openapi.json":         {Data: []byte(`{"openapi":"3.1.0"}`)},
		"LICENSE-scalar.txt":   {Data: []byte("MIT License")},
		"docs/guide.html":      {Data: []byte("<p>guide</p>")},
		"docs/index.html":      {Data: []byte("<p>docs index</p>")},
	}
}

func apiWithSite(t *testing.T, role config.Role) *Router {
	t.Helper()
	rt, err := New(&Deps{Role: role, Reference: siteFS()})
	if err != nil {
		t.Fatalf("New(%s): %v", role, err)
	}
	return rt
}

func get(t *testing.T, rt *Router, method, target string, mut ...func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, nil)
	req.RemoteAddr = "192.0.2.10:4000"
	for _, m := range mut {
		m(req)
	}
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, req)
	return rec
}

func TestAPIReferenceServedInApiAndStandalone(t *testing.T) {
	for _, role := range []config.Role{config.RoleAPI, config.RoleStandalone} {
		rt := apiWithSite(t, role)
		for _, target := range []string{"/api/reference/", "/api/reference/openapi.json", "/api/reference/scalar.standalone.js", "/api/reference/docs/guide.html", "/api/reference/docs/"} {
			rec := get(t, rt, http.MethodGet, target)
			if rec.Code != http.StatusOK {
				t.Errorf("%s GET %s = %d, want 200", role, target, rec.Code)
			}
			// The site is readable without credentials, like the OpenAPI document.
			if rec.Header().Get("Orcon-Request-Id") == "" {
				t.Errorf("%s GET %s has no request id", role, target)
			}
		}
		if rec := get(t, rt, http.MethodHead, "/api/reference/"); rec.Code != http.StatusOK || rec.Body.Len() != 0 {
			t.Errorf("%s HEAD /api/reference/ = %d with %d body bytes, want 200 and none", role, rec.Code, rec.Body.Len())
		}
		if rec := get(t, rt, http.MethodGet, "/api/reference/missing.js"); rec.Code != http.StatusNotFound {
			t.Errorf("%s a missing file = %d, want 404", role, rec.Code)
		}
		if rec := get(t, rt, http.MethodPost, "/api/reference/"); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s POST to the site = %d, want 405", role, rec.Code)
		}
	}
	for _, role := range []config.Role{config.RoleScheduler, config.RoleWorker} {
		if _, err := New(&Deps{Role: role, Reference: siteFS()}); err == nil {
			t.Errorf("the role %s serves the reference site", role)
		}
	}
}

func TestAPIReferenceRedirect(t *testing.T) {
	rt := apiWithSite(t, config.RoleAPI)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := get(t, rt, method, "/api/reference?tag=flows&operation=getFlow")
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("%s /api/reference = %d, want 301", method, rec.Code)
		}
		if got, want := rec.Header().Get("Location"), "/api/reference/?tag=flows&operation=getFlow"; got != want {
			t.Errorf("%s Location = %q, want %q", method, got, want)
		}
	}
	rec := get(t, rt, http.MethodGet, "/api/reference")
	if got := rec.Header().Get("Location"); got != "/api/reference/" {
		t.Errorf("Location without a query = %q, want /api/reference/", got)
	}
}

func TestAPIReferenceCSPAndNoExternalResources(t *testing.T) {
	const wantCSP = "default-src 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
		"base-uri 'self'; form-action 'none'; frame-ancestors 'none'; object-src 'none'"
	rt := apiWithSite(t, config.RoleAPI)
	for _, target := range []string{"/api/reference/", "/api/reference/openapi.json", "/api/reference/missing", "/api/reference"} {
		rec := get(t, rt, http.MethodGet, target)
		if got := rec.Header().Get("Content-Security-Policy"); got != wantCSP {
			t.Errorf("GET %s CSP = %q, want %q", target, got, wantCSP)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("GET %s lacks X-Content-Type-Options: nosniff", target)
		}
	}

	// The embedded site (the placeholder, or the real site after make build)
	// must not name an external origin in its markup or its configuration.
	embedded, err := reference.Site()
	if err != nil {
		t.Fatalf("Site: %v", err)
	}
	external := regexp.MustCompile(`(?i)(src|href|action|url)\s*[=:]\s*["']?(https?:)?//`)
	err = fs.WalkDir(embedded, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(name, ".html") && name != "reference-config.js" {
			return nil
		}
		data, readErr := fs.ReadFile(embedded, name)
		if readErr != nil {
			return readErr
		}
		if external.Match(data) {
			t.Errorf("%s references an external resource", name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

func TestAPIReferenceRevalidates(t *testing.T) {
	rt := apiWithSite(t, config.RoleAPI)
	first := get(t, rt, http.MethodGet, "/api/reference/openapi.json")
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}
	again := get(t, rt, http.MethodGet, "/api/reference/openapi.json", func(r *http.Request) { r.Header.Set("If-None-Match", etag) })
	if again.Code != http.StatusNotModified {
		t.Errorf("revalidation = %d, want 304", again.Code)
	}
	if ct := first.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	js := get(t, rt, http.MethodGet, "/api/reference/scalar.standalone.js")
	if ct := js.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("script Content-Type = %q", ct)
	}
}

func TestEmbeddedPlaceholderIsServed(t *testing.T) {
	rt, err := New(&Deps{Role: config.RoleAPI})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec := get(t, rt, http.MethodGet, "/api/reference/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Orcon") {
		t.Errorf("embedded index = %d %q", rec.Code, rec.Body.String())
	}
}
