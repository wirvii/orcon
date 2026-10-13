// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"bufio"
	"errors"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"testing"
)

type corpusEntry struct {
	reason string
	policy string
	url    string
}

func loadCorpus(tb testing.TB) []corpusEntry {
	tb.Helper()
	f, err := os.Open("testdata/ssrf-corpus.txt")
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // read-only file
	var out []corpusEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			tb.Fatalf("corpus line %q: want 3 tab-separated fields", line)
		}
		out = append(out, corpusEntry{parts[0], parts[1], parts[2]})
	}
	if err := sc.Err(); err != nil {
		tb.Fatal(err)
	}
	return out
}

func corpusPolicy(e corpusEntry) Policy {
	switch e.policy {
	case "std":
		return Policy{Domains: []string{"example.com", "*.example.com"}}
	case "ports":
		return Policy{Domains: []string{"example.com", "*.example.com"}, Ports: []int{443}}
	default: // granted: the URL's own host is granted, so only the address rules decide
		u, err := url.Parse(e.url)
		if err != nil {
			return Policy{}
		}
		return Policy{Domains: []string{strings.ToLower(u.Hostname())}}
	}
}

func newGuard(tb testing.TB) *Client {
	tb.Helper()
	c, err := New(Config{})
	if err != nil {
		tb.Fatal(err)
	}
	return c
}

func TestEgressSSRFCorpus(t *testing.T) {
	t.Parallel()
	c := newGuard(t)
	corpus := loadCorpus(t)
	if len(corpus) < 80 {
		t.Fatalf("corpus has %d entries; it must not shrink below 80", len(corpus))
	}
	for _, e := range corpus {
		t.Run(e.url, func(t *testing.T) {
			t.Parallel()
			err := c.Check(corpusPolicy(e), e.url)
			if e.reason == "OK" {
				if err != nil {
					t.Fatalf("Check(%s) = %v, want allowed", e.url, err)
				}
				return
			}
			wantDenied(t, err, Reason(e.reason))
		})
	}
}

func TestEgressBlockedRanges(t *testing.T) {
	t.Parallel()
	tests := []struct {
		addr    string
		blocked bool
	}{
		{"8.8.8.8", false},
		{"93.184.216.34", false},
		{"2606:4700:4700::1111", false},
		{"::ffff:8.8.8.8", false},
		{"64:ff9b::808:808", false},
		{"2002:808:808::", false},
		{"127.0.0.1", true},
		{"::1", true},
		{"::ffff:127.0.0.1", true},
		{"::ffff:7f00:1", true},
		{"10.1.2.3", true},
		{"::ffff:10.1.2.3", true},
		{"172.16.0.1", true},
		{"192.168.0.1", true},
		{"100.64.0.1", true},
		{"169.254.169.254", true},
		{"::ffff:169.254.169.254", true},
		{"fd00:ec2::254", true},
		{"fe80::1", true},
		{"fe80::1%eth0", true},
		{"fc00::1", true},
		{"fdff::1", true},
		{"64:ff9b::7f00:1", true},
		{"2002:7f00:1::", true},
		{"::", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"224.0.0.251", true},
		{"ff02::fb", true},
		{"::127.0.0.1", true},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			t.Parallel()
			addr, err := netip.ParseAddr(tt.addr)
			if err != nil {
				t.Fatal(err)
			}
			name, blocked := BlockedRange(addr)
			if blocked != tt.blocked {
				t.Fatalf("BlockedRange(%s) = %q, %v; want blocked=%v", tt.addr, name, blocked, tt.blocked)
			}
			if blocked && name == "" {
				t.Fatalf("BlockedRange(%s) blocked without naming the range", tt.addr)
			}
		})
	}
	if _, blocked := BlockedRange(netip.Addr{}); !blocked {
		t.Fatal("the zero address must be blocked")
	}
}

func TestEgressAllowPrivateKeepsMetadataBlocked(t *testing.T) {
	t.Parallel()
	c := newGuard(t)
	p := Policy{Domains: []string{"127.0.0.1", "10.0.0.5", "169.254.169.254", "0.0.0.0", "224.0.0.1", "::1", "fd00:ec2::254"}, AllowPrivateNetworks: true, Schemes: []string{"http"}}
	for _, ok := range []string{"http://127.0.0.1/", "http://10.0.0.5/", "http://[::1]/"} {
		if err := c.Check(p, ok); err != nil {
			t.Errorf("Check(%s) with AllowPrivateNetworks = %v, want allowed", ok, err)
		}
	}
	for _, bad := range []string{"http://169.254.169.254/latest/meta-data", "http://[fd00:ec2::254]/", "http://0.0.0.0/", "http://224.0.0.1/"} {
		err := c.Check(p, bad)
		if err == nil {
			t.Errorf("Check(%s) allowed; metadata, unspecified and multicast stay refused", bad)
		}
		if !errors.Is(err, ErrDenied) {
			t.Errorf("Check(%s) = %v, want a denial", bad, err)
		}
	}
}

func TestEgressDenylistHooks(t *testing.T) {
	t.Parallel()
	c, err := New(Config{
		DenyHosts:    []string{"*.corp.example.com", "blocked.example.com"},
		DenyNetworks: []netip.Prefix{netip.MustParsePrefix("10.9.0.0/16"), netip.MustParsePrefix("203.0.114.5/24")},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := Policy{Domains: []string{"*.example.com", "*.corp.example.com", "blocked.example.com", "10.9.1.1", "10.8.1.1", "203.0.114.77"}, AllowPrivateNetworks: true}
	wantDenied(t, c.Check(p, "https://blocked.example.com/"), ReasonDenylisted)
	wantDenied(t, c.Check(p, "https://x.corp.example.com/"), ReasonDenylisted)
	wantDenied(t, c.Check(p, "https://10.9.1.1/"), ReasonDenylisted)
	wantDenied(t, c.Check(p, "https://203.0.114.77/"), ReasonDenylisted)
	if err := c.Check(p, "https://10.8.1.1/"); err != nil {
		t.Errorf("an address outside the denied network must stay allowed: %v", err)
	}
	if err := c.Check(p, "https://api.example.com/"); err != nil {
		t.Errorf("unrelated host: %v", err)
	}
	wantDenied(t, newGuard(t).Check(Policy{Domains: []string{"metadata.google.internal"}}, "https://metadata.google.internal/"), ReasonDenylisted)
}

func TestEgressDomainMatching(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern, host string
		want          bool
	}{
		{"example.com", "example.com", true},
		{"example.com", "api.example.com", false},
		{"*.example.com", "api.example.com", true},
		{"*.example.com", "example.com", false},
		{"*.example.com", "a.b.example.com", false},
		{"*.example.com", "evilexample.com", false},
		{"*.example.com", "xexample.com", false},
		{"*.example.com", ".example.com", false},
		{"*.example.com", "api.example.com.evil.test", false},
		{"1.2.3.4", "1.2.3.4", true},
		{"1.2.3.4", "1.2.3.40", false},
	}
	for _, tt := range tests {
		if got := matchPattern(tt.pattern, tt.host); got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.host, got, tt.want)
		}
	}
}

func TestEgressEmptyPolicyDeniesEverything(t *testing.T) {
	t.Parallel()
	c := newGuard(t)
	wantDenied(t, c.Check(Policy{}, "https://example.com/"), ReasonDomainNotGranted)
}

func TestEgressPolicyValidate(t *testing.T) {
	t.Parallel()
	bad := []Policy{
		{Domains: []string{"*"}},
		{Domains: []string{"*.com"}},
		{Domains: []string{"*.*.example.com"}},
		{Domains: []string{"Example.com"}},
		{Domains: []string{"example.com."}},
		{Domains: []string{"*.127.0.0.1"}},
		{Domains: []string{"exa mple.com"}},
		{Domains: []string{""}},
		{Schemes: []string{"ftp"}},
		{Ports: []int{0}},
		{Ports: []int{70000}},
		{Timeout: -1},
		{MaxResponseBytes: -1},
		{CredentialHosts: []CredentialHost{{Host: "bad host"}}},
	}
	for i, p := range bad {
		if err := p.Validate(); err == nil {
			t.Errorf("case %d: Validate() = nil for %+v", i, p)
		}
	}
	good := Policy{Domains: []string{"example.com", "*.example.com", "10.0.0.1", "::1"}, Ports: []int{443, 8443}, Schemes: []string{"https", "http"}}
	if err := good.Validate(); err != nil {
		t.Errorf("Validate() = %v", err)
	}
	// A malformed policy refuses requests instead of failing open.
	wantDenied(t, newGuard(t).Check(Policy{Domains: []string{"*"}}, "https://example.com/"), ReasonBadPolicy)
}

func TestEgressCredentialHost(t *testing.T) {
	t.Parallel()
	c := newGuard(t)
	pending := Policy{CredentialHosts: []CredentialHost{{Host: "erp.customer.test", Approved: false}}}
	wantDenied(t, c.Check(pending, "https://erp.customer.test/api"), ReasonCredentialHostPending)
	approved := Policy{CredentialHosts: []CredentialHost{{Host: "ERP.customer.test", Approved: true}}}
	if err := c.Check(approved, "https://erp.customer.test/api"); err != nil {
		t.Fatalf("approved credential host: %v", err)
	}
	wantDenied(t, c.Check(approved, "https://other.customer.test/api"), ReasonDomainNotGranted)
	// An approved host never widens what the address rules allow.
	internal := Policy{CredentialHosts: []CredentialHost{{Host: "10.1.1.1", Approved: true}}}
	wantDenied(t, c.Check(internal, "https://10.1.1.1/"), ReasonPrivateAddress)
	internal.AllowPrivateNetworks = true
	if err := c.Check(internal, "https://10.1.1.1/"); err != nil {
		t.Fatalf("approved internal host with private networks granted: %v", err)
	}
}

func TestEgressSchemeAndPortGrants(t *testing.T) {
	t.Parallel()
	c := newGuard(t)
	p := Policy{Domains: []string{"example.com"}}
	wantDenied(t, c.Check(p, "http://example.com/"), ReasonSchemeNotAllowed)
	p.Schemes = []string{"http"}
	wantDenied(t, c.Check(p, "https://example.com/"), ReasonSchemeNotAllowed)
	if err := c.Check(p, "http://example.com:8080/"); err != nil {
		t.Fatal(err)
	}
	p.Schemes = []string{"http", "https"}
	p.Ports = []int{80}
	wantDenied(t, c.Check(p, "https://example.com/"), ReasonPortNotAllowed)
	if err := c.Check(p, "http://example.com/"); err != nil {
		t.Fatal(err)
	}
}

func TestEgressDeniedErrorHidesPathAndQuery(t *testing.T) {
	t.Parallel()
	err := newGuard(t).Check(Policy{}, "https://example.com/hooks/SECRETPATH?token=SECRETQUERY")
	mustNoSecret(t, "denial", err.Error(), "SECRETPATH", "SECRETQUERY")
	err = newGuard(t).Check(Policy{}, "https://user:SECRETPASS@example.com/")
	mustNoSecret(t, "denial", err.Error(), "SECRETPASS")
}

func FuzzEgressBlockedRange(f *testing.F) {
	for _, s := range []string{"127.0.0.1", "::ffff:10.0.0.1", "64:ff9b::7f00:1", "2002:7f00:1::", "8.8.8.8", "fe80::1%eth0", "::", "fd00:ec2::254"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return
		}
		_, blocked := BlockedRange(addr)
		// An address and its IPv4-mapped form are judged alike.
		if addr.Is4() {
			if _, mapped := BlockedRange(netip.AddrFrom16(addr.As16())); mapped != blocked {
				t.Fatalf("%s: plain blocked=%v, mapped blocked=%v", addr, blocked, mapped)
			}
		}
		// A zone never changes the verdict.
		if addr.Is6() {
			if _, zoned := BlockedRange(addr.WithZone("eth0")); zoned != blocked {
				t.Fatalf("%s: zone changed the verdict", addr)
			}
		}
		// Loopback, private, link-local and unspecified are always blocked.
		if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() || addr.IsMulticast() {
			if !blocked {
				t.Fatalf("%s is loopback/private/link-local/unspecified/multicast but not blocked", addr)
			}
		}
	})
}

func FuzzEgressCheckURL(f *testing.F) {
	for _, e := range loadCorpus(f) {
		f.Add(e.url)
	}
	c, err := New(Config{})
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		u, err := url.Parse(raw)
		if err != nil {
			return
		}
		// Grant the URL's own host by name: only the address and format rules
		// stand between the URL and an allowed verdict.
		p := Policy{Domains: []string{strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))}, Schemes: []string{"https", "http"}}
		if verr := p.Validate(); verr != nil {
			return
		}
		if cerr := c.Check(p, raw); cerr != nil {
			if !errors.Is(cerr, ErrDenied) {
				t.Fatalf("Check(%q) returned a non-denial error: %v", raw, cerr)
			}
			return
		}
		// Allowed: the host must be a plain name or a public literal.
		host, addr, herr := normalizeHost(u.Hostname())
		if herr != nil {
			t.Fatalf("Check(%q) allowed a host that does not normalize: %v", raw, herr)
		}
		if addr.IsValid() {
			if name, blocked := BlockedRange(addr); blocked {
				t.Fatalf("Check(%q) allowed %s in blocked range %q", raw, addr, name)
			}
		}
		if u.User != nil {
			t.Fatalf("Check(%q) allowed credentials in the URL", raw)
		}
		if strings.HasSuffix(host, "metadata.google.internal") && host == "metadata.google.internal" {
			t.Fatalf("Check(%q) allowed a denylisted metadata host", raw)
		}
	})
}
