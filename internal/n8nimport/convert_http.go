// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// typeOrconHTTP is the core HTTP node (SPEC-003 Annex B, CN3-dispatch).
const typeOrconHTTP = "orcon.http@1"

// FindNeedsSecret marks a credential the converted node references. Its secret
// is never imported: a person creates the credential in Orcon before the flow
// can run.
const FindNeedsSecret FindingCode = "NEEDS_SECRET"

// convertHTTP maps HTTP Request, version 4 and later, to orcon.http@1. Every
// parameter the node uses must have a literal meaning in Orcon: the method, the
// URL, the query, the headers and the body are literal (templates are allowed
// only in the body), and the credential is a reference, never a value. Anything
// else leaves the node unsupported with a finding for the field.
//
// A write method is UNSAFE and waits for a person's review (ADR-0037 section 6,
// R-IMP-03), with the same reason the connector mapping gives.
func (c *converter) convertHTTP(n *Node) convResult {
	if ver, _ := strconv.ParseFloat(n.TypeVersion, 64); ver < 4 {
		return c.unsupported(n, []Finding{paramFinding(n, "typeVersion",
			"HTTP Request versions 1 to 3 keep their parameters in another shape; only version 4 and later are converted")}, flow.ElementServiceTask)
	}
	// A secret literal anywhere in the node keeps it unsupported; redactNode
	// reports each field and the original stays redacted.
	if _, secrets := redactNode(n); len(secrets) > 0 {
		return c.unsupported(n, nil, flow.ElementServiceTask)
	}
	p := params(n)
	if opts, ok := p.Get("options"); ok && opts.Kind() == jsonvalue.KindObject && len(opts.Members()) > 0 {
		return c.unsupported(n, []Finding{paramFinding(n, "parameters.options",
			"the HTTP Request options (timeout, redirects, response format, proxy) are not converted")}, flow.ElementServiceTask)
	}

	method, problem := httpMethod(n, p)
	if problem != nil {
		return c.unsupported(n, []Finding{*problem}, flow.ElementServiceTask)
	}
	target, problem := httpURL(n, p)
	if problem != nil {
		return c.unsupported(n, []Finding{*problem}, flow.ElementServiceTask)
	}
	target, problem = httpQuery(n, p, target)
	if problem != nil {
		return c.unsupported(n, []Finding{*problem}, flow.ElementServiceTask)
	}
	headers, problem := httpHeaders(n, p)
	if problem != nil {
		return c.unsupported(n, []Finding{*problem}, flow.ElementServiceTask)
	}
	body, hasBody, problem := httpBody(n, p)
	if problem != nil {
		return c.unsupported(n, []Finding{*problem}, flow.ElementServiceTask)
	}
	credential, findings, problem := c.httpCredential(n, p)
	if problem != nil {
		return c.unsupported(n, []Finding{*problem}, flow.ElementServiceTask)
	}

	members := []jsonvalue.Member{
		{Key: "method", Value: jsonvalue.String(method)},
		{Key: "url", Value: jsonvalue.String(target)},
	}
	if len(headers) > 0 {
		hv, err := jsonvalue.Object(headers...)
		if err != nil {
			return c.unsupported(n, []Finding{paramFinding(n, "parameters.headerParameters", err.Error())}, flow.ElementServiceTask)
		}
		members = append(members, jsonvalue.Member{Key: "headers", Value: hv})
	}
	if hasBody {
		members = append(members, jsonvalue.Member{Key: "body", Value: body})
	}
	if credential != "" {
		members = append(members, jsonvalue.Member{Key: "credential", Value: jsonvalue.String(credential)})
	}
	parameters, err := jsonvalue.Object(members...)
	if err != nil {
		return c.unsupported(n, []Finding{paramFinding(n, "parameters", err.Error())}, flow.ElementServiceTask)
	}

	if !isReadMethod(method) {
		findings = append(findings, Finding{Code: FindReviewRequired, NodeID: n.ID, Node: n.Name, Field: "parameters.method",
			Message: flow.ReasonHTTPWrite + ": " + method + " changes remote state without an idempotency key, so the node is UNSAFE until a person reviews it"})
	}
	reasons := (&Mapped{Findings: findings}).reviewReasons()
	return convResult{
		node: flow.Node{
			Element: flow.ElementServiceTask, Type: typeOrconHTTP, Parameters: parameters,
			Ext: importExt(n, reasons),
		},
		orconType: typeOrconHTTP,
		findings:  findings,
	}
}

func isReadMethod(method string) bool { return method == "GET" || method == "HEAD" }

// httpMethod reads the method. When the method is left out, the request is a
// GET, as in n8n. An expression method is unsupported: the contract of the node
// depends on it.
func httpMethod(n *Node, p jsonvalue.Value) (string, *Finding) {
	v, ok := p.Get("method")
	if !ok {
		return "GET", nil
	}
	s, isStr := v.Str()
	if !isStr || strings.HasPrefix(s, "=") || strings.TrimSpace(s) == "" {
		f := paramFinding(n, "parameters.method", "the method must be a literal name such as GET or POST, not an expression")
		return "", &f
	}
	return strings.ToUpper(strings.TrimSpace(s)), nil
}

// httpURL reads the URL, which must be a literal absolute address.
func httpURL(n *Node, p jsonvalue.Value) (string, *Finding) {
	s, ok := stringParam(p, "url")
	if !ok || s == "" || strings.Contains(s, "{{") {
		f := paramFinding(n, "parameters.url", "the URL must be a literal address, not an expression")
		return "", &f
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		f := paramFinding(n, "parameters.url", "the URL must be an absolute http or https address")
		return "", &f
	}
	return s, nil
}

// httpQuery appends the query parameters to the URL. Only literal parameters
// convert; the order of the n8n list is kept.
func httpQuery(n *Node, p jsonvalue.Value, target string) (string, *Finding) {
	if !boolParam(p, "sendQuery") {
		return target, nil
	}
	if spec, ok := stringParam(p, "specifyQuery"); ok && spec != "keypair" {
		f := paramFinding(n, "parameters.specifyQuery", "only the key and value query list is converted; the JSON query is not")
		return "", &f
	}
	pairs, problem := keyValueList(n, p, "queryParameters")
	if problem != nil {
		return "", problem
	}
	if len(pairs) == 0 {
		return target, nil
	}
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}
	parts := make([]string, 0, len(pairs))
	for _, kv := range pairs {
		parts = append(parts, url.QueryEscape(kv.key)+"="+url.QueryEscape(kv.value))
	}
	return target + sep + strings.Join(parts, "&"), nil
}

// httpHeaders reads the headers as a literal object. A header that repeats a
// name is unsupported: the node would send one of the two values silently.
func httpHeaders(n *Node, p jsonvalue.Value) ([]jsonvalue.Member, *Finding) {
	if !boolParam(p, "sendHeaders") {
		return nil, nil
	}
	if spec, ok := stringParam(p, "specifyHeaders"); ok && spec != "keypair" {
		f := paramFinding(n, "parameters.specifyHeaders", "only the key and value header list is converted; the JSON headers are not")
		return nil, &f
	}
	pairs, problem := keyValueList(n, p, "headerParameters")
	if problem != nil {
		return nil, problem
	}
	seen := map[string]bool{}
	out := make([]jsonvalue.Member, 0, len(pairs))
	for _, kv := range pairs {
		key := strings.ToLower(kv.key)
		if seen[key] {
			f := paramFinding(n, "parameters.headerParameters", fmt.Sprintf("the header %s is set more than once", kv.key))
			return nil, &f
		}
		seen[key] = true
		out = append(out, jsonvalue.Member{Key: kv.key, Value: jsonvalue.String(kv.value)})
	}
	return out, nil
}

// httpBody reads the body. Only a JSON body converts: a JSON text, or a list of
// key and value pairs sent as JSON. Form, multipart and raw bodies do not.
func httpBody(n *Node, p jsonvalue.Value) (jsonvalue.Value, bool, *Finding) {
	if !boolParam(p, "sendBody") {
		return jsonvalue.Value{}, false, nil
	}
	if ct, ok := stringParam(p, "contentType"); ok && ct != "json" {
		f := paramFinding(n, "parameters.contentType", "only a JSON body is converted; the "+ct+" body is not")
		return jsonvalue.Value{}, false, &f
	}
	spec, _ := stringParam(p, "specifyBody")
	switch spec {
	case "json":
		text, ok := stringParam(p, "jsonBody")
		if !ok || text == "" {
			f := paramFinding(n, "parameters.jsonBody", "the JSON body must be a literal JSON text, not an expression")
			return jsonvalue.Value{}, false, &f
		}
		v, err := jsonvalue.Parse([]byte(text))
		if err != nil {
			f := paramFinding(n, "parameters.jsonBody", "the JSON body is not valid JSON: "+err.Error())
			return jsonvalue.Value{}, false, &f
		}
		return v, true, nil
	case "keypair", "":
		pairs, problem := keyValueList(n, p, "bodyParameters")
		if problem != nil {
			return jsonvalue.Value{}, false, problem
		}
		if len(pairs) == 0 {
			return jsonvalue.Value{}, false, nil
		}
		members := make([]jsonvalue.Member, 0, len(pairs))
		for _, kv := range pairs {
			members = append(members, jsonvalue.Member{Key: kv.key, Value: jsonvalue.String(kv.value)})
		}
		v, err := jsonvalue.Object(members...)
		if err != nil {
			f := paramFinding(n, "parameters.bodyParameters", err.Error())
			return jsonvalue.Value{}, false, &f
		}
		return v, true, nil
	default:
		f := paramFinding(n, "parameters.specifyBody", "the body mode "+spec+" is not converted")
		return jsonvalue.Value{}, false, &f
	}
}

// httpCredential turns the node's authentication into a credential reference.
// The secret stays in n8n: the reference carries the credential's name, and a
// NEEDS_SECRET finding asks a person to create it in Orcon.
func (c *converter) httpCredential(n *Node, p jsonvalue.Value) (string, []Finding, *Finding) {
	auth, _ := stringParam(p, "authentication")
	if auth == "" || auth == "none" {
		if len(n.Credentials) > 0 {
			// A credential the node does not use is ambiguous; dropping it could
			// send the request without the authentication its author intended.
			f := paramFinding(n, "parameters.authentication", "the node names a credential but its authentication is not set, so the intent is unclear")
			return "", nil, &f
		}
		return "", nil, nil
	}
	if auth != "predefinedCredentialType" && auth != "genericCredentialType" {
		f := paramFinding(n, "parameters.authentication", "the authentication "+auth+" is not converted")
		return "", nil, &f
	}
	if len(n.Credentials) != 1 {
		f := paramFinding(n, "credentials", "the node must name exactly one credential for its authentication")
		return "", nil, &f
	}
	ref := n.Credentials[0]
	if ref.Name == "" {
		f := paramFinding(n, "credentials."+ref.Type, "the credential "+ref.Type+" has no name to refer to")
		return "", nil, &f
	}
	orcon, ok := c.cat.Credential(ref.Type)
	if !ok {
		f := paramFinding(n, "credentials."+ref.Type, "no Orcon credential type is declared for "+ref.Type)
		return "", nil, &f
	}
	needs := Finding{Code: FindNeedsSecret, NodeID: n.ID, Node: n.Name, Field: "credentials." + ref.Type,
		Message: "create the credential " + ref.Name + " of Orcon type " + orcon + " before the flow runs; its secret is not imported"}
	return ref.Name, []Finding{needs}, nil
}

type pair struct{ key, value string }

// keyValueList reads an n8n list of {name, value} pairs under the given
// parameter. Names and values must be literal, and a name may not repeat in a
// list that becomes a query or a header.
func keyValueList(n *Node, p jsonvalue.Value, key string) ([]pair, *Finding) {
	holder, ok := p.Get(key)
	if !ok {
		return nil, nil
	}
	if holder.Kind() != jsonvalue.KindObject {
		f := paramFinding(n, "parameters."+key, "the list must be an object with a parameters array")
		return nil, &f
	}
	list, ok := holder.Get("parameters")
	if !ok {
		return nil, nil
	}
	if list.Kind() != jsonvalue.KindArray {
		f := paramFinding(n, "parameters."+key+".parameters", "the list of parameters must be an array")
		return nil, &f
	}
	out := make([]pair, 0, list.Len())
	for i, it := range list.Items() {
		field := fmt.Sprintf("parameters.%s.parameters[%d]", key, i)
		name, ok := stringOf(it, "name")
		if !ok || name == "" || strings.HasPrefix(name, "=") {
			f := paramFinding(n, field+".name", "a name must be a literal, non-empty text")
			return nil, &f
		}
		value, ok := it.Get("value")
		if !ok {
			f := paramFinding(n, field+".value", "the entry "+name+" has no value")
			return nil, &f
		}
		text, isStr := value.Str()
		if !isStr {
			text, isStr = numberOrBoolText(value)
		}
		if !isStr || strings.HasPrefix(text, "=") || strings.Contains(text, "{{") {
			f := paramFinding(n, field+".value", "the value of "+name+" must be a literal, not an expression")
			return nil, &f
		}
		out = append(out, pair{key: name, value: text})
	}
	return out, nil
}

// numberOrBoolText renders a literal number or boolean as the text n8n sends.
func numberOrBoolText(v jsonvalue.Value) (string, bool) {
	if b, ok := v.Bool(); ok {
		return strconv.FormatBool(b), true
	}
	if s, ok := v.NumberText(); ok {
		return s, true
	}
	return "", false
}

func boolParam(p jsonvalue.Value, key string) bool {
	v, ok := p.Get(key)
	if !ok {
		return false
	}
	b, _ := v.Bool()
	return b
}
