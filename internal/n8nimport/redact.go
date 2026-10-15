// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// SecretMarker replaces a literal value that looks like a secret. It is the
// name of the credential status that asks for the secret (ADR-0043), so a
// flow with a marker tells the person which credential to create. The value
// itself is never kept.
const SecretMarker = "NEEDS_SECRET"

// FindSecretValue reports a field whose literal value looked like a secret
// and was replaced by [SecretMarker]. The message names the field, never the
// value.
const FindSecretValue FindingCode = "SECRET_VALUE"

// errSecretLiteral marks a condition whose comparison value looks like a
// secret. Such a condition cannot be kept as a comparison.
var errSecretLiteral = fmt.Errorf("%w: a comparison value looks like a secret", errNotConverted)

// secretNames are the normalized name fragments of a field that holds a secret.
// A header, a parameter or a field called like one of them is a secret.
var secretNames = []string{
	"token", "secret", "password", "passwd", "apikey", "bearer",
	"authorization", "privatekey", "accesskey",
}

// Value shapes that are secrets wherever they appear.
var (
	bearerRE    = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{8,}`)
	vendorKeyRE = regexp.MustCompile(`\b(?:sk|pk|rk|ghp|gho|xox[abprs])[-_][A-Za-z0-9_-]{12,}|\bAKIA[0-9A-Z]{16}\b`)
	// Query parameters named like a secret (for example token or api_key) keep
	// their name and lose their value.
	urlSecretRE = regexp.MustCompile(`(?i)([?&][^=&#\s]*(?:token|secret|password|passwd|apikey|api_key|api-key|auth)[^=&#\s]*=)([^&#\s]+)`)
	uuidRE      = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	wordRE      = regexp.MustCompile(`\S+`)
)

// secretKey reports whether a field name says it holds a secret.
func secretKey(name string) bool {
	n := strings.NewReplacer("-", "", "_", "", " ", "", ".", "").Replace(strings.ToLower(name))
	for _, frag := range secretNames {
		if strings.Contains(n, frag) {
			return true
		}
	}
	return false
}

// looksHighEntropy reports whether a word is long, mixes letters and digits,
// is not an identifier or a URL, and has the randomness of a key.
func looksHighEntropy(w string) bool {
	if utf8.RuneCountInString(w) < 24 || strings.Contains(w, "://") || uuidRE.MatchString(w) {
		return false
	}
	var letters, digits bool
	counts := map[rune]int{}
	for _, r := range w {
		counts[r]++
		switch {
		case r >= '0' && r <= '9':
			digits = true
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			letters = true
		}
	}
	if !letters || !digits {
		return false
	}
	n := float64(utf8.RuneCountInString(w))
	var entropy float64
	for _, c := range counts {
		p := float64(c) / n
		entropy -= p * math.Log2(p)
	}
	return entropy >= 3.5
}

// redactText replaces the secret-looking parts of a string and reports
// whether it changed anything.
func redactText(s string) (string, bool) {
	out := s
	out = urlSecretRE.ReplaceAllString(out, "${1}"+SecretMarker)
	out = bearerRE.ReplaceAllString(out, SecretMarker)
	out = vendorKeyRE.ReplaceAllString(out, SecretMarker)
	out = wordRE.ReplaceAllStringFunc(out, func(w string) string {
		if looksHighEntropy(w) {
			return SecretMarker
		}
		return w
	})
	return out, out != s
}

// redactor walks the n8n parameters of one node and keeps only what is safe
// to store: literal secrets become markers and each one is reported by field.
type redactor struct {
	node     *Node
	findings []Finding
}

func (r *redactor) report(path string) {
	r.findings = append(r.findings, Finding{
		Code: FindSecretValue, NodeID: r.node.ID, Node: r.node.Name, Field: path,
		Message: "the value of " + path + " looks like a secret and is not kept; it is replaced by " +
			SecretMarker + ". Create the credential in Orcon and bind it to this node",
	})
}

// literal reports whether v is a string that is not an n8n expression.
func literal(v jsonvalue.Value) (string, bool) {
	s, ok := v.Str()
	return s, ok && !strings.HasPrefix(s, "=")
}

// value returns v without its secrets. The path names the field for the
// report; key is the name of the member v sits under, if any.
func (r *redactor) value(path string, v jsonvalue.Value) jsonvalue.Value {
	switch v.Kind() {
	case jsonvalue.KindString:
		s, _ := v.Str()
		if lit, ok := literal(v); ok && secretKeyOfPath(path) && lit != "" {
			r.report(path)
			return jsonvalue.String(SecretMarker)
		}
		out, changed := redactText(s)
		if changed {
			r.report(path)
		}
		return jsonvalue.String(out)
	case jsonvalue.KindArray:
		items := v.Items()
		out := make([]jsonvalue.Value, len(items))
		for i, item := range items {
			out[i] = r.value(fmt.Sprintf("%s[%d]", path, i), item)
		}
		return jsonvalue.Array(out...)
	case jsonvalue.KindObject:
		return r.object(path, v)
	case jsonvalue.KindInvalid, jsonvalue.KindNull, jsonvalue.KindBool, jsonvalue.KindNumber:
		return v
	default:
		return v
	}
}

// object redacts the members of an object. A {name, value} pair such as an
// HTTP header or a query parameter is checked by its name: a secret name makes
// the value a secret.
func (r *redactor) object(path string, v jsonvalue.Value) jsonvalue.Value {
	pairName := strValue(v, "name")
	members := v.Members()
	out := make([]jsonvalue.Member, 0, len(members))
	for _, m := range members {
		childPath := joinPath(path, m.Key)
		switch {
		case m.Key == "value" && pairName != "" && secretKey(pairName) && isLiteral(m.Value):
			r.report(childPath)
			out = append(out, jsonvalue.Member{Key: m.Key, Value: jsonvalue.String(SecretMarker)})
		case secretKey(m.Key) && isLiteral(m.Value):
			r.report(childPath)
			out = append(out, jsonvalue.Member{Key: m.Key, Value: jsonvalue.String(SecretMarker)})
		default:
			out = append(out, jsonvalue.Member{Key: m.Key, Value: r.value(childPath, m.Value)})
		}
	}
	obj, err := jsonvalue.Object(out...)
	if err != nil {
		// Keys come from a parsed object, so they are unique; keep the
		// original shape if that ever changes.
		return v
	}
	return obj
}

// joinPath appends a member name to a field path; the root has no name.
func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// isLiteral reports whether v is a non-empty string that is not an expression.
func isLiteral(v jsonvalue.Value) bool {
	s, ok := literal(v)
	return ok && s != ""
}

// secretKeyOfPath reports whether the last member name of a path is a secret
// name. Array indexes are skipped.
func secretKeyOfPath(path string) bool {
	last := path
	if i := strings.LastIndex(path, "."); i >= 0 {
		last = path[i+1:]
	}
	if i := strings.Index(last, "["); i >= 0 {
		last = last[:i]
	}
	return last != "" && secretKey(last)
}

// redactNode returns the n8n node object with its parameters cleaned of
// secrets, and the findings that say which fields were replaced. The node's
// own fields (name, type, id, credential references) are copied as they are:
// only the parameters carry values an author typed.
func redactNode(n *Node) (jsonvalue.Value, []Finding) {
	if n.Source.IsZero() {
		return jsonvalue.Value{}, nil
	}
	r := &redactor{node: n}
	members := n.Source.Members()
	out := make([]jsonvalue.Member, 0, len(members))
	for _, m := range members {
		if m.Key == "parameters" {
			out = append(out, jsonvalue.Member{Key: m.Key, Value: r.value("parameters", m.Value)})
			continue
		}
		out = append(out, m)
	}
	clean, err := jsonvalue.Object(out...)
	if err != nil {
		return jsonvalue.Value{}, nil
	}
	return clean, r.findings
}
