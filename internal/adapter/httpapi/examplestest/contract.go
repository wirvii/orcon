// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package examplestest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// maxRefDepth bounds $ref chains so a cyclic contract fails instead of looping.
const maxRefDepth = 32

// Defaults of the time limits in section 4.6 of the orcon-api design.
const (
	defaultStreamWithin = 10 * time.Second
	maxStreamWithin     = 10 * time.Second
	maxOperationWait    = 60 * time.Second
)

// contract is a bundled OpenAPI document (packages/openapi/v1/openapi.bundle.json)
// with the helpers the runner needs: reference resolution by JSON pointer and
// schema compilation for response validation.
type contract struct {
	name    string
	root    map[string]any
	raw     []byte
	schemas map[string]*jsonschema.Schema
	comp    *jsonschema.Compiler
}

// exampleDef is one entry of an operation's x-orcon-examples list.
type exampleDef struct {
	OperationID string
	Path        string
	Method      string
	OpPointer   string
	Op          map[string]any
	Name        string
	Scenario    string
	PathParams  map[string]any
	Query       map[string]any
	Headers     map[string]any
	RequestBody string
	Status      int
	ResponseRef string
	Stream      *streamDef
	Operation   *operationDef
}

// streamDef describes an example that opens a server-sent events stream.
type streamDef struct {
	Trigger     string
	ExpectTypes []string
	Within      time.Duration
}

// operationDef describes an example that starts a long-running operation.
type operationDef struct {
	TerminalStatus string
}

// loadContract parses a bundled contract; the name only appears in messages.
func loadContract(name string, raw []byte) (*contract, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse contract %s: %w", name, err)
	}
	root, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("contract %s: the document is not an object", name)
	}
	if _, ok := root["openapi"]; !ok {
		return nil, fmt.Errorf("contract %s: missing the openapi field", name)
	}
	resource, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse contract %s: %w", name, err)
	}
	comp := jsonschema.NewCompiler()
	comp.DefaultDraft(jsonschema.Draft2020)
	if err := comp.AddResource(resourceURL, resource); err != nil {
		return nil, fmt.Errorf("contract %s: %w", name, err)
	}
	return &contract{name: name, root: root, raw: raw, schemas: map[string]*jsonschema.Schema{}, comp: comp}, nil
}

// resourceURL is the identity the contract document has inside the schema compiler.
const resourceURL = "contract.json"

// escapePointerToken escapes one JSON pointer reference token (RFC 6901).
func escapePointerToken(s string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(s)
}

// lookup follows a JSON pointer ("/a/b") from the document root.
func (c *contract) lookup(pointer string) (any, bool) {
	var cur any = c.root
	if pointer == "" {
		return cur, true
	}
	for _, token := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		token = strings.NewReplacer("~1", "/", "~0", "~").Replace(token)
		switch node := cur.(type) {
		case map[string]any:
			next, ok := node[token]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			i, err := strconv.Atoi(token)
			if err != nil || i < 0 || i >= len(node) {
				return nil, false
			}
			cur = node[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// resolve follows $ref chains of node, which sits at pointer, and returns the
// final object and the pointer where it lives. Only references into the same
// document are supported: the bundle has no others.
func (c *contract) resolve(node any, pointer string) (map[string]any, string, error) {
	for range maxRefDepth {
		obj, ok := node.(map[string]any)
		if !ok {
			return nil, pointer, fmt.Errorf("%s: expected an object", pointer)
		}
		ref, isRef := obj["$ref"].(string)
		if !isRef {
			return obj, pointer, nil
		}
		target, found := strings.CutPrefix(ref, "#")
		if !found {
			return nil, pointer, fmt.Errorf("%s: reference %q leaves the bundled document", pointer, ref)
		}
		if unescaped, err := url.PathUnescape(target); err == nil {
			target = unescaped
		}
		next, ok := c.lookup(target)
		if !ok {
			return nil, pointer, fmt.Errorf("%s: reference %q does not resolve", pointer, ref)
		}
		node, pointer = next, target
	}
	return nil, pointer, fmt.Errorf("%s: reference chain is longer than %d", pointer, maxRefDepth)
}

// child returns obj[key] resolved, with its pointer. The second result is false when absent.
func (c *contract) child(obj map[string]any, pointer, key string) (map[string]any, string, bool, error) {
	raw, ok := obj[key]
	if !ok {
		return nil, "", false, nil
	}
	resolved, ptr, err := c.resolve(raw, pointer+"/"+escapePointerToken(key))
	if err != nil {
		return nil, "", false, err
	}
	return resolved, ptr, true, nil
}

// httpMethods lists the path item keys that hold operations.
var httpMethods = []string{"get", "put", "post", "patch", "delete", "head", "options"}

// examples lists every example of every non-exempt operation in a stable order.
func (c *contract) examples() ([]exampleDef, error) {
	paths, _ := c.root["paths"].(map[string]any)
	names := make([]string, 0, len(paths))
	for p := range paths {
		names = append(names, p)
	}
	sort.Strings(names)
	var out []exampleDef
	for _, p := range names {
		item, ok := paths[p].(map[string]any)
		if !ok {
			continue
		}
		for _, method := range httpMethods {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			ptr := "/paths/" + escapePointerToken(p) + "/" + method
			if isProtocolOperation(p, op) {
				continue
			}
			list, _ := op["x-orcon-examples"].([]any)
			for i, raw := range list {
				def, err := parseExample(raw)
				if err != nil {
					return nil, fmt.Errorf("%s %s %s x-orcon-examples[%d]: %w", c.name, strings.ToUpper(method), p, i, err)
				}
				def.Path, def.Method, def.OpPointer, def.Op = p, strings.ToUpper(method), ptr, op
				def.OperationID, _ = op["operationId"].(string)
				out = append(out, def)
			}
		}
	}
	return out, nil
}

// isProtocolOperation reports the operations the runner does not execute: the
// protocol endpoints and the MCP endpoint are covered by their own tests.
func isProtocolOperation(path string, op map[string]any) bool {
	if path == "/mcp" || strings.HasPrefix(path, "/mcp/") {
		return true
	}
	cli, _ := op["x-orcon-cli"].(map[string]any)
	none, _ := cli["none"].(map[string]any)
	return none["category"] == "protocol"
}

func parseExample(raw any) (exampleDef, error) {
	obj, ok := raw.(map[string]any)
	if !ok {
		return exampleDef{}, errors.New("an example must be an object")
	}
	var def exampleDef
	def.Name, _ = obj["name"].(string)
	def.Scenario, _ = obj["scenario"].(string)
	if def.Name == "" {
		return exampleDef{}, errors.New("an example needs a name")
	}
	request, _ := obj["request"].(map[string]any)
	def.PathParams, _ = request["pathParams"].(map[string]any)
	def.Query, _ = request["query"].(map[string]any)
	def.Headers, _ = request["headers"].(map[string]any)
	def.RequestBody, _ = request["body"].(string)
	response, ok := obj["response"].(map[string]any)
	if !ok {
		return exampleDef{}, fmt.Errorf("example %s has no response", def.Name)
	}
	status, err := statusOf(response["status"])
	if err != nil {
		return exampleDef{}, fmt.Errorf("example %s: %w", def.Name, err)
	}
	def.Status = status
	def.ResponseRef, _ = response["body"].(string)
	if s, ok := obj["stream"].(map[string]any); ok {
		sd, err := parseStream(s)
		if err != nil {
			return exampleDef{}, fmt.Errorf("example %s: %w", def.Name, err)
		}
		def.Stream = sd
	}
	if o, ok := obj["operation"].(map[string]any); ok {
		ts, _ := o["terminalStatus"].(string)
		if ts == "" {
			return exampleDef{}, fmt.Errorf("example %s: operation.terminalStatus is required", def.Name)
		}
		def.Operation = &operationDef{TerminalStatus: ts}
	}
	return def, nil
}

func statusOf(v any) (int, error) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, fmt.Errorf("status %q is not an integer", n)
		}
		return int(i), nil
	case string:
		i, err := strconv.Atoi(n)
		if err != nil {
			return 0, fmt.Errorf("status %q is not an integer", n)
		}
		return i, nil
	default:
		return 0, errors.New("response.status is required")
	}
}

func parseStream(s map[string]any) (*streamDef, error) {
	sd := &streamDef{Within: defaultStreamWithin}
	sd.Trigger, _ = s["trigger"].(string)
	if sd.Trigger == "" {
		return nil, errors.New("stream.trigger is required")
	}
	types, _ := s["expectTypes"].([]any)
	for _, t := range types {
		if str, ok := t.(string); ok {
			sd.ExpectTypes = append(sd.ExpectTypes, str)
		}
	}
	if len(sd.ExpectTypes) == 0 {
		return nil, errors.New("stream.expectTypes is required")
	}
	if within, ok := s["within"].(string); ok {
		d, err := time.ParseDuration(within)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("stream.within %q is not a positive duration", within)
		}
		if d > maxStreamWithin {
			return nil, fmt.Errorf("stream.within %s is over the %s limit", within, maxStreamWithin)
		}
		sd.Within = d
	}
	return sd, nil
}

// namedExample finds the value of the OpenAPI example called name under
// container["content"][media]["examples"][name]. The media type is returned.
func (c *contract) namedExample(container map[string]any, pointer, name string) (value any, media string, err error) {
	content, _ := container["content"].(map[string]any)
	medias := make([]string, 0, len(content))
	for m := range content {
		medias = append(medias, m)
	}
	sort.Strings(medias)
	for _, m := range medias {
		mt, mptr, err := c.resolve(content[m], pointer+"/content/"+escapePointerToken(m))
		if err != nil {
			return nil, "", err
		}
		examples, _ := mt["examples"].(map[string]any)
		raw, ok := examples[name]
		if !ok {
			continue
		}
		ex, _, err := c.resolve(raw, mptr+"/examples/"+escapePointerToken(name))
		if err != nil {
			return nil, "", err
		}
		return ex["value"], m, nil
	}
	return nil, "", fmt.Errorf("no example named %q under %s", name, pointer)
}

// requestExample returns the body of the named request example and its media type.
func (c *contract) requestExample(def exampleDef) (any, string, error) {
	body, ptr, ok, err := c.child(def.Op, def.OpPointer, "requestBody")
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", fmt.Errorf("operation %s has no requestBody to hold example %q", def.OperationID, def.RequestBody)
	}
	return c.namedExample(body, ptr, def.RequestBody)
}

// response returns the response object the example expects, with its pointer.
func (c *contract) response(def exampleDef) (map[string]any, string, bool, error) {
	responses, _ := def.Op["responses"].(map[string]any)
	key := strconv.Itoa(def.Status)
	if _, ok := responses[key]; !ok {
		key = strconv.Itoa(def.Status/100) + "XX"
		if _, ok := responses[key]; !ok {
			if _, ok := responses["default"]; !ok {
				return nil, "", false, nil
			}
			key = "default"
		}
	}
	return c.child(responses, def.OpPointer+"/responses", key)
}

// responseSchema returns the pointer of the schema the response declares for
// its JSON media type, or "" when the response declares none.
func (c *contract) responseSchema(resp map[string]any, pointer string) (schemaPointer string, err error) {
	content, _ := resp["content"].(map[string]any)
	medias := make([]string, 0, len(content))
	for m := range content {
		if isJSONMedia(m) {
			medias = append(medias, m)
		}
	}
	sort.Strings(medias)
	if len(medias) == 0 {
		return "", nil
	}
	mt, mptr, err := c.resolve(content[medias[0]], pointer+"/content/"+escapePointerToken(medias[0]))
	if err != nil {
		return "", err
	}
	if _, ok := mt["schema"]; !ok {
		return "", nil
	}
	return mptr + "/schema", nil
}

func isJSONMedia(m string) bool {
	base, _, _ := strings.Cut(m, ";")
	base = strings.TrimSpace(strings.ToLower(base))
	return base == "application/json" || strings.HasSuffix(base, "+json")
}

// compile returns the compiled schema living at pointer, cached.
func (c *contract) compile(pointer string) (*jsonschema.Schema, error) {
	if s, ok := c.schemas[pointer]; ok {
		return s, nil
	}
	u := url.URL{Fragment: pointer}
	s, err := c.comp.Compile(resourceURL + "#" + u.EscapedFragment())
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", pointer, err)
	}
	c.schemas[pointer] = s
	return s, nil
}
