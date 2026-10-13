// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// publicIP is a routable documentation-free address used as the "public"
// answer of stub resolvers (it is example.com's historical address).
const publicIP = "93.184.216.34"

// stubResolver answers from a script. Each lookup of a name consumes the next
// answer; the last answer repeats.
type stubResolver struct {
	mu      sync.Mutex
	answers map[string][][]string
	calls   map[string]int
	err     error
}

func newStubResolver(answers map[string][][]string) *stubResolver {
	return &stubResolver{answers: answers, calls: map[string]int{}}
}

func fixed(host string, addrs ...string) *stubResolver {
	return newStubResolver(map[string][][]string{host: {addrs}})
}

func (r *stubResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	seq, ok := r.answers[host]
	if !ok {
		return nil, fmt.Errorf("stub: no such host %s", host)
	}
	i := r.calls[host]
	r.calls[host]++
	if i >= len(seq) {
		i = len(seq) - 1
	}
	out := make([]netip.Addr, 0, len(seq[i]))
	for _, s := range seq[i] {
		out = append(out, netip.MustParseAddr(s))
	}
	return out, nil
}

func (r *stubResolver) lookups(host string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[host]
}

// serverDialer records the address it was asked to connect to and connects to
// a local test server instead, whatever the address.
type serverDialer struct {
	target string // host:port of the test server; empty fails every dial

	mu     sync.Mutex
	dialed []string
}

func (d *serverDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d.mu.Lock()
	d.dialed = append(d.dialed, address)
	d.mu.Unlock()
	if d.target == "" {
		return nil, errors.New("test dialer: refusing to connect")
	}
	var nd net.Dialer
	return nd.DialContext(ctx, network, d.target)
}

func (d *serverDialer) addresses() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.dialed...)
}

func newServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s
}

func hostPort(t *testing.T, s *httptest.Server) string {
	t.Helper()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func mustNew(t *testing.T, cfg Config) *Client {
	t.Helper()
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.CloseIdleConnections)
	return c
}

func newReq(t *testing.T, rawURL string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// wantDenied asserts err is a DeniedError with the given reason.
func wantDenied(t *testing.T, err error, reason Reason) {
	t.Helper()
	if err == nil {
		t.Fatalf("got nil error, want denial %s", reason)
	}
	if !errors.Is(err, ErrDenied) {
		t.Fatalf("error %v does not match ErrDenied", err)
	}
	var de *DeniedError
	if !errors.As(err, &de) {
		t.Fatalf("error %v is not a *DeniedError", err)
	}
	if de.Reason != reason {
		t.Fatalf("denial reason = %s (%v), want %s", de.Reason, err, reason)
	}
	if de.Code() != "EgressDenied" {
		t.Fatalf("Code() = %q", de.Code())
	}
}

func mustNoSecret(t *testing.T, where, text string, secrets ...string) {
	t.Helper()
	for _, s := range secrets {
		if strings.Contains(text, s) {
			t.Fatalf("%s leaks %q: %s", where, s, text)
		}
	}
}
