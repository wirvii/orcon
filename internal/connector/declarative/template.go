// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// ErrTemplate is returned when a template cannot be rendered: an unknown
// placeholder, a missing parameter or a value that is not a scalar.
var ErrTemplate = errors.New("declarative: template")

// scope is what a template may read. Credentials are nil outside the
// authentication step, so a placeholder over $credentials cannot leak a secret
// into a request body, a query or a response mapping.
type scope struct {
	params      map[string]jsonvalue.Value
	credentials map[string]string
}

const (
	openTag  = "{{"
	closeTag = "}}"
)

// render expands every {{...}} placeholder of tpl to text.
func (s scope) render(tpl string) (string, error) {
	var b strings.Builder
	rest := tpl
	for {
		start := strings.Index(rest, openTag)
		if start < 0 {
			b.WriteString(rest)
			return b.String(), nil
		}
		end := strings.Index(rest[start:], closeTag)
		if end < 0 {
			return "", fmt.Errorf("%w: unclosed placeholder", ErrTemplate)
		}
		b.WriteString(rest[:start])
		placeholder := strings.TrimSpace(rest[start+len(openTag) : start+end])
		text, err := s.lookupText(placeholder)
		if err != nil {
			return "", err
		}
		b.WriteString(text)
		rest = rest[start+end+len(closeTag):]
	}
}

// renderValue is like render, except that a template made of one parameter
// placeholder and nothing else yields the parameter's JSON value unchanged.
func (s scope) renderValue(tpl string) (jsonvalue.Value, error) {
	placeholder, ok := singlePlaceholder(tpl)
	if ok && strings.HasPrefix(placeholder, "$parameter.") {
		v, found := s.params[strings.TrimPrefix(placeholder, "$parameter.")]
		if !found {
			return jsonvalue.Value{}, fmt.Errorf("%w: parameter %q is missing", ErrTemplate, strings.TrimPrefix(placeholder, "$parameter."))
		}
		return v, nil
	}
	text, err := s.render(tpl)
	if err != nil {
		return jsonvalue.Value{}, err
	}
	return jsonvalue.String(text), nil
}

// renderNode renders a body value: strings are templates (see renderValue),
// arrays and objects are rendered member by member, and other scalars are kept.
func (s scope) renderNode(v jsonvalue.Value) (jsonvalue.Value, error) {
	if text, ok := v.Str(); ok {
		return s.renderValue(text)
	}
	if items := v.Items(); v.Kind() == jsonvalue.KindArray {
		out := make([]jsonvalue.Value, 0, len(items))
		for _, item := range items {
			r, err := s.renderNode(item)
			if err != nil {
				return jsonvalue.Value{}, err
			}
			out = append(out, r)
		}
		return jsonvalue.Array(out...), nil
	}
	if v.Kind() == jsonvalue.KindObject {
		members := v.Members()
		out := make([]jsonvalue.Member, 0, len(members))
		for _, m := range members {
			r, err := s.renderNode(m.Value)
			if err != nil {
				return jsonvalue.Value{}, err
			}
			out = append(out, jsonvalue.Member{Key: m.Key, Value: r})
		}
		obj, err := jsonvalue.Object(out...)
		if err != nil {
			return jsonvalue.Value{}, fmt.Errorf("%w: object member", ErrTemplate)
		}
		return obj, nil
	}
	return v, nil
}

// templateStrings returns every string of a body value, so validation can check
// each template it contains.
func templateStrings(v jsonvalue.Value) []string {
	if s, ok := v.Str(); ok {
		return []string{s}
	}
	var out []string
	for _, item := range v.Items() {
		out = append(out, templateStrings(item)...)
	}
	for _, m := range v.Members() {
		out = append(out, templateStrings(m.Value)...)
	}
	return out
}

func singlePlaceholder(tpl string) (string, bool) {
	t := strings.TrimSpace(tpl)
	if !strings.HasPrefix(t, openTag) || !strings.HasSuffix(t, closeTag) {
		return "", false
	}
	inner := t[len(openTag) : len(t)-len(closeTag)]
	if strings.Contains(inner, openTag) || strings.Contains(inner, closeTag) {
		return "", false
	}
	return strings.TrimSpace(inner), true
}

func (s scope) lookupText(placeholder string) (string, error) {
	switch {
	case strings.HasPrefix(placeholder, "$parameter."):
		name := strings.TrimPrefix(placeholder, "$parameter.")
		v, ok := s.params[name]
		if !ok {
			return "", fmt.Errorf("%w: parameter %q is missing", ErrTemplate, name)
		}
		return scalarText(v)
	case strings.HasPrefix(placeholder, "$credentials."):
		if s.credentials == nil {
			return "", fmt.Errorf("%w: credentials are not visible here", ErrTemplate)
		}
		name := strings.TrimPrefix(placeholder, "$credentials.")
		v, ok := s.credentials[name]
		if !ok {
			return "", fmt.Errorf("%w: credential field %q is missing", ErrTemplate, name)
		}
		return v, nil
	default:
		return "", fmt.Errorf("%w: unknown placeholder %q", ErrTemplate, placeholder)
	}
}

// scalarText is the text of a scalar parameter. Objects and arrays are refused
// so a template never serializes a structure by accident.
func scalarText(v jsonvalue.Value) (string, error) {
	if s, ok := v.Str(); ok {
		return s, nil
	}
	if n, ok := v.NumberText(); ok {
		return n, nil
	}
	if b, ok := v.Bool(); ok {
		if b {
			return "true", nil
		}
		return "false", nil
	}
	if v.Kind() == jsonvalue.KindNull {
		return "", nil
	}
	return "", fmt.Errorf("%w: a parameter used in text must be a scalar", ErrTemplate)
}

// referencesCredentials reports whether a template reads $credentials.
func referencesCredentials(tpl string) bool {
	return strings.Contains(tpl, "$credentials")
}
