// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package egress is the outbound HTTP client of Orcon (ADR-0029 §8). Every
// network request made on behalf of a connector, a webhook subscription or an
// external secret store goes through it, so one place decides where Orcon may
// connect.
//
// A [Client] is built once by the composition root from instance-level
// settings ([Config]): timeouts, size limits, an optional HTTP proxy, a
// denylist, a name resolver and observers. Each use then supplies a [Policy]
// that mirrors the manifest permissions.network block: the granted domains,
// ports and schemes, whether private networks are allowed, and the hosts that
// were approved when a credential was created.
//
// The package refuses, before and during the connection:
//
//   - a host that is not granted, or that the instance denylist names;
//   - a scheme other than HTTPS unless the policy grants HTTP;
//   - loopback, private (RFC 1918), shared (RFC 6598), link-local, unique-local
//     IPv6, IPv4-mapped and other embedded forms of those, multicast, reserved
//     and documentation ranges, and cloud metadata addresses;
//   - a host whose resolved addresses include any refused address (the name is
//     resolved once and the connection is made to the checked address, so a
//     second, different answer cannot be used);
//   - a redirect to anything the policy would refuse as a first request, and
//     more than [DefaultMaxRedirects] redirects;
//   - a response larger than the size limit.
//
// Cloud metadata addresses stay refused even when a policy allows private
// networks. Refusals are [*DeniedError] values that match [ErrDenied] with
// [errors.Is]; error text and logs carry the scheme and host only, never the
// path, query, headers or any credential.
//
// Callers inject the result as an [*http.Client] ([Client.HTTPClient]) or an
// [http.RoundTripper] ([Client.RoundTripper]); no other package imports this
// one (architecture-layers.md AL-8).
package egress
