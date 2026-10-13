// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"testing"
)

// Security review findings F1 to F4.

func TestEgressDenyNetworksSeeThroughTranslationForms(t *testing.T) {
	t.Parallel()
	c, err := New(Config{DenyNetworks: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("93.184.0.0/16")}})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		"10.0.0.1",
		"::ffff:10.0.0.1",
		"64:ff9b::a00:1",        // NAT64 /96
		"64:ff9b:1:a00:0:100::", // RFC 6052 /48 layout: 10.0.0.1 at bytes 6,7,9,10
		"2002:0a00:0001::",      // 6to4
		"2002:5db8:d822::1",     // 6to4 of 93.184.216.34
		"64:ff9b::5db8:d822",    // NAT64 of 93.184.216.34
		"::ffff:93.184.216.34",  // mapped
	} {
		err := c.guard.checkAddr(true, netip.MustParseAddr(s), "https", "x")
		if err == nil || !errors.Is(err, ErrDenied) {
			t.Errorf("checkAddr(%s) = %v, want a denial", s, err)
		}
	}
	if err := c.guard.checkAddr(true, netip.MustParseAddr("64:ff9b::808:808"), "https", "x"); err != nil {
		t.Errorf("NAT64 of an address outside the denied networks: %v", err)
	}
	// The reason is the denylist even for allowed private networks.
	err = c.guard.checkAddr(true, netip.MustParseAddr("64:ff9b::a00:1"), "https", "x")
	wantDenied(t, err, ReasonDenylisted)
}

func TestEgressEmbeddedV4(t *testing.T) {
	t.Parallel()
	tests := []struct{ addr, want string }{
		{"::ffff:1.2.3.4", "1.2.3.4"},
		{"64:ff9b::102:304", "1.2.3.4"},
		{"2002:102:304::", "1.2.3.4"},
		{"64:ff9b:1:102:3:400::", "1.2.3.4"},
		{"2606:4700::1", ""},
		{"1.2.3.4", ""},
	}
	for _, tt := range tests {
		got, ok := embeddedV4(netip.MustParseAddr(tt.addr))
		if tt.want == "" {
			if ok {
				t.Errorf("embeddedV4(%s) = %s, want none", tt.addr, got)
			}
			continue
		}
		if !ok || got.String() != tt.want {
			t.Errorf("embeddedV4(%s) = %s, %v; want %s", tt.addr, got, ok, tt.want)
		}
	}
}

func TestEgressMoreMetadataAndTranslationRanges(t *testing.T) {
	t.Parallel()
	c := newGuard(t)
	p := Policy{AllowPrivateNetworks: true, Schemes: []string{"http"}}
	for _, ip := range []string{
		"169.254.170.2", "169.254.169.253", "169.254.169.123", // AWS link-local services
		"168.63.129.16", "192.0.0.192", "100.100.100.200", "169.254.169.254",
		"fd00:ec2::254", "fd00:ec2::23", "fd00:ec2::1",
		"::ffff:0:1:2", "::ffff:0:0:0", "::ffff:0:7f00:1", // SIIT
		"2001:2::1",
	} {
		addr := netip.MustParseAddr(ip)
		if _, blocked := BlockedRange(addr); !blocked {
			t.Errorf("BlockedRange(%s) = false", ip)
		}
		if err := c.guard.checkAddr(p.AllowPrivateNetworks, addr, "http", "x"); err == nil {
			t.Errorf("%s is allowed with AllowPrivateNetworks; it must stay refused", ip)
		}
	}
	// Ordinary private link-local addresses next to them remain grantable.
	for _, ip := range []string{"169.254.1.1", "fd00:ec3::1", "10.0.0.1"} {
		if err := c.guard.checkAddr(true, netip.MustParseAddr(ip), "http", "x"); err != nil {
			t.Errorf("%s with AllowPrivateNetworks: %v", ip, err)
		}
	}
	for _, host := range []string{"metadata", "instance-data", "instance-data.ec2.internal", "metadata.tencentyun.com", "metadata.google.internal"} {
		wantDenied(t, c.Check(Policy{Domains: []string{host}}, "https://"+host+"/"), ReasonDenylisted)
	}
}

func TestEgressHostHeaderMustMatchURL(t *testing.T) {
	t.Parallel()
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Host) //nolint:gosec // test server echoing to a test client
	}))
	c := mustNew(t, Config{Resolver: fixed("good.example.test", publicIP), Dialer: &serverDialer{target: hostPort(t, srv)}})
	policy := Policy{Domains: []string{"good.example.test"}, Schemes: []string{"http"}}

	for _, host := range []string{"evil.test", "internal.example.test", "good.example.test.evil.test", "127.0.0.1"} {
		req := newReq(t, "http://good.example.test/")
		req.Host = host
		_, err := c.Do(t.Context(), policy, req) //nolint:bodyclose // refused, no response
		wantDenied(t, err, ReasonBadURL)
	}
	for _, host := range []string{"", "good.example.test", "GOOD.example.test", "good.example.test:80", "good.example.test."} {
		req := newReq(t, "http://good.example.test/")
		req.Host = host
		resp, err := c.Do(t.Context(), policy, req)
		if err != nil {
			t.Fatalf("Host %q: %v", host, err)
		}
		_ = readAll(t, resp)
	}
}

// Content-Encoding is decoded by the transport; the cap applies to the
// decoded bytes, so a compressed bomb cannot exceed it.
func TestEgressCompressedResponseCountsDecodedBytes(t *testing.T) {
	t.Parallel()
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		buf := make([]byte, 1<<20)
		for range 50 {
			if _, err := zw.Write(buf); err != nil {
				return
			}
		}
		_ = zw.Close()
	}))
	c := mustNew(t, Config{Resolver: fixed("x.example.test", publicIP), Dialer: &serverDialer{target: hostPort(t, srv)}})
	policy := Policy{Domains: []string{"x.example.test"}, Schemes: []string{"http"}, MaxResponseBytes: 1 << 20}
	resp, err := c.Do(t.Context(), policy, newReq(t, "http://x.example.test/"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test response
	n, err := io.Copy(io.Discard, resp.Body)
	if !errors.Is(err, ErrResponseTooLarge) || n > 1<<20 {
		t.Fatalf("read %d bytes, err = %v; want a cut-off at the limit", n, err)
	}
}
