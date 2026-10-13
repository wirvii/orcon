// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// credential reads one credential type (ADR-0029 section 4).
func (r *reader) credential(o *obj) Credential {
	c := Credential{
		ID:          o.pattern("id", "credential id", credentialIDPattern, true),
		DisplayName: o.text("displayName", maxShortText, true),
		Test:        o.pattern("test", "operation name", operationNamePattern, false),
		N8nTypes:    o.strings("n8nTypes"),
	}
	o.each("fields", true, func(el *obj, _ int) {
		c.Fields = append(c.Fields, CredentialField{
			Name:      el.pattern("name", "field name", fieldNamePattern, true),
			Type:      enumMember(el, "type", FieldTypes(), true),
			Sensitive: el.boolean("sensitive"),
			Required:  el.boolean("required"),
		})
	})
	if s, ok := o.child("scheme", true); ok {
		c.Scheme = r.scheme(s)
		s.done()
	}
	return c
}

// scheme reads the authentication scheme; each type accepts only its own
// members (ADR-0030 section 4).
func (r *reader) scheme(o *obj) Scheme {
	s := Scheme{Type: enumMember(o, "type", SchemeTypes(), true)}
	switch s.Type {
	case SchemeAPIKey:
		s.In = enumMember(o, "in", Placements(), true)
		s.Name = o.text("name", maxShortText, true)
		s.Prefix = o.text("prefix", maxShortText, false)
	case SchemeOAuth2:
		s.Grant = enumMember(o, "grant", OAuthGrants(), true)
		s.AuthURL = o.webURL("authUrl", s.Grant != GrantClientCredentials)
		s.TokenURL = o.webURL("tokenUrl", true)
		s.Scopes = o.strings("scopes")
		s.TokenPlacement = enumMember(o, "tokenPlacement", []string{"HEADER", "QUERY", "BODY"}, false)
	case SchemeCustomHeaders:
		s.Headers = r.headerTemplates(o)
	case SchemeAWSSigV4:
		s.Service = o.text("service", maxShortText, true)
		s.Region = o.text("region", maxShortText, true)
	case SchemeSessionToken:
		s.Login = o.pattern("login", "operation name", operationNamePattern, true)
		s.Extract = o.text("extract", maxShortText, true)
		s.TTL = o.pattern("ttl", "ISO 8601 duration", durationPattern, true)
		s.Placement = enumMember(o, "placement", []string{"HEADER", "QUERY", "BODY"}, true)
	case SchemeNone, SchemeBasic, SchemeBearer:
		// These schemes carry no further members.
	}
	return s
}

// headerTemplates reads the "headers" object of CUSTOM_HEADERS: header name to
// template.
func (r *reader) headerTemplates(o *obj) []HeaderTemplate {
	h, ok := o.child("headers", true)
	if !ok {
		return nil
	}
	var out []HeaderTemplate
	for _, m := range h.v.Members() {
		h.used[m.Key] = true
		tpl, isStr := m.Value.Str()
		if !isStr || tpl == "" {
			r.failf(RuleShape, h.at(m.Key), "must be a non-empty template string")
			continue
		}
		out = append(out, HeaderTemplate{Name: m.Key, Template: tpl})
	}
	if len(out) == 0 {
		r.failf(RuleShape, h.path, "must declare at least one header")
	}
	return out
}

// permissions reads what the connector asks an administrator to approve.
func (r *reader) permissions(o *obj) Permissions {
	p := Permissions{
		Credentials:         o.strings("credentials"),
		CredentialRawAccess: o.boolean("credentialRawAccess"),
	}
	if n, ok := o.child("network", false); ok {
		p.Network = r.network(n)
		n.done()
	}
	if c, ok := o.child("resources", false); ok {
		mem, _ := c.integer("memoryMiB", false, math.MaxInt32)
		concurrent, _ := c.integer("maxConcurrent", false, math.MaxInt32)
		p.Resources = Resources{
			MemoryMiB:     int(mem),
			Timeout:       c.pattern("timeout", "ISO 8601 duration", durationPattern, false),
			MaxConcurrent: int(concurrent),
		}
		c.done()
	}
	return p
}

func (r *reader) network(o *obj) Network {
	n := Network{
		AllowPrivateNetworks: o.boolean("allowPrivateNetworks"),
		FromCredential:       o.strings("fromCredential"),
	}
	n.Domains = o.strings("domains")
	for i, d := range n.Domains {
		if msg := checkDomain(d, n.AllowPrivateNetworks); msg != "" {
			r.failf(RulePermission, fmt.Sprintf("%s/%d", o.at("domains"), i), "%s", msg)
		}
	}
	if ports, ok := o.array("ports", false); ok {
		for i, p := range ports {
			port, isInt := p.Int64()
			if !isInt || port < 1 || port > 65535 {
				r.failf(RuleValue, fmt.Sprintf("%s/%d", o.at("ports"), i), "must be a TCP port from 1 to 65535")
				continue
			}
			n.Ports = append(n.Ports, int(port))
		}
	}
	n.Schemes = o.strings("schemes")
	for i, s := range n.Schemes {
		// URL schemes keep their standard lowercase spelling (ADR-0029 section 10).
		if s != "https" && s != "http" {
			r.failf(RuleValue, fmt.Sprintf("%s/%d", o.at("schemes"), i), "%q is not one of https, http", s)
		}
	}
	return n
}

// publicSuffixes are multi-label public suffixes a wildcard must never cover.
// It approximates the Public Suffix List for the common cases; catalog
// admission applies the full list (SPEC-011).
var publicSuffixes = []string{
	"co.uk", "org.uk", "ac.uk", "gov.uk", "me.uk", "com.au", "net.au", "org.au", "com.br", "net.br", "com.co", "net.co",
	"co.jp", "ne.jp", "or.jp", "co.in", "net.in", "org.in", "com.mx", "com.ar", "com.cn", "com.tw", "com.hk", "com.sg",
	"co.nz", "co.za", "co.kr", "com.tr", "github.io", "gitlab.io", "herokuapp.com", "appspot.com", "web.app", "firebaseapp.com",
	"cloudfront.net", "azurewebsites.net", "netlify.app", "vercel.app", "pages.dev", "workers.dev", "blogspot.com",
}

// checkDomain returns why a domain pattern may not be granted, or "" when it
// may: an exact lowercase host, or one leading "*." label over a host that is
// not itself a public suffix (ADR-0029 section 4, MF-6).
func checkDomain(d string, allowPrivate bool) string {
	if !domainPatternOK(d) {
		return fmt.Sprintf("%q is not a host name or a single leading *. wildcard over one; write lowercase labels such as api.example.com or *.example.com", d)
	}
	base, wildcard := strings.CutPrefix(d, "*.")
	labels := strings.Count(base, ".") + 1
	switch {
	case wildcard && (labels < 2 || slices.Contains(publicSuffixes, base)):
		return fmt.Sprintf("the wildcard %q would cover a public suffix; a wildcard needs a registered domain such as *.example.com", d)
	case !wildcard && labels < 2 && !allowPrivate:
		return fmt.Sprintf("%q is a single label; list the full host name, or ask for allowPrivateNetworks for a self-hosted service", d)
	}
	return ""
}

func domainPatternOK(d string) bool {
	if len(d) > 253 || strings.Contains(d, "**") {
		return false
	}
	return domainRegexp.MatchString(d)
}

func (r *reader) rateLimit(o *obj) RateLimit {
	rl := RateLimit{Name: o.pattern("name", "rate limit name", rateLimitPattern, true)}
	rl.Key = o.text("key", maxShortText, true)
	if rl.Key != "" && rl.Key != RateKeyCredential && rl.Key != RateKeyHost && !strings.HasPrefix(rl.Key, "=") {
		r.failf(RuleEnum, o.at("key"), "%q is not one of %s, %s or an expression starting with =", rl.Key, RateKeyCredential, RateKeyHost)
	}
	rl.RequestsPerSecond = o.positiveNumber("requestsPerSecond")
	burst, _ := o.integer("burst", false, math.MaxInt32)
	conc, _ := o.integer("concurrency", false, math.MaxInt32)
	rl.Burst, rl.Concurrency = int(burst), int(conc)
	if rl.RequestsPerSecond == 0 && rl.Concurrency == 0 {
		if _, ok := o.v.Get("requestsPerSecond"); !ok {
			r.failf(RuleShape, o.at("requestsPerSecond"), "a rate limit needs requestsPerSecond or concurrency")
		}
	}
	if h, ok := o.child("respectHeaders", false); ok {
		rh := &RespectHeaders{
			RetryAfter: h.text("retryAfter", maxShortText, false),
			Remaining:  h.text("remaining", maxShortText, false),
		}
		if c, ok := h.child("resetAt", false); ok {
			rh.ResetAt = &ResetAt{Header: c.text("header", maxShortText, true), Format: enumMember(c, "format", ResetFormats(), true)}
			c.done()
		}
		h.done()
		rl.RespectHeaders = rh
	}
	return rl
}
