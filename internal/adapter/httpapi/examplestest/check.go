// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package examplestest

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// uuidPattern matches the UUID text form the contract uses for identifiers.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isUUID(s string) bool { return uuidPattern.MatchString(s) }

func isDateTime(s string) bool {
	_, err := time.Parse(time.RFC3339Nano, s)
	return err == nil
}

// minSubstringBinding is the shortest binding key that is also replaced inside
// longer strings; shorter keys (an ETag such as "7") only match a whole value.
const minSubstringBinding = 16

// bind replaces the fixed values of the examples with the real ones.
func (b Bindings) bind(s string) string {
	if bound, ok := b[s]; ok {
		return bound
	}
	keys := make([]string, 0, len(b))
	for k := range b {
		if len(k) >= minSubstringBinding {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, k := range keys {
		s = strings.ReplaceAll(s, k, b[k])
	}
	return s
}

// bindValue applies bind to every string of a decoded JSON value.
func (b Bindings) bindValue(v any) any {
	switch n := v.(type) {
	case string:
		return b.bind(n)
	case map[string]any:
		out := make(map[string]any, len(n))
		for k, val := range n {
			out[k] = b.bindValue(val)
		}
		return out
	case []any:
		out := make([]any, len(n))
		for i, val := range n {
			out[i] = b.bindValue(val)
		}
		return out
	default:
		return v
	}
}

// envelopeKeys are the top-level keys of a wrapper around the resource.
var envelopeKeys = map[string]bool{"data": true, "meta": true, "error": true, "errors": true, "status": true, "success": true, "message": true}

// isEnvelope reports a response wrapped in an envelope object (responses.md):
// an object with a data member whose other members are wrapper keys, where the
// schema declares no data property.
func (c *contract) isEnvelope(got any, schema map[string]any, schemaPtr string) bool {
	obj, ok := got.(map[string]any)
	if !ok {
		return false
	}
	if _, has := obj["data"]; !has {
		return false
	}
	for k := range obj {
		if !envelopeKeys[k] {
			return false
		}
	}
	_, _, declared := c.property(schema, schemaPtr, "data")
	return !declared
}

// validate checks body against the schema compiled at pointer. It returns the
// JSON pointer of the first offending instance and a reason, or "" when valid.
func (c *contract) validate(schemaPointer string, body any) (instance, reason string, err error) {
	schema, err := c.compile(schemaPointer)
	if err != nil {
		return "", "", err
	}
	verr := schema.Validate(body)
	if verr == nil {
		return "", "", nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(verr, &ve) {
		return "", verr.Error(), nil
	}
	unit := ve.BasicOutput()
	for i := range unit.Errors {
		u := unit.Errors[i]
		if u.Error != nil {
			return u.InstanceLocation, u.Error.String(), nil
		}
	}
	return "", ve.Error(), nil
}

// diff is the first difference between the example and the real response.
type diff struct {
	pointer string
	reason  string
}

// compare walks the leaves of want (the example, bindings applied) and checks
// them against got. The schema argument, which may be nil, is the response schema
// node at schemaPtr; it is used to find x-orcon-volatile fields.
func (c *contract) compare(want, got any, schema map[string]any, schemaPtr, ptr string) *diff {
	if schema != nil {
		resolved, rptr, err := c.resolve(schema, schemaPtr)
		if err != nil {
			schema = nil
		} else {
			schema, schemaPtr = resolved, rptr
		}
	}
	if schema != nil && schema["x-orcon-volatile"] == true {
		return sameKind(want, got, ptr)
	}
	switch w := want.(type) {
	case map[string]any:
		return c.compareObject(w, got, schema, schemaPtr, ptr)
	case []any:
		return c.compareArray(w, got, schema, schemaPtr, ptr)
	case string:
		return compareString(w, got, ptr)
	default:
		return compareScalar(want, got, ptr)
	}
}

// compareScalar compares numbers, booleans and null: the JSON kind first, so
// 1 never equals "1", true never equals "true" and null never equals the text
// "<nil>"; then the value.
func compareScalar(want, got any, ptr string) *diff {
	if kindOf(want) != kindOf(got) {
		return &diff{ptr, fmt.Sprintf("want %s %v, got %s %v", kindOf(want), want, kindOf(got), got)}
	}
	if kindOf(want) == "number" {
		if numbersEqual(want, got) {
			return nil
		}
	} else if want == got {
		return nil
	}
	return &diff{ptr, fmt.Sprintf("want %v, got %v", want, got)}
}

// numbersEqual compares two JSON numbers by value, so 1 and 1.0 are equal.
func numbersEqual(a, b any) bool {
	na, okA := a.(json.Number)
	nb, okB := b.(json.Number)
	if !okA || !okB {
		return fmt.Sprint(a) == fmt.Sprint(b)
	}
	if na == nb {
		return true
	}
	fa, errA := na.Float64()
	fb, errB := nb.Float64()
	return errA == nil && errB == nil && fa == fb
}

func (c *contract) compareObject(want map[string]any, got any, schema map[string]any, schemaPtr, ptr string) *diff {
	g, ok := got.(map[string]any)
	if !ok {
		return &diff{ptr, "want an object, got " + kindOf(got)}
	}
	for _, k := range sortedKeys(want) {
		gv, present := g[k]
		if !present {
			return &diff{ptr + "/" + escapePointerToken(k), "field is missing in the response"}
		}
		sub, subPtr, _ := c.property(schema, schemaPtr, k)
		if d := c.compare(want[k], gv, sub, subPtr, ptr+"/"+escapePointerToken(k)); d != nil {
			return d
		}
	}
	return nil
}

func (c *contract) compareArray(want []any, got any, schema map[string]any, schemaPtr, ptr string) *diff {
	g, ok := got.([]any)
	if !ok {
		return &diff{ptr, "want an array, got " + kindOf(got)}
	}
	if len(g) != len(want) {
		return &diff{ptr, fmt.Sprintf("want %d items, got %d", len(want), len(g))}
	}
	var items map[string]any
	itemsPtr := ""
	if schema != nil {
		if it, ok := schema["items"].(map[string]any); ok {
			items, itemsPtr = it, schemaPtr+"/items"
		}
	}
	for i := range want {
		if d := c.compare(want[i], g[i], items, itemsPtr, fmt.Sprintf("%s/%d", ptr, i)); d != nil {
			return d
		}
	}
	return nil
}

func compareString(want string, got any, ptr string) *diff {
	gs, ok := got.(string)
	if !ok {
		return &diff{ptr, "want a string, got " + kindOf(got)}
	}
	if want == gs {
		return nil
	}
	// A uuid or date-time the scenario did not bind is generated by the server:
	// only its format is compared.
	if (isUUID(want) && isUUID(gs)) || (isDateTime(want) && isDateTime(gs)) {
		return nil
	}
	return &diff{ptr, fmt.Sprintf("want %q, got %q", want, gs)}
}

// sameKind is the comparison of a volatile field: present and of the same JSON kind.
func sameKind(want, got any, ptr string) *diff {
	if kindOf(want) != kindOf(got) {
		return &diff{ptr, fmt.Sprintf("volatile field: want %s, got %s", kindOf(want), kindOf(got))}
	}
	return nil
}

func kindOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	default:
		return "number"
	}
}

// property finds the schema of the named property, looking through allOf.
func (c *contract) property(schema map[string]any, schemaPtr, name string) (map[string]any, string, bool) {
	return c.propertyDepth(schema, schemaPtr, name, 0)
}

func (c *contract) propertyDepth(schema map[string]any, schemaPtr, name string, depth int) (map[string]any, string, bool) {
	if schema == nil || depth > maxRefDepth {
		return nil, "", false
	}
	resolved, rptr, err := c.resolve(schema, schemaPtr)
	if err != nil {
		return nil, "", false
	}
	schema, schemaPtr = resolved, rptr
	if props, ok := schema["properties"].(map[string]any); ok {
		if p, ok := props[name].(map[string]any); ok {
			return p, schemaPtr + "/properties/" + escapePointerToken(name), true
		}
	}
	if all, ok := schema["allOf"].([]any); ok {
		for i, part := range all {
			pm, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if p, ptr, found := c.propertyDepth(pm, fmt.Sprintf("%s/allOf/%d", schemaPtr, i), name, depth+1); found {
				return p, ptr, true
			}
		}
	}
	return nil, "", false
}
