// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package webui

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

const (
	indexBody = "<!doctype html><title>shell</title>"
	jsBody    = "console.log('hello hello hello hello hello hello hello hello');"
)

func testAssets() fstest.MapFS {
	return fstest.MapFS{
		"index.html":            {Data: []byte(indexBody)},
		"favicon.ico":           {Data: []byte("ico")},
		"assets/app-1a2b3c.js":  {Data: []byte(jsBody)},
		"assets/app-1a2b3c.css": {Data: []byte("body{}")},
		"assets/pre-9f.js":      {Data: []byte("raw")},
		"assets/pre-9f.js.gz":   {Data: []byte("GZ")},
		"assets/pre-9f.js.br":   {Data: []byte("BR")},
		"assets/font-1.woff2":   {Data: []byte("font")},
	}
}

func do(t *testing.T, method, target string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	Handler(testAssets()).ServeHTTP(rec, req)
	return rec
}

func TestSPAFallback(t *testing.T) {
	const immutable = "public, max-age=31536000, immutable"
	tests := []struct {
		name     string
		method   string
		target   string
		status   int
		body     string // exact body when non-empty
		cache    string
		ctypeHas string
		allow    string
	}{
		{name: "root", method: "GET", target: "/", status: 200, body: indexBody, cache: "no-cache", ctypeHas: "text/html"},
		{name: "deep route", method: "GET", target: "/settings/appearance", status: 200, body: indexBody, cache: "no-cache", ctypeHas: "text/html"},
		{name: "route with trailing slash", method: "GET", target: "/flows/abc/", status: 200, body: indexBody, cache: "no-cache"},
		{name: "path traversal is cleaned", method: "GET", target: "/a/../../settings", status: 200, body: indexBody, cache: "no-cache"},
		{name: "existing asset", method: "GET", target: "/assets/app-1a2b3c.js", status: 200, body: jsBody, cache: immutable, ctypeHas: "text/javascript"},
		{name: "existing css", method: "GET", target: "/assets/app-1a2b3c.css", status: 200, body: "body{}", cache: immutable, ctypeHas: "text/css"},
		{name: "root file", method: "GET", target: "/favicon.ico", status: 200, body: "ico", cache: "no-cache", ctypeHas: "image/x-icon"},
		{name: "missing asset", method: "GET", target: "/assets/missing-1.js", status: 404},
		{name: "missing assets dir", method: "GET", target: "/assets/", status: 404},
		{name: "missing file with extension", method: "GET", target: "/robots.txt", status: 404},
		{name: "api unknown", method: "GET", target: "/api/v1/x", status: 404, ctypeHas: "application/problem+json"},
		{name: "api root", method: "GET", target: "/api", status: 404},
		{name: "api post unknown", method: "POST", target: "/api/v1/x", status: 404, ctypeHas: "application/problem+json"},
		{name: "healthz reserved", method: "GET", target: "/healthz", status: 404},
		{name: "similar prefix is a route", method: "GET", target: "/apiary", status: 200, body: indexBody},
		{name: "head route", method: "HEAD", target: "/settings", status: 200, cache: "no-cache"},
		{name: "post route", method: "POST", target: "/settings", status: 405, allow: "GET, HEAD"},
		{name: "delete root", method: "DELETE", target: "/", status: 405, allow: "GET, HEAD"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, tc.method, tc.target, nil)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if tc.body != "" && rec.Body.String() != tc.body {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.body)
			}
			if tc.method == "HEAD" && rec.Body.Len() != 0 {
				t.Errorf("HEAD returned a body of %d bytes", rec.Body.Len())
			}
			if tc.cache != "" && rec.Header().Get("Cache-Control") != tc.cache {
				t.Errorf("Cache-Control = %q, want %q", rec.Header().Get("Cache-Control"), tc.cache)
			}
			if tc.ctypeHas != "" && !strings.Contains(rec.Header().Get("Content-Type"), tc.ctypeHas) {
				t.Errorf("Content-Type = %q, want it to contain %q", rec.Header().Get("Content-Type"), tc.ctypeHas)
			}
			if tc.allow != "" && rec.Header().Get("Allow") != tc.allow {
				t.Errorf("Allow = %q, want %q", rec.Header().Get("Allow"), tc.allow)
			}
			if tc.status == 404 && strings.Contains(rec.Body.String(), "<title>shell") {
				t.Error("a 404 served the application shell")
			}
		})
	}
}

func TestPrecompressedAssets(t *testing.T) {
	tests := []struct {
		name, accept, wantEnc, wantBody string
	}{
		{"brotli preferred", "gzip, br", "br", "BR"},
		{"gzip only", "gzip", "gzip", "GZ"},
		{"br refused", "br;q=0, gzip", "gzip", "GZ"},
		{"identity", "", "", "raw"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, "GET", "/assets/pre-9f.js", map[string]string{"Accept-Encoding": tc.accept})
			if rec.Code != 200 {
				t.Fatalf("status = %d", rec.Code)
			}
			if got := rec.Header().Get("Content-Encoding"); got != tc.wantEnc {
				t.Errorf("Content-Encoding = %q, want %q", got, tc.wantEnc)
			}
			if rec.Body.String() != tc.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/javascript") {
				t.Errorf("Content-Type = %q, want the type of the uncompressed file", got)
			}
			if got := rec.Header().Get("Cache-Control"); got != cacheImmutable {
				t.Errorf("Cache-Control = %q", got)
			}
			if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
				t.Error("missing Vary: Accept-Encoding")
			}
		})
	}
}

func TestOnTheFlyGzip(t *testing.T) {
	for target, want := range map[string]string{"/assets/app-1a2b3c.js": jsBody, "/settings": indexBody} {
		rec := do(t, "GET", target, map[string]string{"Accept-Encoding": "gzip"})
		if rec.Header().Get("Content-Encoding") != "gzip" {
			t.Fatalf("%s: Content-Encoding = %q, want gzip", target, rec.Header().Get("Content-Encoding"))
		}
		zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		got, err := io.ReadAll(zr)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		if string(got) != want {
			t.Errorf("%s: decompressed body = %q, want %q", target, got, want)
		}
	}
}

func TestNoCompressionForBinaryOrUnaccepted(t *testing.T) {
	rec := do(t, "GET", "/assets/font-1.woff2", map[string]string{"Accept-Encoding": "gzip, br"})
	if rec.Header().Get("Content-Encoding") != "" || rec.Body.String() != "font" {
		t.Errorf("font was compressed: enc=%q body=%q", rec.Header().Get("Content-Encoding"), rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "font/woff2" {
		t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	rec = do(t, "GET", "/assets/app-1a2b3c.js", nil)
	if rec.Header().Get("Content-Encoding") != "" || rec.Body.String() != jsBody {
		t.Error("identity request received a compressed body")
	}
}

func TestConditionalRequest(t *testing.T) {
	etag := do(t, "GET", "/assets/app-1a2b3c.js", nil).Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag")
	}
	rec := do(t, "GET", "/assets/app-1a2b3c.js", map[string]string{"If-None-Match": etag})
	if rec.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304", rec.Code)
	}
}

func TestMissingIndexIsNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	Handler(fstest.MapFS{}).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/settings", nil))
	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestEmbeddedAssetsServeIndex(t *testing.T) {
	assets, err := Assets()
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	Handler(assets).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/settings/appearance", nil))
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("status=%d cache=%q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}
