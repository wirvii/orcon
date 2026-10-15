// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"bytes"
	"fmt"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// ArgumentError reports tool arguments that do not match the tool's schema.
// The engine returns it to the model as an error [ToolResult] and counts it
// as a step; two consecutive invalid calls to the same tool fail the run
// (ADR-0061 §3).
type ArgumentError struct {
	// Path locates the failing value, for example "$.items[2].sku".
	Path   string
	Reason string
}

// Error implements the error interface.
func (e *ArgumentError) Error() string {
	return fmt.Sprintf("arguments at %s: %s", e.Path, e.Reason)
}

// ValidateArguments checks args against schema. Supported keywords are type
// (one of object, array, string, number, integer, boolean, null), properties,
// required, items, enum and additionalProperties set to false. Unknown
// keywords are ignored, so a richer schema still validates its supported part.
// A zero schema accepts every value.
func ValidateArguments(schema, args jsonvalue.Value) error {
	return validateAt(schema, args, "$")
}

func validateAt(schema, v jsonvalue.Value, path string) error {
	if schema.IsZero() {
		return nil
	}
	if want, ok := strKeyword(schema, "type"); ok && !typeMatches(want, v) {
		return &ArgumentError{Path: path, Reason: fmt.Sprintf("want type %s, got %s", want, kindName(v))}
	}
	if err := checkEnum(schema, v, path); err != nil {
		return err
	}
	if v.Kind() == jsonvalue.KindObject {
		return validateObject(schema, v, path)
	}
	if v.Kind() == jsonvalue.KindArray {
		return validateItems(schema, v, path)
	}
	return nil
}

func validateObject(schema, v jsonvalue.Value, path string) error {
	if req, ok := schema.Get("required"); ok {
		for _, name := range req.Items() {
			key, _ := name.Str()
			if _, present := v.Get(key); !present {
				return &ArgumentError{Path: path, Reason: fmt.Sprintf("missing required property %q", key)}
			}
		}
	}
	props, hasProps := schema.Get("properties")
	if hasProps {
		for _, m := range v.Members() {
			sub, declared := props.Get(m.Key)
			if !declared {
				continue
			}
			if err := validateAt(sub, m.Value, path+"."+m.Key); err != nil {
				return err
			}
		}
	}
	if ap, ok := schema.Get("additionalProperties"); ok && ap.Kind() == jsonvalue.KindBool {
		if flag, _ := ap.Bool(); !flag {
			for _, m := range v.Members() {
				if !hasProps {
					return &ArgumentError{Path: path, Reason: fmt.Sprintf("unexpected property %q", m.Key)}
				}
				if _, declared := props.Get(m.Key); !declared {
					return &ArgumentError{Path: path, Reason: fmt.Sprintf("unexpected property %q", m.Key)}
				}
			}
		}
	}
	return nil
}

func validateItems(schema, v jsonvalue.Value, path string) error {
	items, ok := schema.Get("items")
	if !ok {
		return nil
	}
	for i, el := range v.Items() {
		if err := validateAt(items, el, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func checkEnum(schema, v jsonvalue.Value, path string) error {
	enum, ok := schema.Get("enum")
	if !ok {
		return nil
	}
	want, err := jsonvalue.Canonical(v)
	if err != nil {
		return &ArgumentError{Path: path, Reason: "value is not canonical JSON"}
	}
	for _, option := range enum.Items() {
		got, err := jsonvalue.Canonical(option)
		if err != nil {
			return fmt.Errorf("enum option at %s: %w", path, err)
		}
		if bytes.Equal(got, want) {
			return nil
		}
	}
	return &ArgumentError{Path: path, Reason: "value is not one of the allowed options"}
}

func strKeyword(schema jsonvalue.Value, key string) (string, bool) {
	v, ok := schema.Get(key)
	if !ok {
		return "", false
	}
	return v.Str()
}

func typeMatches(want string, v jsonvalue.Value) bool {
	switch want {
	case "object":
		return v.Kind() == jsonvalue.KindObject
	case "array":
		return v.Kind() == jsonvalue.KindArray
	case "string":
		return v.Kind() == jsonvalue.KindString
	case "boolean":
		return v.Kind() == jsonvalue.KindBool
	case "null":
		return v.Kind() == jsonvalue.KindNull
	case "number":
		return v.Kind() == jsonvalue.KindNumber
	case "integer":
		if v.Kind() != jsonvalue.KindNumber {
			return false
		}
		_, ok := v.Int64()
		return ok
	default:
		// An unknown type name cannot be enforced; the keyword is ignored, as
		// documented on ValidateArguments.
		return true
	}
}

func kindName(v jsonvalue.Value) string {
	switch v.Kind() {
	case jsonvalue.KindObject:
		return "object"
	case jsonvalue.KindArray:
		return "array"
	case jsonvalue.KindString:
		return "string"
	case jsonvalue.KindNumber:
		return "number"
	case jsonvalue.KindBool:
		return "boolean"
	case jsonvalue.KindNull:
		return "null"
	case jsonvalue.KindInvalid:
		return "absent"
	default:
		return "unknown"
	}
}
