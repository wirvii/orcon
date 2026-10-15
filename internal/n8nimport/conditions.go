// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// errNotConverted marks a construct the converter does not translate yet. The
// message says what it is; the caller turns it into a finding.
var errNotConverted = errors.New("not converted")

// filterJS translates the filter of an n8n IF (version 2) or Switch (version
// 3) rule into one boolean JavaScript expression. The filter is the object
// {options, conditions, combinator}. Every operator the translation does not
// know fails the whole filter: an approximated condition would route items
// the wrong way without a trace (SPEC-003 R-IMP-01, D7b).
func filterJS(filter jsonvalue.Value) (string, error) {
	items, ok := filter.Get("conditions")
	if !ok || items.Kind() != jsonvalue.KindArray || items.Len() == 0 {
		return "", fmt.Errorf("%w: the filter has no conditions", errNotConverted)
	}
	caseSensitive := true
	if opts, ok := filter.Get("options"); ok {
		if v, ok := opts.Get("caseSensitive"); ok {
			if b, isBool := v.Bool(); isBool {
				caseSensitive = b
			}
		}
	}
	joiner := " && "
	if c, _ := filter.Get("combinator"); c.Kind() == jsonvalue.KindString {
		s, _ := c.Str()
		switch s {
		case "", "and":
		case "or":
			joiner = " || "
		default:
			return "", fmt.Errorf("%w: combinator %q", errNotConverted, s)
		}
	}
	parts := make([]string, 0, items.Len())
	for _, item := range items.Items() {
		js, err := conditionJS(item, caseSensitive)
		if err != nil {
			return "", err
		}
		parts = append(parts, "("+js+")")
	}
	return strings.Join(parts, joiner), nil
}

// conditionJS translates one condition {leftValue, rightValue, operator}.
func conditionJS(c jsonvalue.Value, caseSensitive bool) (string, error) {
	op, ok := c.Get("operator")
	if !ok || op.Kind() != jsonvalue.KindObject {
		return "", fmt.Errorf("%w: a condition has no operator", errNotConverted)
	}
	typ := strValue(op, "type")
	operation := strValue(op, "operation")
	left, err := operandJS(c, "leftValue")
	if err != nil {
		return "", err
	}
	single := operationIsSingle(typ, operation)
	var right string
	if !single {
		right, err = operandJS(c, "rightValue")
		if err != nil {
			return "", err
		}
	}
	if !caseSensitive && typ == "string" {
		left = "String(" + left + ").toLowerCase()"
		if !single {
			right = "String(" + right + ").toLowerCase()"
		}
	}
	js, ok := operatorJS(typ, operation, left, right)
	if !ok {
		return "", fmt.Errorf("%w: the operator %s.%s", errNotConverted, typ, operation)
	}
	return js, nil
}

// operatorJS maps one operator of the string, number, boolean and any types.
// Dates, arrays, objects and regular expressions are not translated yet.
func operatorJS(typ, operation, l, r string) (string, bool) {
	switch typ {
	case "string":
		switch operation {
		case "equals":
			return l + " === " + r, true
		case "notEquals":
			return l + " !== " + r, true
		case "contains":
			return "String(" + l + ").includes(String(" + r + "))", true
		case "notContains":
			return "!String(" + l + ").includes(String(" + r + "))", true
		case "startsWith":
			return "String(" + l + ").startsWith(String(" + r + "))", true
		case "notStartsWith":
			return "!String(" + l + ").startsWith(String(" + r + "))", true
		case "endsWith":
			return "String(" + l + ").endsWith(String(" + r + "))", true
		case "notEndsWith":
			return "!String(" + l + ").endsWith(String(" + r + "))", true
		case "isEmpty":
			return emptyJS(l), true
		case "isNotEmpty":
			return "!(" + emptyJS(l) + ")", true
		}
	case "number":
		switch operation {
		case "equals":
			return "Number(" + l + ") === Number(" + r + ")", true
		case "notEquals":
			return "Number(" + l + ") !== Number(" + r + ")", true
		case "gt":
			return "Number(" + l + ") > Number(" + r + ")", true
		case "gte":
			return "Number(" + l + ") >= Number(" + r + ")", true
		case "lt":
			return "Number(" + l + ") < Number(" + r + ")", true
		case "lte":
			return "Number(" + l + ") <= Number(" + r + ")", true
		}
	case "boolean":
		switch operation {
		case "true":
			return l + " === true", true
		case "false":
			return l + " === false", true
		case "equals":
			return l + " === " + r, true
		case "notEquals":
			return l + " !== " + r, true
		}
	case "any":
		switch operation {
		case "exists":
			return "(" + l + " !== undefined && " + l + " !== null)", true
		case "notExists":
			return "(" + l + " === undefined || " + l + " === null)", true
		}
	}
	return "", false
}

// emptyJS is true for a missing, null or empty-string value.
func emptyJS(v string) string {
	return "(" + v + " === undefined || " + v + " === null || " + v + " === \"\")"
}

// operationIsSingle reports whether the operator takes no right-hand value.
func operationIsSingle(typ, operation string) bool {
	switch {
	case typ == "string" && (operation == "isEmpty" || operation == "isNotEmpty"):
		return true
	case typ == "boolean" && (operation == "true" || operation == "false"):
		return true
	case typ == "any" && (operation == "exists" || operation == "notExists"):
		return true
	}
	return false
}

// operandJS translates a leftValue or rightValue into a JavaScript operand. A
// string that starts with "=" is an n8n expression: its {{ }} parts are
// evaluated and its literal parts are quoted. Any other value is a literal.
func operandJS(c jsonvalue.Value, key string) (string, error) {
	v, ok := c.Get(key)
	if !ok {
		return "undefined", nil
	}
	switch v.Kind() {
	case jsonvalue.KindString:
		s, _ := v.Str()
		return templateJS(s)
	case jsonvalue.KindNumber:
		text, _ := v.NumberText()
		return text, nil
	case jsonvalue.KindBool:
		b, _ := v.Bool()
		if b {
			return "true", nil
		}
		return "false", nil
	case jsonvalue.KindNull:
		return "null", nil
	case jsonvalue.KindInvalid, jsonvalue.KindArray, jsonvalue.KindObject:
		return "", fmt.Errorf("%w: a %s operand", errNotConverted, v.Kind())
	default:
		return "", fmt.Errorf("%w: a %s operand", errNotConverted, v.Kind())
	}
}

// templateJS translates an n8n string value into a JavaScript operand.
func templateJS(s string) (string, error) {
	if !strings.HasPrefix(s, "=") {
		return quoteJS(s)
	}
	body := s[1:]
	if !strings.Contains(body, "{{") {
		return quoteJS(body)
	}
	var parts []string
	for body != "" {
		open := strings.Index(body, "{{")
		if open < 0 {
			lit, err := quoteJS(body)
			if err != nil {
				return "", err
			}
			parts = append(parts, lit)
			break
		}
		if open > 0 {
			lit, err := quoteJS(body[:open])
			if err != nil {
				return "", err
			}
			parts = append(parts, lit)
		}
		rest := body[open+2:]
		end := strings.Index(rest, "}}")
		if end < 0 {
			return "", fmt.Errorf("%w: an expression is not closed", errNotConverted)
		}
		code := strings.TrimSpace(rest[:end])
		if code == "" {
			return "", fmt.Errorf("%w: an empty expression", errNotConverted)
		}
		parts = append(parts, "("+code+")")
		body = rest[end+2:]
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	return "(\"\" + " + strings.Join(parts, " + ") + ")", nil
}

// quoteJS renders a Go string as a JavaScript string literal. JSON strings are
// valid JavaScript string literals.
func quoteJS(s string) (string, error) {
	b, err := jsonvalue.Marshal(jsonvalue.String(s))
	if err != nil {
		return "", fmt.Errorf("n8nimport: quote: %w", err)
	}
	return string(b), nil
}

// strValue reads a string member; it is empty when the member is missing or
// is not a string.
func strValue(v jsonvalue.Value, key string) string {
	m, ok := v.Get(key)
	if !ok {
		return ""
	}
	s, _ := m.Str()
	return s
}

// whenParam wraps a JavaScript boolean as the n8n-style template of a
// condition parameter: "={{ <js> }}".
func whenParam(js string) string { return "={{ " + js + " }}" }
