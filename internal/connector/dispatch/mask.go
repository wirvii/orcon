// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"encoding/base64"
	"slices"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/redact"
)

// masker replaces every resolved secret value in the items a provider sent back
// (ADR-0043 section 5, R-CRED-04). A host that echoes its request headers, or an
// error body that quotes the key, would otherwise put the secret into the node
// output, the journal response and the execution data a reader can fetch.
//
// It matches exact substrings of string values and of object keys. The forms the
// credential takes on the wire are matched too: the base64 of "user:password" (a
// BASIC header value). "Bearer <token>" needs no extra entry because the token
// is a substring of it. A provider that transforms the value in another way
// (hashing, partial display, a different encoding) is outside what the masker
// can see; threat TM-06 records that limit.
type masker struct {
	values []string
}

// newMasker builds the masker of one resolved credential. It returns nil when
// there is nothing to mask.
func newMasker(c Credential) *masker {
	set := map[string]bool{}
	for _, v := range c.Mask {
		if v != "" {
			set[v] = true
		}
	}
	if user, ok := c.Fields["username"]; ok && c.Auth.Type == declarative.AuthBasic {
		basic := user + ":" + c.Fields["password"]
		for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			set[enc.EncodeToString([]byte(basic))] = true
		}
	}
	if len(set) == 0 {
		return nil
	}
	values := make([]string, 0, len(set))
	for v := range set {
		values = append(values, v)
	}
	// Longest first, so a value that contains another is replaced whole.
	slices.SortFunc(values, func(a, b string) int {
		if len(a) != len(b) {
			return len(b) - len(a)
		}
		return strings.Compare(a, b)
	})
	return &masker{values: values}
}

func (m *masker) text(s string) string {
	if m == nil {
		return s
	}
	for _, v := range m.values {
		if strings.Contains(s, v) {
			s = strings.ReplaceAll(s, v, redact.Redacted)
		}
	}
	return s
}

// items masks a list of items.
func (m *masker) items(in []jsonvalue.Value) []jsonvalue.Value {
	if m == nil {
		return in
	}
	out := make([]jsonvalue.Value, len(in))
	for i, v := range in {
		out[i] = m.value(v)
	}
	return out
}

// value returns v with every secret replaced in its strings and keys.
func (m *masker) value(v jsonvalue.Value) jsonvalue.Value {
	switch v.Kind() {
	case jsonvalue.KindString:
		s, _ := v.Str()
		return jsonvalue.String(m.text(s))
	case jsonvalue.KindNumber:
		// A numeric secret travels as the text of a number; it becomes a string.
		if n, _ := v.NumberText(); m.text(n) != n {
			return jsonvalue.String(redact.Redacted)
		}
		return v
	case jsonvalue.KindArray:
		items := v.Items()
		out := make([]jsonvalue.Value, len(items))
		for i, it := range items {
			out[i] = m.value(it)
		}
		return jsonvalue.Array(out...)
	case jsonvalue.KindObject:
		members := v.Members()
		out := make([]jsonvalue.Member, 0, len(members))
		seen := make(map[string]bool, len(members))
		for _, mem := range members {
			key := m.text(mem.Key)
			// Two keys that both collapse to the same redacted text stay distinct.
			for n := 2; seen[key]; n++ {
				key = m.text(mem.Key) + "#" + strconv.Itoa(n)
			}
			seen[key] = true
			out = append(out, jsonvalue.Member{Key: key, Value: m.value(mem.Value)})
		}
		obj, err := jsonvalue.Object(out...)
		if err != nil {
			return jsonvalue.String(redact.Redacted)
		}
		return obj
	case jsonvalue.KindInvalid, jsonvalue.KindNull, jsonvalue.KindBool:
		return v
	}
	return v
}
