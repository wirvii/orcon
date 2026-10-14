// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/authz"
)

// ErrSpec means the contract does not declare what the pipeline needs.
var ErrSpec = errors.New("invalid API contract")

// Operation is what the request pipeline knows about one operation of the
// contract. It is read from the embedded OpenAPI document and its x-orcon-*
// extensions, never written by hand: permission, rate class and stability
// come from the extensions.
type Operation struct {
	// ID is the operationId.
	ID string
	// Method is the upper-case HTTP method.
	Method string
	// Path is the path as the contract writes it, without the server URL path.
	Path string
	// Pattern is the route pattern registered on the mux: method, server URL
	// path and path.
	Pattern string
	// Requirement is x-orcon-permission.
	Requirement authz.Requirement
	// Class is x-orcon-rate-class.
	Class middleware.Class
	// Stability is x-orcon-stability: stable, beta or experimental.
	Stability string
	// Protocol is true for operations tagged protocol, whose shape an external
	// standard fixes.
	Protocol bool
	// Public is true when the operation needs no authentication: it declares an
	// empty security list or the anonymous permission.
	Public bool
	// IdempotencyKey is "required", "optional" or empty (no Idempotency-Key header).
	IdempotencyKey string
	// IfMatchRequired is true when the operation declares a required If-Match header.
	IfMatchRequired bool
	// IssuesSecret is true when a 2xx response is a <Resource>WithSecret schema.
	IssuesSecret bool
	// PathParams are the names of the path parameters in order of appearance.
	PathParams []string
	// ExperimentalFeature is x-orcon-experimental-feature: the name Orcon-Experimental must
	// carry to use an experimental operation; empty for the others.
	ExperimentalFeature string
	// HasBody is true when the operation declares a request body.
	HasBody bool
	// BodyRequired is true when the request body is required.
	BodyRequired bool
	// BodyMedia maps each media type the body accepts (lower case) to the JSON
	// pointer of its schema, or "" when it declares none.
	BodyMedia map[string]string
	// Description is the description of the operation.
	Description string
}

// Spec is the parsed contract.
type Spec struct {
	// BasePath is the path of the first server URL, for example /api/v1.
	BasePath string
	// Operations are the operations in a stable order (path, then method).
	Operations []Operation

	doc map[string]any
}

// ParseSpec reads the operations of a bundled contract in JSON form.
func ParseSpec(raw []byte) (*Spec, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSpec, err)
	}
	s := &Spec{doc: doc}
	servers, _ := doc["servers"].([]any)
	if len(servers) == 0 {
		return nil, fmt.Errorf("%w: no server URL", ErrSpec)
	}
	first, _ := servers[0].(map[string]any)
	rawURL, _ := first["url"].(string)
	u, err := url.Parse(rawURL)
	if err != nil || rawURL == "" {
		return nil, fmt.Errorf("%w: server URL %q", ErrSpec, rawURL)
	}
	s.BasePath = strings.TrimRight(u.Path, "/")

	paths, _ := doc["paths"].(map[string]any)
	names := make([]string, 0, len(paths))
	for p := range paths {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		item, _ := paths[p].(map[string]any)
		for _, method := range []string{"get", "put", "post", "patch", "delete", "head", "options"} {
			opDoc, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			op, err := s.operation(p, method, item, opDoc)
			if err != nil {
				return nil, err
			}
			s.Operations = append(s.Operations, op)
		}
	}
	return s, nil
}

func (s *Spec) operation(path, method string, item, doc map[string]any) (Operation, error) {
	op := Operation{Method: strings.ToUpper(method), Path: path}
	op.ID, _ = doc["operationId"].(string)
	where := fmt.Sprintf("%s %s", op.Method, path)
	if op.ID == "" {
		return op, fmt.Errorf("%w: %s has no operationId", ErrSpec, where)
	}
	op.Pattern = op.Method + " " + s.BasePath + path

	req, err := parseRequirement(doc["x-orcon-permission"])
	if err != nil {
		return op, fmt.Errorf("%w: %s: %w", ErrSpec, where, err)
	}
	op.Requirement = req

	class, _ := doc["x-orcon-rate-class"].(string)
	switch middleware.Class(class) {
	case middleware.ClassRead, middleware.ClassWrite, middleware.ClassRun, middleware.ClassStream:
		op.Class = middleware.Class(class)
	default:
		return op, fmt.Errorf("%w: %s: x-orcon-rate-class %q", ErrSpec, where, class)
	}
	op.Stability, _ = doc["x-orcon-stability"].(string)
	switch op.Stability {
	case "stable", "beta", "experimental":
	default:
		return op, fmt.Errorf("%w: %s: x-orcon-stability %q", ErrSpec, where, op.Stability)
	}
	op.ExperimentalFeature, _ = doc["x-orcon-experimental-feature"].(string)
	if (op.Stability == "experimental") != (op.ExperimentalFeature != "") {
		return op, fmt.Errorf("%w: %s: x-orcon-experimental-feature is required if and only if the operation is experimental", ErrSpec, where)
	}

	tags, _ := doc["tags"].([]any)
	for _, t := range tags {
		if t == "protocol" {
			op.Protocol = true
		}
	}
	// Only the anonymous permission makes an operation public, and only protocol
	// operations and the closed list of ADR-0080 section 4 may use it. An empty
	// security list on its own never opens an operation: the two must agree, or the
	// contract is refused (fail closed, BL-088).
	sec, hasSecurity := doc["security"].([]any)
	emptySecurity := hasSecurity && len(sec) == 0
	switch {
	case req.Mode == authz.ModeAnonymous:
		if !op.Protocol && !anonymousOperation(op.ID) {
			return op, fmt.Errorf("%w: %s: the anonymous permission is for protocol operations and %v only", ErrSpec, where, anonymousOperations)
		}
		if !emptySecurity {
			return op, fmt.Errorf("%w: %s: an anonymous operation declares security: []", ErrSpec, where)
		}
		op.Public = true
	case emptySecurity:
		return op, fmt.Errorf("%w: %s: security: [] is only for anonymous operations", ErrSpec, where)
	}

	op.PathParams = pathParams(path)
	params := append(listOf(item["parameters"]), listOf(doc["parameters"])...)
	for _, raw := range params {
		p, err := s.resolveObject(raw)
		if err != nil {
			return op, fmt.Errorf("%w: %s: %w", ErrSpec, where, err)
		}
		if p["in"] != "header" {
			continue
		}
		name, _ := p["name"].(string)
		required, _ := p["required"].(bool)
		switch strings.ToLower(name) {
		case "idempotency-key":
			op.IdempotencyKey = "optional"
			if required {
				op.IdempotencyKey = "required"
			}
		case "if-match":
			op.IfMatchRequired = required
		}
	}
	op.IssuesSecret = s.issuesSecret(doc)
	_, op.HasBody = doc["requestBody"]
	op.Description, _ = doc["description"].(string)
	if op.HasBody {
		if op.BodyRequired, op.BodyMedia, err = s.requestBody(doc, path, method); err != nil {
			return op, fmt.Errorf("%w: %s: %w", ErrSpec, where, err)
		}
	}
	return op, nil
}

// anonymousOperations is the closed list of non-protocol operations that need no
// authentication (ADR-0080 section 4 as amended by SPEC-016).
var anonymousOperations = []string{"createSession", "getSetupState", "createSetupSession"}

func anonymousOperation(id string) bool { return slices.Contains(anonymousOperations, id) }

// requestBody reads whether the body is required and where the schema of each
// media type lives, as a JSON pointer into the document.
func (s *Spec) requestBody(doc map[string]any, path, method string) (required bool, media map[string]string, err error) {
	pointer := "/paths/" + escapeToken(path) + "/" + method + "/requestBody"
	node := doc["requestBody"]
	for range 16 {
		obj, ok := node.(map[string]any)
		if !ok {
			return false, nil, errors.New("the request body is not an object")
		}
		ref, isRef := obj["$ref"].(string)
		if !isRef {
			break
		}
		target, found := strings.CutPrefix(ref, "#")
		if !found {
			return false, nil, fmt.Errorf("reference %q leaves the document", ref)
		}
		if node, found = s.lookup(target); !found {
			return false, nil, fmt.Errorf("reference %q does not resolve", ref)
		}
		pointer = target
	}
	body, _ := node.(map[string]any)
	required, _ = body["required"].(bool)
	content, _ := body["content"].(map[string]any)
	media = map[string]string{}
	for m, raw := range content {
		mt, _ := raw.(map[string]any)
		if _, has := mt["schema"]; has {
			media[strings.ToLower(m)] = pointer + "/content/" + escapeToken(m) + "/schema"
		} else {
			media[strings.ToLower(m)] = ""
		}
	}
	return required, media, nil
}

// escapeToken escapes one JSON pointer reference token (RFC 6901).
func escapeToken(s string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(s)
}

func listOf(v any) []any {
	l, _ := v.([]any)
	return l
}

// parseRequirement reads x-orcon-permission: a permission name,
// authenticated, anonymous, {allOf: [...]} or {anyOf: [...]}.
func parseRequirement(v any) (authz.Requirement, error) {
	switch x := v.(type) {
	case string:
		switch x {
		case "authenticated":
			return authz.Requirement{Mode: authz.ModeAuthenticated}, nil
		case "anonymous":
			return authz.Requirement{Mode: authz.ModeAnonymous}, nil
		case "":
			return authz.Requirement{}, errors.New("x-orcon-permission is empty")
		default:
			return authz.Requirement{Mode: authz.ModeAll, Permissions: []authz.Permission{authz.Permission(x)}}, nil
		}
	case map[string]any:
		// Exactly one of allOf and anyOf: declaring both, or anything else, is refused so
		// the answer never depends on the iteration order of a map.
		_, hasAll := x["allOf"]
		_, hasAny := x["anyOf"]
		if hasAll == hasAny || len(x) != 1 {
			return authz.Requirement{}, errors.New("x-orcon-permission declares exactly one of allOf and anyOf")
		}
		key, mode := "allOf", authz.ModeAll
		if hasAny {
			key, mode = "anyOf", authz.ModeAny
		}
		list, ok := x[key].([]any)
		if !ok {
			return authz.Requirement{}, fmt.Errorf("x-orcon-permission.%s is not a list", key)
		}
		out := authz.Requirement{Mode: mode}
		for _, item := range list {
			name, ok := item.(string)
			if !ok || name == "" {
				return authz.Requirement{}, fmt.Errorf("x-orcon-permission.%s holds a non-name", key)
			}
			out.Permissions = append(out.Permissions, authz.Permission(name))
		}
		if len(out.Permissions) == 0 {
			return authz.Requirement{}, fmt.Errorf("x-orcon-permission.%s is empty", key)
		}
		return out, nil
	}
	return authz.Requirement{}, errors.New("x-orcon-permission is missing or malformed")
}

// pathParams lists the {name} segments of path.
func pathParams(path string) []string {
	var out []string
	for _, seg := range strings.Split(path, "/") {
		if len(seg) > 2 && seg[0] == '{' && seg[len(seg)-1] == '}' {
			out = append(out, seg[1:len(seg)-1])
		}
	}
	return out
}

// resolveObject follows a $ref into the same document.
func (s *Spec) resolveObject(node any) (map[string]any, error) {
	for range 16 {
		obj, ok := node.(map[string]any)
		if !ok {
			return nil, errors.New("expected an object")
		}
		ref, isRef := obj["$ref"].(string)
		if !isRef {
			return obj, nil
		}
		target, ok := strings.CutPrefix(ref, "#")
		if !ok {
			return nil, fmt.Errorf("reference %q leaves the document", ref)
		}
		next, found := s.lookup(target)
		if !found {
			return nil, fmt.Errorf("reference %q does not resolve", ref)
		}
		node = next
	}
	return nil, errors.New("reference chain is too long")
}

func (s *Spec) lookup(pointer string) (any, bool) {
	var cur any = s.doc
	if pointer == "" {
		return cur, true
	}
	for _, token := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		token = strings.NewReplacer("~1", "/", "~0", "~").Replace(token)
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[token]; !ok {
			return nil, false
		}
	}
	return cur, true
}

// issuesSecret reports whether a 2xx response of the operation is a
// <Resource>WithSecret schema, the only place a generated secret is returned
// (FIELD-7).
func (s *Spec) issuesSecret(doc map[string]any) bool {
	responses, _ := doc["responses"].(map[string]any)
	for status, raw := range responses {
		if !strings.HasPrefix(status, "2") {
			continue
		}
		resp, err := s.resolveObject(raw)
		if err != nil {
			continue
		}
		content, _ := resp["content"].(map[string]any)
		for _, mt := range content {
			m, _ := mt.(map[string]any)
			schema, _ := m["schema"].(map[string]any)
			if ref, _ := schema["$ref"].(string); strings.HasSuffix(ref, "WithSecret") {
				return true
			}
		}
	}
	return false
}
