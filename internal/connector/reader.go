// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Rules name the kind of problem an [Issue] reports, so tests, tools and
// reviewers can refer to them.
const (
	// RuleShape marks a member that is missing, unknown or of the wrong JSON type.
	RuleShape = "SHAPE"
	// RuleEnum marks a value outside a closed UPPER_SNAKE value set.
	RuleEnum = "ENUM"
	// RuleValue marks a value that is out of range or badly formed.
	RuleValue = "VALUE"
	// RuleReference marks a name that points at nothing.
	RuleReference = "REFERENCE"
	// RuleIdentity marks an identifier the connector may not use.
	RuleIdentity = "IDENTITY"
	// RuleSafety marks a safety contract that breaks a rule of ADR-0029 section 5.
	RuleSafety = "SAFETY"
	// RulePermission marks a permission the manifest may not ask for.
	RulePermission = "PERMISSION"
)

// maxIssues bounds the findings of one read, so hostile input cannot make the
// report grow without limit.
const maxIssues = 100

// Issue is one finding of reading or validating a manifest.
type Issue struct {
	// Rule is one of the Rule constants.
	Rule string
	// Path is a JSON Pointer (RFC 6901) to the offending part of the document.
	Path    string
	Message string
}

// String renders the issue on one line, naming the field and the rule.
func (i Issue) String() string {
	return fmt.Sprintf("%s at %s: %s", i.Rule, i.Path, i.Message)
}

// ManifestError is returned when a document is not a valid manifest. It lists
// every problem found, up to a fixed bound.
type ManifestError struct {
	Issues []Issue
}

// Error implements error.
func (e *ManifestError) Error() string {
	parts := make([]string, 0, min(len(e.Issues), 5))
	for _, i := range e.Issues[:min(len(e.Issues), 5)] {
		parts = append(parts, i.String())
	}
	more := ""
	if len(e.Issues) > 5 {
		more = fmt.Sprintf(" (and %d more)", len(e.Issues)-5)
	}
	return fmt.Sprintf("invalid connector manifest: %s%s", strings.Join(parts, "; "), more)
}

// Has reports whether the error carries an issue of the rule at the path.
func (e *ManifestError) Has(rule, path string) bool {
	return slices.ContainsFunc(e.Issues, func(i Issue) bool { return i.Rule == rule && i.Path == path })
}

type reader struct {
	issues []Issue
}

func (r *reader) failf(rule, path, format string, args ...any) {
	if len(r.issues) >= maxIssues {
		return
	}
	r.issues = append(r.issues, Issue{Rule: rule, Path: path, Message: fmt.Sprintf(format, args...)})
}

// obj reads the members of one JSON object and reports the ones nobody asked
// for when it is closed with done.
type obj struct {
	r    *reader
	path string
	v    jsonvalue.Value
	used map[string]bool
}

func (r *reader) object(path string, v jsonvalue.Value) (*obj, bool) {
	if v.Kind() != jsonvalue.KindObject {
		r.failf(RuleShape, path, "must be an object, found %s", v.Kind())
		return nil, false
	}
	return &obj{r: r, path: path, v: v, used: map[string]bool{}}, true
}

func (o *obj) at(key string) string { return o.path + "/" + escapePointer(key) }

func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func (o *obj) get(key string, required bool) (jsonvalue.Value, bool) {
	o.used[key] = true
	v, ok := o.v.Get(key)
	if !ok && required {
		o.r.failf(RuleShape, o.at(key), "is required")
	}
	return v, ok
}

// str reads a string member; an empty string is a problem.
func (o *obj) str(key string, required bool) string {
	v, ok := o.get(key, required)
	if !ok {
		return ""
	}
	s, isStr := v.Str()
	switch {
	case !isStr:
		o.r.failf(RuleShape, o.at(key), "must be a string, found %s", v.Kind())
	case s == "":
		o.r.failf(RuleShape, o.at(key), "must not be empty")
	}
	return s
}

func (o *obj) boolean(key string) bool {
	v, ok := o.get(key, false)
	if !ok {
		return false
	}
	b, isBool := v.Bool()
	if !isBool {
		o.r.failf(RuleShape, o.at(key), "must be a boolean, found %s", v.Kind())
	}
	return b
}

// integer reads an integer member from 1 to highest. It reports whether the
// member was present and valid.
func (o *obj) integer(key string, required bool, highest int64) (int64, bool) {
	v, ok := o.get(key, required)
	if !ok {
		return 0, false
	}
	n, isInt := v.Int64()
	if !isInt || n < 1 || n > highest {
		o.r.failf(RuleValue, o.at(key), "must be an integer from 1 to %d", highest)
		return 0, false
	}
	return n, true
}

func (o *obj) child(key string, required bool) (*obj, bool) {
	v, ok := o.get(key, required)
	if !ok {
		return nil, false
	}
	return o.r.object(o.at(key), v)
}

func (o *obj) array(key string, required bool) ([]jsonvalue.Value, bool) {
	v, ok := o.get(key, required)
	if !ok {
		return nil, false
	}
	if v.Kind() != jsonvalue.KindArray {
		o.r.failf(RuleShape, o.at(key), "must be an array, found %s", v.Kind())
		return nil, false
	}
	return v.Items(), true
}

// strings reads an array of non-empty strings.
func (o *obj) strings(key string) []string {
	items, ok := o.array(key, false)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for i, item := range items {
		s, isStr := item.Str()
		if !isStr || s == "" {
			o.r.failf(RuleShape, fmt.Sprintf("%s/%d", o.at(key), i), "must be a non-empty string")
			continue
		}
		out = append(out, s)
	}
	return out
}

// opaque returns a member that must be an object and is kept as written.
func (o *obj) opaque(key string) jsonvalue.Value {
	v, ok := o.get(key, false)
	if !ok {
		return jsonvalue.Value{}
	}
	if v.Kind() != jsonvalue.KindObject {
		o.r.failf(RuleShape, o.at(key), "must be an object, found %s", v.Kind())
		return jsonvalue.Value{}
	}
	return v
}

// done reports every member that was not read: manifests are closed objects.
func (o *obj) done() {
	for _, m := range o.v.Members() {
		if !o.used[m.Key] {
			o.r.failf(RuleShape, o.at(m.Key), "is not a member of this object")
		}
	}
}

// each walks an array of objects, giving each element its path.
func (o *obj) each(key string, required bool, fn func(el *obj, index int)) {
	items, ok := o.array(key, required)
	if !ok {
		return
	}
	for i, item := range items {
		el, isObj := o.r.object(fmt.Sprintf("%s/%d", o.at(key), i), item)
		if !isObj {
			continue
		}
		fn(el, i)
		el.done()
	}
}

// enum reads a required string member and checks it against a closed set. A
// value that matches one of the allowed values when case is ignored gets a
// hint, because lowercase spellings are the common mistake (ADR-0029
// section 10).
func enumMember[T ~string](o *obj, key string, allowed []T, required bool) T {
	s := o.str(key, required)
	if s == "" {
		return ""
	}
	return checkEnum(o.r, o.at(key), s, allowed)
}

func checkEnum[T ~string](r *reader, path, s string, allowed []T) T {
	if slices.Contains(allowed, T(s)) {
		return T(s)
	}
	names := make([]string, len(allowed))
	for i, a := range allowed {
		names[i] = string(a)
	}
	hint := ""
	for _, a := range allowed {
		if strings.EqualFold(strings.ReplaceAll(string(a), "_", ""), strings.ReplaceAll(s, "_", "")) {
			hint = fmt.Sprintf("; write it as %q (enumerated values are UPPER_SNAKE)", a)
			break
		}
	}
	r.failf(RuleEnum, path, "%q is not one of %s%s", s, strings.Join(names, ", "), hint)
	return T(s)
}

// pattern checks a string member against a regular expression.
func (o *obj) pattern(key, what string, re *regexp.Regexp, required bool) string {
	s := o.str(key, required)
	if s != "" && !re.MatchString(s) {
		o.r.failf(RuleValue, o.at(key), "%s %q is not valid; it must match %s", what, s, re)
	}
	return s
}

// number reads a positive finite number member.
func (o *obj) positiveNumber(key string) float64 {
	v, ok := o.get(key, false)
	if !ok {
		return 0
	}
	f, isNum := v.Float64()
	if !isNum || f <= 0 || math.IsInf(f, 0) || math.IsNaN(f) {
		o.r.failf(RuleValue, o.at(key), "must be a number greater than zero")
		return 0
	}
	return f
}
