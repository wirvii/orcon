// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Policy is the network permission a request runs under. It mirrors the
// permissions.network block of a connector manifest (ADR-0029 §4). The zero
// value grants nothing: every request is refused.
type Policy struct {
	// Domains are the granted hosts: an exact host name or a pattern with a
	// single leading "*." that matches exactly one extra label ("*.example.com"
	// matches "api.example.com", not "example.com" or "a.b.example.com").
	// An IP literal is granted only by an exact entry.
	Domains []string
	// Ports restricts the destination ports. Empty means any port.
	Ports []int
	// Schemes lists the granted URL schemes, "https" and "http". Empty means
	// only "https".
	Schemes []string
	// AllowPrivateNetworks lets the request reach loopback, private, shared and
	// link-local ranges. Cloud metadata addresses stay refused.
	AllowPrivateNetworks bool
	// CredentialHosts are hosts derived from a credential field
	// (permissions.network.fromCredential). A host counts as granted only when
	// Approved is true, which the credential store sets when an administrator
	// approved the concrete host while creating the credential.
	CredentialHosts []CredentialHost

	// Timeout bounds the whole exchange, including reading the body. Zero uses
	// the client default.
	Timeout time.Duration
	// MaxResponseBytes caps the response body. Zero uses the client default.
	// Callers that stream a response into a blob set a larger value.
	MaxResponseBytes int64
}

// CredentialHost is a host taken from a credential field.
type CredentialHost struct {
	Host     string
	Approved bool
}

// Validate reports a malformed policy: an unusable domain pattern, scheme or
// port. A policy that fails validation refuses every request.
func (p Policy) Validate() error {
	for _, d := range p.Domains {
		if err := validatePattern(d); err != nil {
			return fmt.Errorf("domain %q: %w", d, err)
		}
	}
	for _, c := range p.CredentialHosts {
		if _, _, err := normalizeHost(c.Host); err != nil {
			return fmt.Errorf("credential host %q: %w", c.Host, err)
		}
	}
	for _, s := range p.Schemes {
		if s != "https" && s != "http" {
			return fmt.Errorf("scheme %q: must be https or http", s)
		}
	}
	for _, port := range p.Ports {
		if port < 1 || port > 65535 {
			return fmt.Errorf("port %d: out of range", port)
		}
	}
	if p.Timeout < 0 || p.MaxResponseBytes < 0 {
		return errors.New("timeout and size limit must not be negative")
	}
	return nil
}

func validatePattern(pattern string) error {
	host := pattern
	if rest, ok := strings.CutPrefix(pattern, "*."); ok {
		if !strings.Contains(rest, ".") {
			return errors.New("a wildcard needs a registrable suffix")
		}
		if strings.Contains(rest, "*") {
			return errors.New("only one leading wildcard label is allowed")
		}
		host = rest
	}
	norm, addr, err := normalizeHost(host)
	if err != nil {
		return err
	}
	if !addr.IsValid() && norm != host {
		return errors.New("write the host in lower case without a trailing dot")
	}
	if addr.IsValid() && host != pattern {
		return errors.New("a wildcard cannot apply to an IP address")
	}
	return nil
}

// normalizeHost lower-cases a URL host, removes one trailing dot and checks
// that it is a well-formed ASCII name or an IP literal. It rejects the
// non-canonical IPv4 spellings ("2130706433", "0x7f.1", "127.1") that a system
// resolver would turn into an address.
func normalizeHost(host string) (string, netip.Addr, error) {
	host = strings.ToLower(host)
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return "", netip.Addr{}, errors.New("empty host")
	}
	if strings.Contains(host, ":") {
		addr, err := netip.ParseAddr(host)
		if err != nil || addr.Zone() != "" {
			return "", netip.Addr{}, errors.New("malformed IPv6 literal")
		}
		return addr.String(), addr, nil
	}
	if len(host) > 253 {
		return "", netip.Addr{}, errors.New("host too long")
	}
	labels := strings.Split(host, ".")
	for _, l := range labels {
		if l == "" || len(l) > 63 {
			return "", netip.Addr{}, errors.New("empty or oversized label")
		}
		for i := range len(l) {
			c := l[i]
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' && c != '_' {
				return "", netip.Addr{}, errors.New("character not allowed in host")
			}
		}
	}
	if isNumericLabel(labels[len(labels)-1]) {
		addr, err := netip.ParseAddr(host)
		if err != nil || !addr.Is4() {
			return "", netip.Addr{}, errors.New("ambiguous numeric host")
		}
		return addr.String(), addr, nil
	}
	return host, netip.Addr{}, nil
}

// isNumericLabel reports whether a last label makes the host an IPv4 number
// in the WHATWG sense: decimal digits, or 0x followed by hex digits.
func isNumericLabel(label string) bool {
	if rest, ok := strings.CutPrefix(label, "0x"); ok {
		for i := range len(rest) {
			c := rest[i]
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return false
			}
		}
		return true
	}
	for i := range len(label) {
		if label[i] < '0' || label[i] > '9' {
			return false
		}
	}
	return label != ""
}

// matchPattern reports whether a normalized host is granted by a pattern.
func matchPattern(pattern, host string) bool {
	rest, wildcard := strings.CutPrefix(pattern, "*.")
	if !wildcard {
		if norm, _, err := normalizeHost(pattern); err == nil {
			pattern = norm // IP literals compare in canonical form
		}
		return pattern == host
	}
	label, found := strings.CutSuffix(host, "."+rest)
	return found && label != "" && !strings.Contains(label, ".")
}

// guard holds the instance-level denylist and applies policies to URLs and
// addresses. It is immutable after construction.
type guard struct {
	denyHosts    []string
	denyNetworks []netip.Prefix
}

func denied(reason Reason, scheme, host, detail string) *DeniedError {
	return &DeniedError{Reason: reason, Scheme: scheme, Host: host, Detail: detail}
}

// checkURL applies the policy to a URL before any connection is made and
// returns the normalized host.
func (g *guard) checkURL(p *Policy, u *url.URL) (string, error) {
	if u == nil {
		return "", denied(ReasonBadURL, "", "", "missing URL")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", denied(ReasonSchemeNotAllowed, "", "", "only http and https are supported")
	}
	if u.User != nil {
		return "", denied(ReasonBadURL, scheme, "", "credentials in the URL are not allowed")
	}
	host, addr, err := normalizeHost(u.Hostname())
	if err != nil {
		return "", denied(ReasonBadURL, scheme, "", err.Error())
	}
	if err := p.Validate(); err != nil {
		return "", denied(ReasonBadPolicy, scheme, host, err.Error())
	}
	if !schemeGranted(p.Schemes, scheme) {
		return "", denied(ReasonSchemeNotAllowed, scheme, host, "")
	}
	port := defaultPort(scheme)
	if s := u.Port(); s != "" {
		n, perr := strconv.Atoi(s)
		if perr != nil || n < 1 || n > 65535 {
			return "", denied(ReasonBadURL, scheme, host, "invalid port")
		}
		port = n
	}
	if len(p.Ports) > 0 && !containsInt(p.Ports, port) {
		return "", denied(ReasonPortNotAllowed, scheme, host, "")
	}
	for _, d := range g.denyHosts {
		if matchPattern(d, host) {
			return "", denied(ReasonDenylisted, scheme, host, "")
		}
	}
	if err := granted(p, scheme, host); err != nil {
		return "", err
	}
	if addr.IsValid() {
		if err := g.checkAddr(p.AllowPrivateNetworks, addr, scheme, host); err != nil {
			return "", err
		}
	}
	return host, nil
}

func granted(p *Policy, scheme, host string) error {
	for _, d := range p.Domains {
		if matchPattern(d, host) {
			return nil
		}
	}
	pending := false
	for _, c := range p.CredentialHosts {
		norm, _, err := normalizeHost(c.Host)
		if err != nil || norm != host {
			continue
		}
		if c.Approved {
			return nil
		}
		pending = true
	}
	if pending {
		return denied(ReasonCredentialHostPending, scheme, host, "the host has not been approved")
	}
	return denied(ReasonDomainNotGranted, scheme, host, "")
}

// checkAddr applies the denylist and the address ranges to one resolved or
// literal address.
func (g *guard) checkAddr(allowPrivate bool, addr netip.Addr, scheme, host string) error {
	plain := addr.WithZone("")
	if plain.Is4In6() {
		plain = plain.Unmap()
	}
	for _, n := range g.denyNetworks {
		if n.Contains(plain) {
			return denied(ReasonDenylisted, scheme, host, "address in a denied network")
		}
	}
	name, kind, blocked := classify(addr)
	switch {
	case !blocked:
		return nil
	case kind == kindPrivate && allowPrivate:
		return nil
	case strings.Contains(name, "metadata"):
		return denied(ReasonMetadataAddress, scheme, host, name)
	default:
		return denied(ReasonPrivateAddress, scheme, host, name)
	}
}

func schemeGranted(granted []string, scheme string) bool {
	if len(granted) == 0 {
		return scheme == "https"
	}
	for _, s := range granted {
		if s == scheme {
			return true
		}
	}
	return false
}

func defaultPort(scheme string) int {
	if scheme == "http" {
		return 80
	}
	return 443
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
