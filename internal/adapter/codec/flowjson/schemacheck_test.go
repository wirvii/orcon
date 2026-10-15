// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flowjson_test

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// schemaChecker validates a JSON value against the subset of JSON Schema
// (draft 2020-12) that orcon-flow-1.schema.json uses. It exists so the tests
// can prove the published schema accepts and rejects what the Go validator
// does without adding a schema library to the product (dependencies.md
// DEP-5.2). It refuses any keyword it does not implement, so the schema can
// never silently use a rule these tests ignore.
type schemaChecker struct {
	root     jsonvalue.Value
	mu       sync.Mutex // guards patterns; tests run in parallel
	patterns map[string]*regexp.Regexp
}

var supportedKeywords = map[string]bool{
	"$schema": true, "$id": true, "$comment": true, "title": true, "description": true, "$defs": true,
	"$ref": true, "type": true, "properties": true, "required": true, "additionalProperties": true,
	"items": true, "minItems": true, "maxItems": true, "uniqueItems": true, "enum": true, "const": true, "pattern": true, "minLength": true, "maxLength": true, "if": true, "then": true,
	"minimum": true, "maximum": true, "format": true, "oneOf": true, "allOf": true, "not": true, "propertyNames": true,
}

func newSchemaChecker(t *testing.T, data []byte) *schemaChecker {
	t.Helper()
	root, err := jsonvalue.Parse(data)
	if err != nil {
		t.Fatalf("the schema is not valid JSON: %v", err)
	}
	c := &schemaChecker{root: root, patterns: map[string]*regexp.Regexp{}}
	c.checkKeywords(t, root, "#")
	return c
}

// checkKeywords walks the schema and fails on keywords this checker does not
// implement and on references that do not resolve.
func (c *schemaChecker) checkKeywords(t *testing.T, schema jsonvalue.Value, at string) {
	t.Helper()
	if schema.Kind() != jsonvalue.KindObject {
		return
	}
	for _, m := range schema.Members() {
		path := at + "/" + m.Key
		switch m.Key {
		case "properties", "$defs":
			for _, sub := range m.Value.Members() {
				c.checkKeywords(t, sub.Value, path+"/"+sub.Key)
			}
		case "items", "additionalProperties", "not", "propertyNames", "if", "then":
			c.checkKeywords(t, m.Value, path)
		case "oneOf", "allOf":
			for i, sub := range m.Value.Items() {
				c.checkKeywords(t, sub, fmt.Sprintf("%s/%d", path, i))
			}
		case "$ref":
			ref, _ := m.Value.Str()
			if _, ok := c.resolve(ref); !ok {
				t.Errorf("%s: reference %q does not resolve", at, ref)
			}
		default:
			if !supportedKeywords[m.Key] {
				t.Errorf("%s: keyword %q is not supported by the test checker", at, m.Key)
			}
		}
	}
}

func (c *schemaChecker) resolve(ref string) (jsonvalue.Value, bool) {
	rest, ok := strings.CutPrefix(ref, "#/")
	if !ok {
		return jsonvalue.Value{}, false
	}
	cur := c.root
	for _, part := range strings.Split(rest, "/") {
		next, found := cur.Get(part)
		if !found {
			return jsonvalue.Value{}, false
		}
		cur = next
	}
	return cur, true
}

// errors returns the reasons instance does not satisfy the schema.
func (c *schemaChecker) errors(instance jsonvalue.Value) []string {
	return c.validate(c.root, instance, "")
}

func (c *schemaChecker) valid(instance jsonvalue.Value) bool { return len(c.errors(instance)) == 0 }

func (c *schemaChecker) validate(schema, inst jsonvalue.Value, path string) []string {
	var errs []string
	fail := func(format string, args ...any) { errs = append(errs, path+": "+fmt.Sprintf(format, args...)) }

	if ref, ok := schema.Get("$ref"); ok {
		target, _ := ref.Str()
		resolved, _ := c.resolve(target)
		errs = append(errs, c.validate(resolved, inst, path)...)
	}
	if typ, ok := schema.Get("type"); ok {
		want, _ := typ.Str()
		if !matchesType(want, inst) {
			fail("want type %s, found %s", want, inst.Kind())
			return errs // the keywords below assume the type
		}
	}
	if enum, ok := schema.Get("enum"); ok {
		found := false
		for _, e := range enum.Items() {
			found = found || jsonvalue.Equal(e, inst)
		}
		if !found {
			fail("value is not one of the allowed values")
		}
	}
	if want, ok := schema.Get("const"); ok && !jsonvalue.Equal(want, inst) {
		fail("value is not the constant")
	}
	errs = append(errs, c.checkString(schema, inst, path)...)
	errs = append(errs, c.checkNumber(schema, inst, path)...)
	errs = append(errs, c.checkObject(schema, inst, path)...)
	errs = append(errs, c.checkArray(schema, inst, path)...)
	errs = append(errs, c.checkCombinators(schema, inst, path)...)
	return errs
}

func matchesType(want string, v jsonvalue.Value) bool {
	switch want {
	case "object":
		return v.Kind() == jsonvalue.KindObject
	case "array":
		return v.Kind() == jsonvalue.KindArray
	case "string":
		return v.Kind() == jsonvalue.KindString
	case "boolean":
		return v.Kind() == jsonvalue.KindBool
	case "number":
		return v.Kind() == jsonvalue.KindNumber
	case "integer":
		f, ok := v.Float64()
		return ok && f == float64(int64(f))
	default:
		return false
	}
}

func (c *schemaChecker) pattern(p string) *regexp.Regexp {
	c.mu.Lock()
	defer c.mu.Unlock()
	re, ok := c.patterns[p]
	if !ok {
		re = regexp.MustCompile(p)
		c.patterns[p] = re
	}
	return re
}

func (c *schemaChecker) checkString(schema, inst jsonvalue.Value, path string) []string {
	s, isStr := inst.Str()
	if !isStr {
		return nil
	}
	var errs []string
	if p, ok := schema.Get("pattern"); ok {
		pat, _ := p.Str()
		if !c.pattern(pat).MatchString(s) {
			errs = append(errs, fmt.Sprintf("%s: %q does not match %s", path, s, pat))
		}
	}
	if n, ok := intKeyword(schema, "minLength"); ok && utf8.RuneCountInString(s) < n {
		errs = append(errs, fmt.Sprintf("%s: shorter than %d", path, n))
	}
	if n, ok := intKeyword(schema, "maxLength"); ok && utf8.RuneCountInString(s) > n {
		errs = append(errs, fmt.Sprintf("%s: longer than %d", path, n))
	}
	if f, ok := schema.Get("format"); ok {
		if name, _ := f.Str(); name == "date-time" {
			if _, err := time.Parse(time.RFC3339, s); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %q is not a date-time", path, s))
			}
		}
	}
	return errs
}

func intKeyword(schema jsonvalue.Value, key string) (int, bool) {
	v, ok := schema.Get(key)
	if !ok {
		return 0, false
	}
	n, ok := v.Int64()
	return int(n), ok
}

func (c *schemaChecker) checkNumber(schema, inst jsonvalue.Value, path string) []string {
	f, isNum := inst.Float64()
	if !isNum {
		return nil
	}
	if m, ok := schema.Get("maximum"); ok {
		if highest, _ := m.Float64(); f > highest {
			return []string{fmt.Sprintf("%s: %v is above the maximum %v", path, f, highest)}
		}
	}
	if m, ok := schema.Get("minimum"); ok {
		if lowest, _ := m.Float64(); f < lowest {
			return []string{fmt.Sprintf("%s: %v is below the minimum %v", path, f, lowest)}
		}
	}
	return nil
}

func (c *schemaChecker) checkObject(schema, inst jsonvalue.Value, path string) []string {
	if inst.Kind() != jsonvalue.KindObject {
		return nil
	}
	var errs []string
	props, _ := schema.Get("properties")
	if req, ok := schema.Get("required"); ok {
		for _, r := range req.Items() {
			name, _ := r.Str()
			if _, present := inst.Get(name); !present {
				errs = append(errs, fmt.Sprintf("%s: missing required property %q", path, name))
			}
		}
	}
	closed := false
	if ap, ok := schema.Get("additionalProperties"); ok {
		allowed, isBool := ap.Bool()
		closed = isBool && !allowed
	}
	for _, m := range inst.Members() {
		sub, known := props.Get(m.Key)
		switch {
		case known:
			errs = append(errs, c.validate(sub, m.Value, path+"/"+m.Key)...)
		case closed:
			errs = append(errs, fmt.Sprintf("%s: unknown property %q", path, m.Key))
		}
		if names, ok := schema.Get("propertyNames"); ok {
			errs = append(errs, c.validate(names, jsonvalue.String(m.Key), path+"/"+m.Key+"(name)")...)
		}
	}
	return errs
}

func (c *schemaChecker) checkArray(schema, inst jsonvalue.Value, path string) []string {
	if inst.Kind() != jsonvalue.KindArray {
		return nil
	}
	var errs []string
	if n, ok := intKeyword(schema, "minItems"); ok && inst.Len() < n {
		errs = append(errs, fmt.Sprintf("%s: fewer than %d items", path, n))
	}
	if n, ok := intKeyword(schema, "maxItems"); ok && inst.Len() > n {
		errs = append(errs, fmt.Sprintf("%s: more than %d items", path, n))
	}
	if unique, ok := schema.Get("uniqueItems"); ok {
		if want, _ := unique.Bool(); want {
			items := inst.Items()
			for i := range items {
				for j := i + 1; j < len(items); j++ {
					if jsonvalue.Equal(items[i], items[j]) {
						errs = append(errs, fmt.Sprintf("%s: items %d and %d are equal", path, i, j))
					}
				}
			}
		}
	}
	items, ok := schema.Get("items")
	if !ok {
		return errs
	}
	for i, item := range inst.Items() {
		errs = append(errs, c.validate(items, item, fmt.Sprintf("%s/%d", path, i))...)
	}
	return errs
}

func (c *schemaChecker) checkCombinators(schema, inst jsonvalue.Value, path string) []string {
	var errs []string
	if all, ok := schema.Get("allOf"); ok {
		for _, sub := range all.Items() {
			errs = append(errs, c.validate(sub, inst, path)...)
		}
	}
	if one, ok := schema.Get("oneOf"); ok {
		matches := 0
		for _, sub := range one.Items() {
			if len(c.validate(sub, inst, path)) == 0 {
				matches++
			}
		}
		if matches != 1 {
			errs = append(errs, fmt.Sprintf("%s: matches %d of the oneOf alternatives, want exactly 1", path, matches))
		}
	}
	if cond, ok := schema.Get("if"); ok && len(c.validate(cond, inst, path)) == 0 {
		if then, hasThen := schema.Get("then"); hasThen {
			errs = append(errs, c.validate(then, inst, path)...)
		}
	}
	if not, ok := schema.Get("not"); ok && len(c.validate(not, inst, path)) == 0 {
		errs = append(errs, path+": matches a schema it must not match")
	}
	return errs
}
