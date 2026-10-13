// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close() //nolint:errcheck // test response
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEgressRequestToGrantedHost(t *testing.T) {
	t.Parallel()
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "hello %s %s", r.Host, r.Header.Get("User-Agent")) //nolint:gosec // test server echoing to a test client
	}))
	dialer := &serverDialer{target: hostPort(t, srv)}
	c := mustNew(t, Config{Resolver: fixed("api.example.test", publicIP), Dialer: dialer, UserAgent: "orcon-test"})
	policy := Policy{Domains: []string{"*.example.test"}, Schemes: []string{"http"}}

	resp, err := c.Do(t.Context(), policy, newReq(t, "http://api.example.test/x"))
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, resp); got != "hello api.example.test orcon-test" {
		t.Fatalf("body = %q", got)
	}
	if got := dialer.addresses(); len(got) != 1 || got[0] != publicIP+":80" {
		t.Fatalf("dialed %v, want the pinned %s:80", got, publicIP)
	}
}

func TestEgressRefusesUngrantedAndPrivateBeforeConnecting(t *testing.T) {
	// Not parallel: the subtests share one request counter checked afterwards.
	var hits atomic.Int32
	srv := newServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	dialer := &serverDialer{target: hostPort(t, srv)}
	resolver := newStubResolver(map[string][][]string{
		"internal.example.test": {{"10.1.2.3"}},
		"loop.example.test":     {{"127.0.0.1"}},
		"v6.example.test":       {{"::1"}},
		"mapped.example.test":   {{"::ffff:127.0.0.1"}},
		"meta.example.test":     {{"169.254.169.254"}},
		"mixed.example.test":    {{publicIP, "192.168.1.1"}},
		"ula.example.test":      {{"fd12::1"}},
		"cgnat.example.test":    {{"100.64.1.1"}},
		"ok.example.test":       {{publicIP}},
	})
	c := mustNew(t, Config{Resolver: resolver, Dialer: dialer})
	policy := Policy{Domains: []string{"*.example.test"}, Schemes: []string{"http", "https"}}

	tests := []struct {
		name, url string
		reason    Reason
	}{
		{"private resolution", "http://internal.example.test/", ReasonPrivateAddress},
		{"loopback resolution", "http://loop.example.test/", ReasonPrivateAddress},
		{"IPv6 loopback resolution", "http://v6.example.test/", ReasonPrivateAddress},
		{"IPv4-mapped loopback resolution", "http://mapped.example.test/", ReasonPrivateAddress},
		{"metadata resolution", "http://meta.example.test/latest", ReasonMetadataAddress},
		{"one private answer among public ones", "http://mixed.example.test/", ReasonPrivateAddress},
		{"unique local resolution", "http://ula.example.test/", ReasonPrivateAddress},
		{"shared address space resolution", "http://cgnat.example.test/", ReasonPrivateAddress},
		{"ungranted domain", "http://evil.test/", ReasonDomainNotGranted},
		{"literal loopback ungranted", "http://127.0.0.1:" + portOfServer(t, srv) + "/", ReasonDomainNotGranted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Do(t.Context(), policy, newReq(t, tt.url)) //nolint:bodyclose // closed by readAll
			wantDenied(t, err, tt.reason)
		})
	}
	if hits.Load() != 0 {
		t.Fatalf("the server received %d requests; refusals must happen before any request is sent", hits.Load())
	}
	if got := dialer.addresses(); len(got) != 0 {
		t.Fatalf("dialed %v for refused requests", got)
	}
}

func portOfServer(t *testing.T, s *httptest.Server) string {
	t.Helper()
	_, port, err := net.SplitHostPort(hostPort(t, s))
	if err != nil {
		t.Fatal(err)
	}
	return port
}

// A real socket: the default dialer connects to a loopback server only when
// the policy grants private networks, and its socket-level check backs up the
// resolution check.
func TestEgressDefaultDialerSocketCheck(t *testing.T) {
	t.Parallel()
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") }))
	port := portOfServer(t, srv)
	c := mustNew(t, Config{Resolver: fixed("svc.example.test", "127.0.0.1")})

	strict := Policy{Domains: []string{"svc.example.test"}, Schemes: []string{"http"}}
	_, err := c.Do(t.Context(), strict, newReq(t, "http://svc.example.test:"+port+"/")) //nolint:bodyclose // closed by readAll
	wantDenied(t, err, ReasonPrivateAddress)

	private := strict
	private.AllowPrivateNetworks = true
	resp, err := c.Do(t.Context(), private, newReq(t, "http://svc.example.test:"+port+"/"))
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, resp); got != "ok" {
		t.Fatalf("body = %q", got)
	}

	// A literal loopback address granted by name works only with the grant.
	lit := Policy{Domains: []string{"127.0.0.1"}, Schemes: []string{"http"}}
	_, err = c.Do(t.Context(), lit, newReq(t, srv.URL)) //nolint:bodyclose // closed by readAll
	wantDenied(t, err, ReasonPrivateAddress)
	lit.AllowPrivateNetworks = true
	resp, err = c.Do(t.Context(), lit, newReq(t, srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	_ = readAll(t, resp)
}

// The private-network transport must never serve a strict policy from its
// pool.
func TestEgressPoolsAreSeparatedByPolicyClass(t *testing.T) {
	t.Parallel()
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") }))
	c := mustNew(t, Config{Resolver: fixed("svc.example.test", "127.0.0.1")})
	port := portOfServer(t, srv)
	private := Policy{Domains: []string{"svc.example.test"}, Schemes: []string{"http"}, AllowPrivateNetworks: true}
	resp, err := c.Do(t.Context(), private, newReq(t, "http://svc.example.test:"+port+"/"))
	if err != nil {
		t.Fatal(err)
	}
	_ = readAll(t, resp) // the connection returns to the private pool
	strict := private
	strict.AllowPrivateNetworks = false
	_, err = c.Do(t.Context(), strict, newReq(t, "http://svc.example.test:"+port+"/")) //nolint:bodyclose // closed by readAll
	wantDenied(t, err, ReasonPrivateAddress)
}

func TestEgressDNSRebinding(t *testing.T) {
	t.Parallel()
	t.Run("the second answer is never used", func(t *testing.T) {
		t.Parallel()
		resolver := newStubResolver(map[string][][]string{"rebind.example.test": {{publicIP}, {"127.0.0.1"}, {"169.254.169.254"}}})
		dialer := &serverDialer{} // fails after recording
		c := mustNew(t, Config{Resolver: resolver, Dialer: dialer})
		policy := Policy{Domains: []string{"rebind.example.test"}}
		_, err := c.Do(t.Context(), policy, newReq(t, "https://rebind.example.test/")) //nolint:bodyclose // closed by readAll
		if err == nil {
			t.Fatal("the test dialer fails, so the request must fail")
		}
		if n := resolver.lookups("rebind.example.test"); n != 1 {
			t.Fatalf("resolved %d times, want exactly 1 (resolve then pin)", n)
		}
		if got := dialer.addresses(); len(got) != 1 || got[0] != publicIP+":443" {
			t.Fatalf("dialed %v, want only the first, checked answer", got)
		}
	})
	t.Run("a private first answer is refused and never retried", func(t *testing.T) {
		t.Parallel()
		resolver := newStubResolver(map[string][][]string{"rebind.example.test": {{"127.0.0.1"}, {publicIP}}})
		dialer := &serverDialer{}
		c := mustNew(t, Config{Resolver: resolver, Dialer: dialer})
		policy := Policy{Domains: []string{"rebind.example.test"}}
		_, err := c.Do(t.Context(), policy, newReq(t, "https://rebind.example.test/")) //nolint:bodyclose // closed by readAll
		wantDenied(t, err, ReasonPrivateAddress)
		if n := resolver.lookups("rebind.example.test"); n != 1 {
			t.Fatalf("resolved %d times, want 1", n)
		}
		if len(dialer.addresses()) != 0 {
			t.Fatalf("dialed %v", dialer.addresses())
		}
	})
	t.Run("a connection pooled for a name does not re-resolve", func(t *testing.T) {
		t.Parallel()
		srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") }))
		resolver := newStubResolver(map[string][][]string{"rebind.example.test": {{publicIP}, {"127.0.0.1"}}})
		dialer := &serverDialer{target: hostPort(t, srv)}
		c := mustNew(t, Config{Resolver: resolver, Dialer: dialer})
		policy := Policy{Domains: []string{"rebind.example.test"}, Schemes: []string{"http"}}
		for range 2 {
			resp, err := c.Do(t.Context(), policy, newReq(t, "http://rebind.example.test/"))
			if err != nil {
				t.Fatal(err)
			}
			_ = readAll(t, resp)
		}
		for _, a := range dialer.addresses() {
			if a != publicIP+":80" {
				t.Fatalf("dialed %s, only the first answer may ever be used", a)
			}
		}
	})
}

func TestEgressResolverFailureIsNotADenial(t *testing.T) {
	t.Parallel()
	resolver := fixed("x.example.test", publicIP)
	resolver.err = errors.New("servfail")
	c := mustNew(t, Config{Resolver: resolver, Dialer: &serverDialer{}})
	_, err := c.Do(t.Context(), Policy{Domains: []string{"x.example.test"}}, newReq(t, "https://x.example.test/")) //nolint:bodyclose // closed by readAll
	if err == nil || errors.Is(err, ErrDenied) {
		t.Fatalf("a resolver failure is a network error, got %v", err)
	}
}

func TestEgressRedirects(t *testing.T) {
	t.Parallel()
	var seenKey atomic.Value
	var srv *httptest.Server
	srv = newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/to-private":
			http.Redirect(w, r, "http://127.0.0.1:"+portOfServer(t, srv)+"/final", http.StatusFound)
		case "/to-metadata":
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusMovedPermanently)
		case "/to-private-name":
			http.Redirect(w, r, "http://internal.example.test/final", http.StatusTemporaryRedirect)
		case "/to-ungranted":
			http.Redirect(w, r, "http://evil.test/final", http.StatusFound)
		case "/to-https":
			http.Redirect(w, r, "https://good.example.test/final", http.StatusFound)
		case "/to-granted":
			http.Redirect(w, r, "http://other.example.test/final", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/final":
			seenKey.Store(r.Header.Get("X-Api-Key"))
			_, _ = io.WriteString(w, "final "+r.Host) //nolint:gosec // test server echoing to a test client
		default:
			http.NotFound(w, r)
		}
	}))
	resolver := newStubResolver(map[string][][]string{
		"good.example.test":     {{publicIP}},
		"other.example.test":    {{publicIP}},
		"internal.example.test": {{"10.0.0.7"}},
	})
	dialer := &serverDialer{target: hostPort(t, srv)}
	c := mustNew(t, Config{Resolver: resolver, Dialer: dialer})
	policy := Policy{Domains: []string{"good.example.test", "other.example.test", "internal.example.test"}, Schemes: []string{"http"}}

	deniedTests := []struct {
		name, path string
		reason     Reason
	}{
		{"redirect to a private IP literal", "/to-private", ReasonDomainNotGranted},
		{"redirect to the metadata address", "/to-metadata", ReasonDomainNotGranted},
		{"redirect to a name resolving to a private address", "/to-private-name", ReasonPrivateAddress},
		{"redirect to an ungranted domain", "/to-ungranted", ReasonDomainNotGranted},
		{"redirect to a scheme the policy does not grant", "/to-https", ReasonSchemeNotAllowed},
		{"redirect loop", "/loop", ReasonTooManyRedirects},
	}
	for _, tt := range deniedTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := c.Do(t.Context(), policy, newReq(t, "http://good.example.test"+tt.path)) //nolint:bodyclose // closed by readAll
			wantDenied(t, err, tt.reason)
		})
	}

	t.Run("redirect to another granted host works and drops custom headers", func(t *testing.T) {
		t.Parallel()
		req := newReq(t, "http://good.example.test/to-granted")
		req.Header.Set("X-Api-Key", "SECRETKEY")
		resp, err := c.Do(t.Context(), policy, req)
		if err != nil {
			t.Fatal(err)
		}
		if got := readAll(t, resp); got != "final other.example.test" {
			t.Fatalf("body = %q", got)
		}
		if got, _ := seenKey.Load().(string); got != "" {
			t.Fatalf("X-Api-Key reached another origin after a redirect: %q", got)
		}
	})
}

// A redirect to an address literal the policy explicitly grants is followed
// only when the address rules allow it.
func TestEgressRedirectToGrantedPrivateLiteral(t *testing.T) {
	t.Parallel()
	var srv *httptest.Server
	srv = newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "http://127.0.0.1:"+portOfServer(t, srv)+"/final", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "final")
	}))
	c := mustNew(t, Config{Resolver: fixed("svc.example.test", "127.0.0.1")})
	policy := Policy{Domains: []string{"svc.example.test", "127.0.0.1"}, Schemes: []string{"http"}}
	_, err := c.Do(t.Context(), policy, newReq(t, "http://svc.example.test:"+portOfServer(t, srv)+"/start")) //nolint:bodyclose // closed by readAll
	wantDenied(t, err, ReasonPrivateAddress)
	policy.AllowPrivateNetworks = true
	resp, err := c.Do(t.Context(), policy, newReq(t, "http://svc.example.test:"+portOfServer(t, srv)+"/start"))
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, resp); got != "final" {
		t.Fatalf("body = %q", got)
	}
}

func TestEgressRedirectLimit(t *testing.T) {
	t.Parallel()
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := 0
		_, _ = fmt.Sscanf(r.URL.Path, "/hop/%d", &n)
		if n >= 10 {
			_, _ = io.WriteString(w, "done")
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/hop/%d", n+1), http.StatusFound)
	}))
	c := mustNew(t, Config{Resolver: fixed("good.example.test", publicIP), Dialer: &serverDialer{target: hostPort(t, srv)}})
	policy := Policy{Domains: []string{"good.example.test"}, Schemes: []string{"http"}}
	// Exactly ten redirects are followed.
	resp, err := c.Do(t.Context(), policy, newReq(t, "http://good.example.test/hop/0"))
	if err != nil {
		t.Fatalf("10 redirects must be followed: %v", err)
	}
	if got := readAll(t, resp); got != "done" {
		t.Fatalf("body = %q", got)
	}
	// The eleventh is refused.
	_, err = c.Do(t.Context(), policy, newReq(t, "http://good.example.test/hop/-1")) //nolint:bodyclose // closed by readAll
	wantDenied(t, err, ReasonTooManyRedirects)
}

func TestEgressHTTPSDowngradeAndTLS(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/down" {
			http.Redirect(w, r, "http://example.com/plain", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "secure")
	}))
	t.Cleanup(srv.Close)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	c := mustNew(t, Config{
		Resolver:  fixed("example.com", publicIP),
		Dialer:    &serverDialer{target: hostPort(t, srv)},
		TLSConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
	})
	policy := Policy{Domains: []string{"example.com"}}
	resp, err := c.Do(t.Context(), policy, newReq(t, "https://example.com/"))
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, resp); got != "secure" {
		t.Fatalf("body = %q", got)
	}
	_, err = c.Do(t.Context(), policy, newReq(t, "https://example.com/down")) //nolint:bodyclose // closed by readAll
	wantDenied(t, err, ReasonSchemeNotAllowed)
}

func TestEgressResponseSizeLimit(t *testing.T) {
	t.Parallel()
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/exact":
			_, _ = w.Write(bytes.Repeat([]byte("a"), 100))
		case "/declared":
			w.Header().Set("Content-Length", "1000")
			_, _ = w.Write(bytes.Repeat([]byte("a"), 1000))
		case "/chunked":
			fl, _ := w.(http.Flusher)
			for range 20 {
				_, _ = w.Write(bytes.Repeat([]byte("a"), 10))
				fl.Flush()
			}
		}
	}))
	var events []Event
	var mu sync.Mutex
	c := mustNew(t, Config{
		Resolver:         fixed("x.example.test", publicIP),
		Dialer:           &serverDialer{target: hostPort(t, srv)},
		MaxResponseBytes: 100,
		Observer: ObserverFunc(func(e Event) {
			mu.Lock()
			events = append(events, e)
			mu.Unlock()
		}),
	})
	policy := Policy{Domains: []string{"x.example.test"}, Schemes: []string{"http"}}
	get := func(path string, p Policy) (*http.Response, error) {
		return c.Do(t.Context(), p, newReq(t, "http://x.example.test"+path))
	}

	resp, err := get("/exact", policy)
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, resp); len(got) != 100 {
		t.Fatalf("read %d bytes at the limit", len(got))
	}

	if _, err := get("/declared", policy); !errors.Is(err, ErrResponseTooLarge) { //nolint:bodyclose // closed by readAll
		t.Fatalf("declared oversize: err = %v, want ErrResponseTooLarge", err)
	}

	resp, err = get("/chunked", policy)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("streamed oversize: err = %v, want ErrResponseTooLarge", err)
	}
	if len(b) > 100 {
		t.Fatalf("delivered %d bytes past the 100 byte limit", len(b))
	}

	// A policy may raise the cap, as a caller streaming into a blob does.
	big := policy
	big.MaxResponseBytes = 5000
	resp, err = get("/declared", big)
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, resp); len(got) != 1000 {
		t.Fatalf("read %d bytes", len(got))
	}

	mu.Lock()
	defer mu.Unlock()
	tooLarge := 0
	for _, e := range events {
		if e.Outcome == OutcomeTooLarge {
			tooLarge++
		}
	}
	if tooLarge != 2 {
		t.Fatalf("observed %d TOO_LARGE events, want 2: %+v", tooLarge, events)
	}
}

func TestEgressRequestSizeLimit(t *testing.T) {
	t.Parallel()
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := io.Copy(io.Discard, r.Body)
		_, _ = fmt.Fprintf(w, "%d", n)
	}))
	c := mustNew(t, Config{Resolver: fixed("x.example.test", publicIP), Dialer: &serverDialer{target: hostPort(t, srv)}, MaxRequestBytes: 10})
	policy := Policy{Domains: []string{"x.example.test"}, Schemes: []string{"http"}}

	post := func(body io.Reader, length int64) error {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://x.example.test/", body)
		if err != nil {
			t.Fatal(err)
		}
		req.ContentLength = length
		resp, err := c.Do(t.Context(), policy, req)
		if err == nil {
			_ = readAll(t, resp)
		}
		return err
	}
	if err := post(strings.NewReader("0123456789"), 10); err != nil {
		t.Fatalf("body at the limit: %v", err)
	}
	if err := post(strings.NewReader("0123456789ab"), 12); !errors.Is(err, ErrRequestTooLarge) {
		t.Fatalf("declared oversize body: %v", err)
	}
	// Unknown length: the stream is cut off at the limit.
	if err := post(io.MultiReader(strings.NewReader("0123456789"), strings.NewReader("overflow")), -1); err == nil {
		t.Fatal("an oversize streamed body must fail")
	}
}

func TestEgressTimeouts(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow-headers":
			select {
			case <-release:
			case <-r.Context().Done():
			}
		case "/slow-body":
			w.WriteHeader(http.StatusOK)
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}
	}))
	t.Cleanup(func() { close(release) })
	base := Config{Resolver: fixed("x.example.test", publicIP), Dialer: &serverDialer{target: hostPort(t, srv)}}

	t.Run("response header timeout", func(t *testing.T) {
		t.Parallel()
		cfg := base
		cfg.ResponseHeaderTimeout = 100 * time.Millisecond
		c := mustNew(t, cfg)
		policy := Policy{Domains: []string{"x.example.test"}, Schemes: []string{"http"}}
		start := time.Now()
		_, err := c.Do(t.Context(), policy, newReq(t, "http://x.example.test/slow-headers")) //nolint:bodyclose // closed by readAll
		if err == nil || time.Since(start) > 5*time.Second {
			t.Fatalf("err = %v after %v", err, time.Since(start))
		}
	})
	t.Run("overall timeout covers the body", func(t *testing.T) {
		t.Parallel()
		c := mustNew(t, base)
		policy := Policy{Domains: []string{"x.example.test"}, Schemes: []string{"http"}, Timeout: 150 * time.Millisecond}
		resp, err := c.Do(t.Context(), policy, newReq(t, "http://x.example.test/slow-body"))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close() //nolint:errcheck // test response
		start := time.Now()
		if _, err := io.ReadAll(resp.Body); err == nil || time.Since(start) > 5*time.Second {
			t.Fatalf("reading the stalled body: err = %v after %v", err, time.Since(start))
		}
	})
	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()
		c := mustNew(t, base)
		policy := Policy{Domains: []string{"x.example.test"}, Schemes: []string{"http"}}
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		_, err := c.Do(ctx, policy, newReq(t, "http://x.example.test/slow-headers")) //nolint:bodyclose // closed by readAll
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want deadline exceeded", err)
		}
	})
	t.Run("connect timeout", func(t *testing.T) {
		t.Parallel()
		// A listener that never accepts, with a full backlog is hard to make
		// portable; a dialer that blocks until its context ends proves the
		// connect timeout is applied to every attempt.
		c := mustNew(t, Config{Resolver: fixed("x.example.test", publicIP), Dialer: blockingDialer{}, ConnectTimeout: 100 * time.Millisecond})
		policy := Policy{Domains: []string{"x.example.test"}}
		start := time.Now()
		_, err := c.Do(t.Context(), policy, newReq(t, "https://x.example.test/")) //nolint:bodyclose // closed by readAll
		if err == nil || time.Since(start) > 5*time.Second {
			t.Fatalf("err = %v after %v", err, time.Since(start))
		}
	})
}

type blockingDialer struct{}

func (blockingDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestEgressProxy(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var proxied []string
	proxy := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		proxied = append(proxied, r.URL.String())
		mu.Unlock()
		_, _ = io.WriteString(w, "via proxy")
	}))
	direct := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "direct") }))
	resolver := newStubResolver(map[string][][]string{
		"public.example.test":  {{publicIP}},
		"private.example.test": {{"127.0.0.1"}},
	})
	// The proxy itself listens on loopback: it is operator configuration, not a
	// request destination, so it is dialed without the private-network check.
	c := mustNew(t, Config{Resolver: resolver, ProxyURL: proxy.URL})
	policy := Policy{Domains: []string{"*.example.test"}, Schemes: []string{"http"}}

	resp, err := c.Do(t.Context(), policy, newReq(t, "http://public.example.test/path?x=1"))
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, resp); got != "via proxy" {
		t.Fatalf("body = %q", got)
	}
	mu.Lock()
	if len(proxied) != 1 || proxied[0] != "http://public.example.test/path?x=1" {
		t.Fatalf("proxy saw %v", proxied)
	}
	mu.Unlock()

	// The local answer for the target is still checked before the proxy is used.
	_, err = c.Do(t.Context(), policy, newReq(t, "http://private.example.test/")) //nolint:bodyclose // closed by readAll
	wantDenied(t, err, ReasonPrivateAddress)
	mu.Lock()
	n := len(proxied)
	mu.Unlock()
	if n != 1 {
		t.Fatalf("the proxy received a refused request (%d requests)", n)
	}

	// A policy that grants private networks connects directly.
	private := policy
	private.AllowPrivateNetworks = true
	resp, err = c.Do(t.Context(), private, newReq(t, "http://private.example.test:"+portOfServer(t, direct)+"/"))
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, resp); got != "direct" {
		t.Fatalf("body = %q, want a direct connection", got)
	}
}

func TestEgressConfigErrors(t *testing.T) {
	t.Parallel()
	bad := []Config{
		{ProxyURL: "ftp://proxy.test:21"},
		{ProxyURL: "http://"},
		{ProxyURL: "://SECRETPROXYPASS@bad"},
		{Timeout: -1},
		{MaxResponseBytes: -1},
		{MaxRedirects: -1},
		{DenyHosts: []string{"*"}},
		{DenyNetworks: []netip.Prefix{{}}},
	}
	for i, cfg := range bad {
		_, err := New(cfg)
		if err == nil {
			t.Errorf("case %d: New(%+v) = nil error", i, cfg)
			continue
		}
		mustNoSecret(t, "config error", err.Error(), "SECRETPROXYPASS")
	}
	if _, err := New(Config{ProxyURL: "socks5://user:pw@proxy.test:1080"}); err != nil {
		t.Errorf("socks5 proxy: %v", err)
	}
}

func TestEgressObserverAndLogHygiene(t *testing.T) {
	t.Parallel()
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		w.WriteHeader(http.StatusTeapot)
	}))
	var logs bytes.Buffer
	var mu sync.Mutex
	var events []Event
	c := mustNew(t, Config{
		Resolver: newStubResolver(map[string][][]string{"ok.example.test": {{publicIP}}, "in.example.test": {{"10.0.0.9"}}}),
		Dialer:   &serverDialer{target: hostPort(t, srv)},
		Logger:   slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Observer: ObserverFunc(func(e Event) { mu.Lock(); events = append(events, e); mu.Unlock() }),
	})
	policy := Policy{Domains: []string{"*.example.test"}, Schemes: []string{"http"}}
	secrets := []string{"SECRETPATH", "SECRETQUERY", "SECRETHEADER", "SECRETBODY"}

	var errTexts []string
	do := func(rawURL string) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, rawURL, strings.NewReader("SECRETBODY"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer SECRETHEADER")
		resp, err := c.Do(t.Context(), policy, req)
		if err != nil {
			errTexts = append(errTexts, err.Error())
			return
		}
		_ = readAll(t, resp)
	}
	do("http://ok.example.test/hooks/SECRETPATH?token=SECRETQUERY") // completed
	do("http://in.example.test/hooks/SECRETPATH?token=SECRETQUERY") // denied: private
	do("http://evil.test/hooks/SECRETPATH?token=SECRETQUERY")       // denied: not granted
	do("http://ok.example.test/fail?token=SECRETQUERY")             // failed

	mustNoSecret(t, "log output", logs.String(), secrets...)
	mustNoSecret(t, "error text", strings.Join(errTexts, "\n"), secrets...)
	mu.Lock()
	defer mu.Unlock()
	mustNoSecret(t, "observer events", fmt.Sprintf("%+v", events), secrets...)

	var got []string
	for _, e := range events {
		got = append(got, fmt.Sprintf("%s/%s/%d", e.Outcome, e.Reason, e.Status))
	}
	want := []string{"COMPLETED//418", "DENIED/PRIVATE_ADDRESS/0", "DENIED/DOMAIN_NOT_GRANTED/0", "FAILED//0"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v, want %v", got, want)
	}
	// Log lines are valid JSON records with the fixed field set.
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		for _, k := range []string{"outcome", "scheme", "host", "method"} {
			if _, ok := rec[k]; !ok {
				t.Fatalf("log record lacks %s: %s", k, line)
			}
		}
	}
}

func TestEgressProxyCredentialsStayOutOfLogs(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	proxy := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") }))
	u, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword("proxyuser", "SECRETPROXYPASS")
	c := mustNew(t, Config{
		Resolver: fixed("x.example.test", publicIP),
		ProxyURL: u.String(),
		Logger:   slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	resp, err := c.Do(t.Context(), Policy{Domains: []string{"x.example.test"}, Schemes: []string{"http"}}, newReq(t, "http://x.example.test/"))
	if err != nil {
		t.Fatal(err)
	}
	_ = readAll(t, resp)
	mustNoSecret(t, "log output", logs.String(), "SECRETPROXYPASS", "proxyuser")
}

func TestEgressConcurrentUse(t *testing.T) {
	t.Parallel()
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") }))
	c := mustNew(t, Config{Resolver: fixed("x.example.test", publicIP), Dialer: &serverDialer{target: hostPort(t, srv)}})
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			policy := Policy{Domains: []string{"x.example.test"}, Schemes: []string{"http"}, AllowPrivateNetworks: i%2 == 0}
			for range 5 {
				resp, err := c.Do(t.Context(), policy, newReq(t, "http://x.example.test/"))
				if err != nil {
					// Even-numbered workers use the private pool, whose stub
					// answer is public; both must work.
					t.Errorf("worker %d: %v", i, err)
					return
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
		}()
	}
	wg.Wait()
}

func TestEgressHTTPClientAndRoundTripper(t *testing.T) {
	t.Parallel()
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") }))
	c := mustNew(t, Config{Resolver: fixed("x.example.test", publicIP), Dialer: &serverDialer{target: hostPort(t, srv)}})
	policy := Policy{Domains: []string{"x.example.test"}, Schemes: []string{"http"}}

	hc := c.HTTPClient(policy)
	resp, err := hc.Do(newReq(t, "http://x.example.test/"))
	if err != nil {
		t.Fatal(err)
	}
	_ = readAll(t, resp)
	_, err = hc.Do(newReq(t, "http://elsewhere.test/")) //nolint:bodyclose // refused, no response //nolint:bodyclose // closed by readAll
	wantDenied(t, err, ReasonDomainNotGranted)

	rt := c.RoundTripper(policy)
	_, err = rt.RoundTrip(newReq(t, "http://elsewhere.test/")) //nolint:bodyclose // refused, no response
	wantDenied(t, err, ReasonDomainNotGranted)
}

// A custom dialer or the default one never sees a non-TCP network.
func TestEgressRejectsNonTCPDial(t *testing.T) {
	t.Parallel()
	c := mustNew(t, Config{})
	_, err := c.dialContext(false)(t.Context(), "udp", "example.com:53")
	wantDenied(t, err, ReasonUnsupportedNetworkType)
}
