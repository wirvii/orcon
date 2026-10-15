// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// maxHosts bounds the allowed-host list of one credential.
const maxHosts = 20

// maxHostLength bounds one host entry, in octets, as DNS does.
const maxHostLength = 253

// normalizeHost returns the canonical form of an allowed-host entry: lowercase, without a trailing dot. An entry is a
// DNS name ("api.example.com") or a wildcard with one leading label ("*.example.com"). Schemes, ports, paths, user
// information and IP literals are refused: the host binding compares names only.
func normalizeHost(entry string) (string, error) {
	h := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(entry), "."))
	if h == "" || len(h) > maxHostLength {
		return "", fmt.Errorf("%w: an allowed host must be 1 to %d characters", ErrInvalid, maxHostLength)
	}
	name := h
	wildcard := strings.HasPrefix(h, "*.")
	if wildcard {
		name = h[2:]
	}
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("%w: an allowed host needs a domain with at least two labels", ErrInvalid)
	}
	for _, label := range labels {
		if !validLabel(label) {
			return "", fmt.Errorf("%w: an allowed host may contain only letters, digits and hyphens between dots", ErrInvalid)
		}
	}
	return h, nil
}

// validLabel reports whether one DNS label is 1 to 63 letters, digits or inner hyphens.
func validLabel(label string) bool {
	if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for i := range len(label) {
		c := label[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
		default:
			return false
		}
	}
	return true
}

// normalizeHosts validates and canonicalizes a host list, dropping duplicates and keeping the order.
func normalizeHosts(entries []string) ([]string, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("%w: a credential needs at least one allowed host", ErrInvalid)
	}
	if len(entries) > maxHosts {
		return nil, fmt.Errorf("%w: at most %d allowed hosts", ErrInvalid, maxHosts)
	}
	seen := make(map[string]bool, len(entries))
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		h, err := normalizeHost(e)
		if err != nil {
			return nil, err
		}
		if !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	return out, nil
}

// hostAllowed reports whether host matches one of the allowed entries. The request host is compared in the same
// canonical form; an empty or malformed host matches nothing.
func hostAllowed(allowed []string, host string) bool {
	h := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if h == "" {
		return false
	}
	for _, entry := range allowed {
		if suffix, ok := strings.CutPrefix(entry, "*."); ok {
			// A wildcard matches exactly one extra label: "x.example.com" for "*.example.com", not "a.b.example.com".
			label, rest, found := strings.Cut(h, ".")
			if found && label != "" && rest == suffix {
				return true
			}
			continue
		}
		if h == entry {
			return true
		}
	}
	return false
}

// EncodeHosts returns the stored form of an allowed-host list: a canonical JSON array of the entries, in order. The
// adapters store it as the allowed_host_document of a credential.
func EncodeHosts(hosts []string) ([]byte, error) {
	items := make([]jsonvalue.Value, 0, len(hosts))
	for _, h := range hosts {
		items = append(items, jsonvalue.String(h))
	}
	doc, err := jsonvalue.Canonical(jsonvalue.Array(items...))
	if err != nil {
		return nil, fmt.Errorf("encode allowed hosts: %w", err)
	}
	return doc, nil
}

// DecodeHosts reads the stored form written by [EncodeHosts]. A damaged document is an error.
func DecodeHosts(doc []byte) ([]string, error) {
	v, err := jsonvalue.Parse(doc)
	if err != nil {
		return nil, fmt.Errorf("decode allowed hosts: %w", err)
	}
	items := v.Items()
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.Str()
		if !ok {
			return nil, errors.New("decode allowed hosts: an entry is not a string")
		}
		out = append(out, s)
	}
	return out, nil
}
