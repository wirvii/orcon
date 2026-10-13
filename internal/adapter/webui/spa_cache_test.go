// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package webui

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestAcceptsEncodingHeaders(t *testing.T) {
	tests := []struct {
		name   string
		lines  []string
		coding string
		want   bool
	}{
		{"explicit", []string{"gzip"}, "gzip", true},
		{"case insensitive", []string{"GZip"}, "gzip", true},
		{"absent", []string{"deflate"}, "gzip", false},
		{"empty", nil, "gzip", false},
		{"refused", []string{"gzip;q=0"}, "gzip", false},
		{"refused spaced decimal", []string{"gzip; q=0.000"}, "gzip", false},
		{"low but allowed", []string{"gzip;q=0.1"}, "gzip", true},
		{"wildcard", []string{"*"}, "br", true},
		{"wildcard refused", []string{"*;q=0"}, "br", false},
		{"explicit refusal beats wildcard", []string{"br;q=0, *"}, "br", false},
		{"explicit allow beats wildcard refusal", []string{"*;q=0, gzip"}, "gzip", true},
		{"second header line", []string{"deflate", "gzip, br"}, "br", true},
		{"wildcard on later line", []string{"deflate", "*"}, "gzip", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := accepts(tc.lines, tc.coding); got != tc.want {
				t.Errorf("accepts(%q, %q) = %v, want %v", tc.lines, tc.coding, got, tc.want)
			}
		})
	}
}

func TestWildcardAndRepeatedHeaderLines(t *testing.T) {
	h := Handler(testAssets())

	req := httptest.NewRequestWithContext(t.Context(), "GET", "/assets/pre-9f.js", nil)
	req.Header.Add("Accept-Encoding", "deflate")
	req.Header.Add("Accept-Encoding", "br")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "br" || rec.Body.String() != "BR" {
		t.Errorf("repeated lines: enc=%q body=%q", rec.Header().Get("Content-Encoding"), rec.Body.String())
	}

	req = httptest.NewRequestWithContext(t.Context(), "GET", "/assets/app-1a2b3c.js", nil)
	req.Header.Set("Accept-Encoding", "*")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("wildcard: enc=%q", rec.Header().Get("Content-Encoding"))
	}
}

func TestAPINotFoundIsNosniff(t *testing.T) {
	rec := do(t, "GET", "/api/v1/x", nil)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", rec.Header().Get("X-Content-Type-Options"))
	}
}

func TestVariantsAreCachedPerEncoding(t *testing.T) {
	s, ok := Handler(testAssets()).(*spa)
	if !ok {
		t.Fatal("Handler does not return *spa")
	}
	etags := map[string]string{}
	for _, enc := range []string{"", "gzip", "br"} {
		name := "assets/pre-9f.js"
		key := variantKey{name: name, encoding: enc}
		first, ok := s.lookup(key)
		if !ok {
			t.Fatalf("lookup %q failed", enc)
		}
		second, _ := s.lookup(key)
		if first != second {
			t.Errorf("encoding %q: variant rebuilt on the second lookup", enc)
		}
		if etags[first.etag] != "" {
			t.Errorf("encoding %q shares an ETag with %q", enc, etags[first.etag])
		}
		etags[first.etag] = enc
	}
}

func TestParallelRequestsShareGzipCache(t *testing.T) {
	h := Handler(testAssets())
	const workers = 32
	var wg sync.WaitGroup
	etags := make(chan string, workers)
	errs := make(chan string, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequestWithContext(t.Context(), "GET", "/assets/app-1a2b3c.js", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
			if err != nil {
				errs <- err.Error()
				return
			}
			got, err := io.ReadAll(zr)
			if err != nil || string(got) != jsBody {
				errs <- "bad body"
				return
			}
			etags <- rec.Header().Get("ETag")
		}()
	}
	wg.Wait()
	close(etags)
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	seen := map[string]bool{}
	for e := range etags {
		seen[e] = true
	}
	if len(seen) != 1 {
		t.Errorf("parallel requests saw %d distinct ETags, want 1", len(seen))
	}
}
