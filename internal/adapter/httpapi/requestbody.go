// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
)

// maxFieldErrors bounds the field errors one answer lists.
const maxFieldErrors = 20

// specResource is the name the contract has inside the schema compiler.
const specResource = "spec.json"

// bodyValidator enforces the request body of one operation against the contract:
// the media type, strict JSON without trailing data, and the request schema of
// that media type, with its closed objects, types, nullability and constraints.
// Generated code decodes whatever it is given; this is what makes the contract
// the rule (BL-090).
type bodyValidator struct {
	op      *Operation
	schemas map[string]*jsonschema.Schema // media type to compiled schema (nil: no schema declared)
	accept  []string
}

// newBodyValidators compiles the request schemas of every operation with a body.
func newBodyValidators(spec *Spec, raw []byte) (map[string]*bodyValidator, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSpec, err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.AssertFormat()
	if err := compiler.AddResource(specResource, doc); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSpec, err)
	}
	out := map[string]*bodyValidator{}
	for i := range spec.Operations {
		op := &spec.Operations[i]
		if !op.HasBody {
			continue
		}
		v := &bodyValidator{op: op, schemas: map[string]*jsonschema.Schema{}}
		for media, pointer := range op.BodyMedia {
			v.accept = append(v.accept, media)
			if pointer == "" {
				v.schemas[media] = nil
				continue
			}
			u := url.URL{Fragment: pointer}
			schema, err := compiler.Compile(specResource + "#" + u.EscapedFragment())
			if err != nil {
				return nil, fmt.Errorf("%w: %s: compile the request schema: %w", ErrSpec, op.ID, err)
			}
			v.schemas[media] = schema
		}
		out[op.ID] = v
	}
	return out, nil
}

// wrap returns the middleware of the validator. It runs after authorization, so
// an unauthenticated caller learns nothing about the schema.
func (v *bodyValidator) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := readAll(r)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				problem.Write(w, r, problem.PayloadTooLarge)
				return
			}
			problem.Write(w, r, problem.ValidationFailed, problem.Detail("The body could not be read."))
			return
		}
		if len(bytes.TrimSpace(data)) == 0 {
			if v.op.BodyRequired {
				problem.Write(w, r, problem.ValidationFailed, problem.Fields(problem.FieldError{
					Pointer: "", Code: "required", Message: "The request body is required.",
				}))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(data))
			next.ServeHTTP(w, r)
			return
		}
		media, _, perr := mime.ParseMediaType(r.Header.Get("Content-Type"))
		schema, accepted := v.schemas[strings.ToLower(media)]
		if perr != nil || !accepted {
			problem.Write(w, r, problem.UnsupportedMediaType,
				problem.Detail("This operation accepts "+strings.Join(sortedStrings(v.accept), ", ")+"."))
			return
		}
		value, reason := decodeStrict(data)
		if reason != "" {
			problem.Write(w, r, problem.ValidationFailed, problem.Fields(problem.FieldError{
				Pointer: "", Code: "invalid-json", Message: reason,
			}))
			return
		}
		if schema != nil {
			if fields := validationFields(schema.Validate(value)); len(fields) > 0 {
				problem.Write(w, r, problem.ValidationFailed, problem.Fields(fields...))
				return
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(data))
		next.ServeHTTP(w, r)
	})
}

func readAll(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read the request body: %w", err)
	}
	return data, nil
}

// decodeStrict decodes exactly one JSON value; anything after it is refused. It
// returns the reason as a sentence for the client, empty when the body is fine.
// Numbers keep their text so a large integer is validated as written.
func decodeStrict(data []byte) (value any, reason string) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return nil, "The body is not valid JSON."
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, "The body has data after the first JSON value."
	}
	return value, ""
}

// keywordCode names a failed schema keyword for the client. The message never
// repeats the offending value: request bodies can carry secrets.
var keywordCodes = map[string]string{
	"additionalProperties": "unknown-field", "unevaluatedProperties": "unknown-field", "required": "required",
	"type": "wrong-type", "enum": "not-allowed", "const": "not-allowed", "format": "bad-format", "pattern": "bad-format",
	"maxLength": "too-long", "minLength": "too-short", "maxItems": "too-many-items", "minItems": "too-few-items",
	"maximum": "out-of-range", "minimum": "out-of-range", "exclusiveMaximum": "out-of-range", "exclusiveMinimum": "out-of-range",
	"uniqueItems": "duplicate-items", "maxProperties": "too-many-members", "minProperties": "too-few-members",
}

// validationFields turns a schema validation error into field errors.
func validationFields(err error) []problem.FieldError {
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []problem.FieldError{{Pointer: "", Code: "invalid", Message: "The body does not satisfy the contract."}}
	}
	var out []problem.FieldError
	collectFields(ve, &out)
	if len(out) == 0 {
		out = append(out, problem.FieldError{Pointer: "", Code: "invalid", Message: "The body does not satisfy the contract."})
	}
	return out
}

// collectFields walks the failures of a validation down to its leaves.
func collectFields(ve *jsonschema.ValidationError, out *[]problem.FieldError) {
	if len(*out) >= maxFieldErrors {
		return
	}
	if len(ve.Causes) > 0 {
		for _, cause := range ve.Causes {
			collectFields(cause, out)
		}
		return
	}
	var sb strings.Builder
	for _, token := range ve.InstanceLocation {
		sb.WriteString("/" + escapeToken(token))
	}
	location := sb.String()
	// Name the member for unknown and missing fields: names are not values.
	switch k := ve.ErrorKind.(type) {
	case *kind.AdditionalProperties:
		for _, name := range k.Properties {
			*out = append(*out, problem.FieldError{Pointer: location + "/" + escapeToken(name), Code: "unknown-field", Message: "The contract has no such field."})
		}
		return
	case *kind.Required:
		for _, name := range k.Missing {
			*out = append(*out, problem.FieldError{Pointer: location + "/" + escapeToken(name), Code: "required", Message: "The field is required."})
		}
		return
	}
	keyword := ""
	if path := ve.ErrorKind.KeywordPath(); len(path) > 0 {
		keyword = path[len(path)-1]
	}
	code, ok := keywordCodes[keyword]
	if !ok {
		code = "invalid"
	}
	*out = append(*out, problem.FieldError{
		Pointer: location, Code: code,
		Message: "The value does not satisfy the " + keyword + " rule of the contract.",
	})
}

func sortedStrings(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return out
}

// bodyMiddleware returns the validation step of op, or nil when it has no body.
func bodyMiddleware(validators map[string]*bodyValidator, op *Operation) middleware.Middleware {
	v, ok := validators[op.ID]
	if !ok {
		return nil
	}
	return v.wrap
}
