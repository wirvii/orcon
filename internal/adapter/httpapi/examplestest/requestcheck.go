// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package examplestest

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// RequestChecker checks that a request is one the contract describes: it
// belongs to an operation, carries that operation's required parameters and
// headers, and has a body that validates against the request schema. Scenario
// tests use it to prove that the state they seed is asked for in the words of
// the contract.
type RequestChecker struct {
	c      *contract
	prefix string
	routes []route
}

type route struct {
	method   string
	path     string
	template *regexp.Regexp
	pointer  string
	op       map[string]any
	item     map[string]any
}

// ErrRequestMismatch means a request does not follow the contract.
var ErrRequestMismatch = errors.New("request does not follow the contract")

// NewRequestChecker reads a bundled contract; apiPrefix is the server URL path.
func NewRequestChecker(contractJSON []byte, apiPrefix string) (*RequestChecker, error) {
	c, err := loadContract("contract", contractJSON)
	if err != nil {
		return nil, err
	}
	rc := &RequestChecker{c: c, prefix: apiPrefix}
	paths, _ := c.root["paths"].(map[string]any)
	for _, p := range sortedKeys(paths) {
		item, _ := paths[p].(map[string]any)
		for _, method := range httpMethods {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			expr := "^" + regexp.QuoteMeta(apiPrefix) + regexp.MustCompile(`\\\{[^/]*\\\}`).ReplaceAllString(regexp.QuoteMeta(p), `[^/]+`) + "$"
			rc.routes = append(rc.routes, route{
				method: strings.ToUpper(method), path: p, template: regexp.MustCompile(expr),
				pointer: "/paths/" + escapePointerToken(p) + "/" + method, op: op, item: item,
			})
		}
	}
	return rc, nil
}

// Check returns an error naming what is wrong with the request, or nil.
func (rc *RequestChecker) Check(method, path string, query url.Values, header http.Header, body []byte) error {
	var r *route
	for i := range rc.routes {
		if rc.routes[i].method == method && rc.routes[i].template.MatchString(path) {
			r = &rc.routes[i]
			break
		}
	}
	if r == nil {
		return fmt.Errorf("%w: %s %s is not an operation of the contract", ErrRequestMismatch, method, path)
	}
	params := append(listOf(r.item["parameters"]), listOf(r.op["parameters"])...)
	for _, raw := range params {
		p, _, err := rc.c.resolve(raw, r.pointer+"/parameters")
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrRequestMismatch, r.op["operationId"], err)
		}
		required, _ := p["required"].(bool)
		name, _ := p["name"].(string)
		switch p["in"] {
		case "header":
			if required && header.Get(name) == "" {
				return fmt.Errorf("%w: %s needs the header %s", ErrRequestMismatch, r.op["operationId"], name)
			}
		case "query":
			if required && !query.Has(name) {
				return fmt.Errorf("%w: %s needs the query parameter %s", ErrRequestMismatch, r.op["operationId"], name)
			}
		}
	}
	for name := range query {
		if !hasQueryParam(rc.c, r, params, name) {
			return fmt.Errorf("%w: %s has no query parameter %s", ErrRequestMismatch, r.op["operationId"], name)
		}
	}
	return rc.checkBody(r, header, body)
}

func hasQueryParam(c *contract, r *route, params []any, name string) bool {
	for _, raw := range params {
		p, _, err := c.resolve(raw, r.pointer+"/parameters")
		if err == nil && p["in"] == "query" && p["name"] == name {
			return true
		}
	}
	return false
}

func listOf(v any) []any {
	l, _ := v.([]any)
	return l
}

func (rc *RequestChecker) checkBody(r *route, header http.Header, body []byte) error {
	reqBody, ptr, ok, err := rc.c.child(r.op, r.pointer, "requestBody")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRequestMismatch, err)
	}
	if !ok {
		if len(body) > 0 {
			return fmt.Errorf("%w: %s takes no body", ErrRequestMismatch, r.op["operationId"])
		}
		return nil
	}
	if len(body) == 0 {
		if required, _ := reqBody["required"].(bool); required {
			return fmt.Errorf("%w: %s needs a body", ErrRequestMismatch, r.op["operationId"])
		}
		return nil
	}
	media, _, _ := strings.Cut(header.Get("Content-Type"), ";")
	content, _ := reqBody["content"].(map[string]any)
	mt, ok := content[strings.TrimSpace(media)]
	if !ok {
		return fmt.Errorf("%w: %s does not accept the media type %q", ErrRequestMismatch, r.op["operationId"], media)
	}
	mtObj, mptr, err := rc.c.resolve(mt, ptr+"/content/"+escapePointerToken(strings.TrimSpace(media)))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRequestMismatch, err)
	}
	if _, has := mtObj["schema"]; !has {
		return nil
	}
	schema, err := rc.c.compile(mptr + "/schema")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRequestMismatch, err)
	}
	decoded, err := jsonschema.UnmarshalJSON(strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("%w: the body is not JSON: %w", ErrRequestMismatch, err)
	}
	if err := schema.Validate(decoded); err != nil {
		return fmt.Errorf("%w: %s body: %w", ErrRequestMismatch, r.op["operationId"], err)
	}
	return nil
}
