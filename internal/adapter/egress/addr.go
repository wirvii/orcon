// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"net/netip"
)

// rangeKind says whether a policy that allows private networks may reach a
// range.
type rangeKind uint8

const (
	// Private ranges are reachable only when the policy sets
	// AllowPrivateNetworks.
	kindPrivate rangeKind = iota + 1
	// Never ranges are never reachable: cloud metadata, unspecified,
	// multicast, broadcast, reserved and documentation ranges.
	kindNever
)

type namedPrefix struct {
	prefix netip.Prefix
	name   string
	kind   rangeKind
}

func np(prefix, name string, kind rangeKind) namedPrefix {
	return namedPrefix{prefix: netip.MustParsePrefix(prefix), name: name, kind: kind}
}

// Order matters: the first match wins, so specific entries (metadata) precede
// the ranges that contain them.
var blockedRanges = []namedPrefix{
	// Cloud metadata services.
	np("169.254.169.254/32", "cloud metadata", kindNever),
	np("fd00:ec2::254/128", "cloud metadata", kindNever),
	np("168.63.129.16/32", "cloud metadata", kindNever),
	np("100.100.100.200/32", "cloud metadata", kindNever),
	np("192.0.0.192/32", "cloud metadata", kindNever),
	// AWS: ECS task credentials, DNS and time-sync link-local services, and
	// the whole IPv6 instance-metadata network (fd00:ec2::254, fd00:ec2::23).
	np("169.254.170.2/32", "cloud metadata", kindNever),
	np("169.254.169.253/32", "cloud metadata", kindNever),
	np("169.254.169.123/32", "cloud metadata", kindNever),
	np("fd00:ec2::/64", "cloud metadata", kindNever),

	// IPv4.
	np("0.0.0.0/8", "unspecified", kindNever),
	np("10.0.0.0/8", "private (RFC 1918)", kindPrivate),
	np("100.64.0.0/10", "shared address space (RFC 6598)", kindPrivate),
	np("127.0.0.0/8", "loopback", kindPrivate),
	np("169.254.0.0/16", "link-local", kindPrivate),
	np("172.16.0.0/12", "private (RFC 1918)", kindPrivate),
	np("192.0.0.0/24", "IETF protocol assignments", kindNever),
	np("192.0.2.0/24", "documentation", kindNever),
	np("192.88.99.0/24", "6to4 relay anycast", kindNever),
	np("192.168.0.0/16", "private (RFC 1918)", kindPrivate),
	np("198.18.0.0/15", "benchmarking", kindNever),
	np("198.51.100.0/24", "documentation", kindNever),
	np("203.0.113.0/24", "documentation", kindNever),
	np("224.0.0.0/4", "multicast", kindNever),
	np("240.0.0.0/4", "reserved", kindNever),

	// IPv6.
	np("::1/128", "loopback", kindPrivate),
	np("::/96", "unspecified or IPv4-compatible", kindNever),
	np("100::/64", "discard-only", kindNever),
	np("::ffff:0:0:0/96", "SIIT translated", kindNever),
	np("2001::/32", "Teredo", kindNever),
	np("2001:2::/48", "benchmarking", kindNever),
	np("2001:10::/28", "ORCHID", kindNever),
	np("2001:20::/28", "ORCHIDv2", kindNever),
	np("2001:db8::/32", "documentation", kindNever),
	np("3fff::/20", "documentation", kindNever),
	np("5f00::/16", "segment routing SID", kindNever),
	np("64:ff9b:1::/48", "local-use NAT64", kindNever),
	np("fc00::/7", "unique local (RFC 4193)", kindPrivate),
	np("fe80::/10", "link-local", kindPrivate),
	np("fec0::/10", "site-local (deprecated)", kindPrivate),
	np("ff00::/8", "multicast", kindNever),
}

var (
	nat64Prefix      = netip.MustParsePrefix("64:ff9b::/96")
	localNAT64Prefix = netip.MustParsePrefix("64:ff9b:1::/48")
	sixToFour        = netip.MustParsePrefix("2002::/16")
)

// embeddedV4 returns the IPv4 address an IPv6 address carries inside a
// translation or tunnel form: IPv4-mapped (::ffff:a.b.c.d), NAT64
// (64:ff9b::/96 and the local-use 64:ff9b:1::/48, RFC 6052 and RFC 8215) and
// 6to4 (2002::/16). Denylists are matched against it as well as against the
// address itself, so a denied IPv4 network cannot be reached through its
// IPv6 spelling.
func embeddedV4(addr netip.Addr) (netip.Addr, bool) {
	addr = addr.WithZone("")
	switch {
	case addr.Is4In6():
		return addr.Unmap(), true
	case !addr.Is6():
		return netip.Addr{}, false
	}
	raw := addr.As16()
	switch {
	case nat64Prefix.Contains(addr):
		return netip.AddrFrom4([4]byte{raw[12], raw[13], raw[14], raw[15]}), true
	case localNAT64Prefix.Contains(addr):
		// RFC 6052 section 2.2: for a /48 prefix the IPv4 address occupies
		// bytes 6, 7, 9 and 10 (byte 8 is reserved).
		return netip.AddrFrom4([4]byte{raw[6], raw[7], raw[9], raw[10]}), true
	case sixToFour.Contains(addr):
		return netip.AddrFrom4([4]byte{raw[2], raw[3], raw[4], raw[5]}), true
	}
	return netip.Addr{}, false
}

// BlockedRange reports whether addr belongs to a range Orcon refuses to
// connect to by default, and names the range. IPv4-mapped IPv6 addresses
// (::ffff:a.b.c.d), NAT64 addresses (64:ff9b::/96) and 6to4 addresses
// (2002::/16) are judged by the IPv4 address they embed. An invalid address
// is blocked.
func BlockedRange(addr netip.Addr) (name string, blocked bool) {
	name, _, blocked = classify(addr)
	return name, blocked
}

func classify(addr netip.Addr) (name string, kind rangeKind, blocked bool) {
	if !addr.IsValid() {
		return "invalid address", kindNever, true
	}
	addr = addr.WithZone("")
	if addr.Is4In6() {
		n, k, b := classify(addr.Unmap())
		if b {
			return "IPv4-mapped " + n, k, true
		}
		return "", 0, false
	}
	if addr.Is6() {
		raw := addr.As16()
		switch {
		case nat64Prefix.Contains(addr):
			n, k, b := classify(netip.AddrFrom4([4]byte{raw[12], raw[13], raw[14], raw[15]}))
			if b {
				return "NAT64 " + n, k, true
			}
			return "", 0, false
		case sixToFour.Contains(addr):
			n, k, b := classify(netip.AddrFrom4([4]byte{raw[2], raw[3], raw[4], raw[5]}))
			if b {
				return "6to4 " + n, k, true
			}
			return "", 0, false
		}
	}
	for _, r := range blockedRanges {
		if r.prefix.Contains(addr) {
			return r.name, r.kind, true
		}
	}
	// Anything that is not global unicast and was not named above (for
	// example the IPv4 broadcast address).
	if !addr.IsGlobalUnicast() {
		return "not global unicast", kindNever, true
	}
	return "", 0, false
}
