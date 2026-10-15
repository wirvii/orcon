// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

const (
	methodGet  = "GET"
	methodHead = "HEAD"
)

var (
	validMethods      = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"}
	validAuthTypes    = []string{AuthNone, AuthAPIKey, AuthBasic, AuthBearer, AuthOAuth2, AuthCustomHeaders, AuthAWSSigV4, AuthSessionToken}
	validPlacements   = []string{"HEADER", "QUERY", "COOKIE"}
	validBodyTypes    = []string{"JSON", "FORM", "RAW"}
	validEmpty        = []string{"", "NO_ITEMS", "ONE_EMPTY_ITEM"}
	validStrategies   = []string{"OFFSET", "PAGE", "CURSOR", "LINK_HEADER", "NEXT_URL"}
	validSafetyLevels = []string{SafetyReadOnly, SafetyIdempotencyKey, SafetyReconcile, SafetyUnsafe}
)

// Validate reports every reason the interpreter cannot run def safely, joined
// into one error. A nil result means def can be executed. It never reads a
// credential: it only checks the shape of the document.
func Validate(def Definition) error {
	v := &validator{}
	if def.Format != Format {
		v.addf("format", "must be %q, not %q", Format, def.Format)
	}
	v.baseURL(def.HTTP)
	v.auth(def.HTTP.Auth)
	if def.HTTP.Timeout != "" {
		if d, err := time.ParseDuration(def.HTTP.Timeout); err != nil || d <= 0 {
			v.addf("http/timeout", "must be a positive duration such as \"30s\"")
		}
	}
	if len(def.Operations) == 0 {
		v.addf("operations", "a connector needs at least one operation")
	}
	names := make([]string, 0, len(def.Operations))
	for name := range def.Operations {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		v.operation(name, def.Operations[name])
	}
	for _, name := range slices.Sorted(maps.Keys(def.Triggers)) {
		v.trigger("triggers/"+name, def.Triggers[name])
	}
	return v.err()
}

type validator struct {
	problems []error
}

func (v *validator) addf(path, format string, args ...any) {
	v.problems = append(v.problems, fmt.Errorf("%s: %s", path, fmt.Sprintf(format, args...)))
}

func (v *validator) err() error { return errors.Join(v.problems...) }

func (v *validator) baseURL(h HTTP) {
	u, err := url.Parse(h.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		v.addf("http/baseUrl", "must be an absolute https URL")
	}
}

func (v *validator) auth(a Auth) {
	if !slices.Contains(validAuthTypes, a.Type) {
		v.addf("http/auth/type", "%q is not an authentication scheme", a.Type)
		return
	}
	switch a.Type {
	case AuthOAuth2, AuthSessionToken:
		v.addf("http/auth/type", "%s is not supported by this interpreter yet", a.Type)
	case AuthAPIKey:
		if !slices.Contains(validPlacements, a.In) {
			v.addf("http/auth/in", "must be HEADER, QUERY or COOKIE")
		}
		if a.Name == "" {
			v.addf("http/auth/name", "API_KEY needs the name of the header, query parameter or cookie")
		}
	case AuthCustomHeaders:
		if len(a.Headers) == 0 {
			v.addf("http/auth/headers", "CUSTOM_HEADERS needs at least one header")
		}
	case AuthAWSSigV4:
		if a.Service == "" || a.Region == "" {
			v.addf("http/auth", "AWS_SIG_V4 needs service and region")
		}
	}
}

func (v *validator) operation(name string, op Operation) {
	base := "operations/" + name
	if parts := strings.Split(name, "."); len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		v.addf(base, "operation names are <resource>.<verb>")
	}
	v.safety(base, op)
	v.request(base+"/request", op)
	v.response(base+"/response", op)
	v.pagination(base+"/pagination", op)
	if op.MaxAttempts < 0 {
		v.addf(base+"/maxAttempts", "must not be negative")
	}
}

func (v *validator) safety(base string, op Operation) {
	switch op.Purity {
	case PurityRead, PurityEffect:
	default:
		v.addf(base+"/purity", "must be READ or EFFECT, not %q", op.Purity)
	}
	if !slices.Contains(validSafetyLevels, op.Safety) {
		v.addf(base+"/safety", "must be READ_ONLY, IDEMPOTENCY_KEY, RECONCILE or UNSAFE, not %q", op.Safety)
	}
	if op.Purity == PurityRead && op.Safety != SafetyReadOnly {
		v.addf(base+"/safety", "a READ operation is READ_ONLY")
	}
	if op.Purity == PurityEffect && op.Safety == SafetyReadOnly {
		v.addf(base+"/safety", "an EFFECT operation cannot be READ_ONLY")
	}
	if op.Safety == SafetyIdempotencyKey && (op.IdempotencyHeader == "" || op.IdempotencyKey == "") {
		v.addf(base+"/idempotencyKey", "IDEMPOTENCY_KEY needs idempotencyHeader and idempotencyKey")
	}
}

func (v *validator) request(base string, op Operation) {
	r := op.Request
	if !slices.Contains(validMethods, r.Method) {
		v.addf(base+"/method", "%q is not an HTTP method the interpreter sends", r.Method)
	}
	if op.Purity == PurityRead && r.Method != methodGet && r.Method != methodHead {
		v.addf(base+"/method", "a READ operation uses GET or HEAD")
	}
	if !strings.HasPrefix(r.Path, "/") {
		v.addf(base+"/path", "must start with /")
	}
	templates := []string{r.Path, op.IdempotencyKey}
	for _, val := range r.Query {
		templates = append(templates, val)
	}
	for _, val := range r.Headers {
		templates = append(templates, val)
	}
	if r.Body != nil {
		if !slices.Contains(validBodyTypes, r.Body.Type) {
			v.addf(base+"/body/type", "must be JSON, FORM or RAW")
		}
		if r.Body.Type == "RAW" && r.Body.Raw == "" {
			v.addf(base+"/body/raw", "a RAW body needs its text")
		}
		for _, k := range slices.Sorted(maps.Keys(r.Body.Fields)) {
			n := r.Body.Fields[k].Value
			if r.Body.Type == "FORM" {
				if n.Kind() != jsonvalue.KindString {
					v.addf(base+"/body/fields/"+k, "a FORM field must be a string template")
					continue
				}
			}
			templates = append(templates, templateStrings(n)...)
		}
	}
	for _, tpl := range templates {
		if referencesCredentials(tpl) {
			v.addf(base, "$credentials is visible only to the authentication step")
			break
		}
	}
}

func (v *validator) response(base string, op Operation) {
	rs := op.Response
	if rs.Select != "" && !strings.HasPrefix(rs.Select, "$") {
		v.addf(base+"/select", "must be a path starting with $")
	}
	if referencesCredentials(rs.Select) {
		v.addf(base+"/select", "$credentials is not visible in response mapping")
	}
	if _, err := parsePath(rs.Select); rs.Select != "" && err != nil {
		v.addf(base+"/select", "unsupported path: %v", err)
	}
	if !slices.Contains(validEmpty, rs.Empty) {
		v.addf(base+"/empty", "must be NO_ITEMS or ONE_EMPTY_ITEM")
	}
	for i, rule := range rs.Errors {
		v.errorRule(fmt.Sprintf("%s/errors/%d", base, i), op, rule)
	}
	v.responseMap(base+"/map", rs.Map)
}

func (v *validator) errorRule(p string, op Operation, rule ErrorRule) {
	if !slices.Contains(Classes(), Class(rule.Class)) {
		v.addf(p+"/class", "must be TRANSIENT, PERMANENT or UNCERTAIN")
	}
	if Class(rule.Class) == ClassUncertain && op.Purity != PurityEffect {
		v.addf(p+"/class", "UNCERTAIN is allowed only on EFFECT operations (ADR-0079 §4)")
	}
	if rule.Code != "" && !IsCode(rule.Code) {
		v.addf(p+"/code", "%q is not one of the fourteen codes of ADR-0079 §5", rule.Code)
	}
	if len(rule.Status) == 0 && rule.Body == nil {
		v.addf(p, "needs a status code, a body match or both")
	}
	for _, s := range rule.Status {
		if s < 100 || s > 599 {
			v.addf(p+"/status", "%d is not an HTTP status", s)
		}
	}
	if rule.Body != nil {
		v.bodyMatch(p+"/body", *rule.Body)
	}
}

// responseMap checks the keys and paths of a response map: each output key is
// named and each path starts with $ and is supported by the interpreter.
func (v *validator) responseMap(base string, mapping map[string]string) {
	for _, k := range slices.Sorted(maps.Keys(mapping)) {
		if k == "" {
			v.addf(base, "output keys must not be empty")
		}
		p := base + "/" + k
		if !strings.HasPrefix(mapping[k], "$") {
			v.addf(p, "must be a path starting with $")
		} else if _, err := parsePath(mapping[k]); err != nil {
			v.addf(p, "unsupported path: %v", err)
		}
	}
}

// bodyMatch checks the path and the scalars of a body condition of an error
// rule. Objects and arrays cannot be compared, so they are refused.
func (v *validator) bodyMatch(base string, m BodyMatch) {
	if _, err := parsePath(m.Path); err != nil {
		v.addf(base+"/path", "unsupported path: %v", err)
	}
	if len(m.In) == 0 {
		v.addf(base+"/in", "needs at least one scalar to compare with")
	}
	for i, n := range m.In {
		if _, isText := n.Value.Str(); isText {
			continue
		}
		if _, isNumber := n.Value.NumberText(); isNumber {
			continue
		}
		if _, isBool := n.Value.Bool(); !isBool {
			v.addf(fmt.Sprintf("%s/in/%d", base, i), "must be a string, a number or a boolean")
		}
	}
}

func (v *validator) pagination(base string, op Operation) {
	pg := op.Pagination
	if pg == nil {
		return
	}
	if op.Purity != PurityRead {
		v.addf(base, "pagination is allowed only on READ operations")
		return
	}
	if !slices.Contains(validStrategies, pg.Strategy) {
		v.addf(base+"/strategy", "must be OFFSET, PAGE, CURSOR, LINK_HEADER or NEXT_URL")
		return
	}
	switch pg.Strategy {
	case "OFFSET", "PAGE":
		if pg.Param == "" {
			v.addf(base+"/param", "%s needs the query parameter", pg.Strategy)
		}
	case "CURSOR":
		if pg.Param == "" || (pg.From != "PATH" && pg.From != "HEADER") {
			v.addf(base, "CURSOR needs param and from PATH or HEADER")
		}
		if pg.From == "PATH" && pg.Path == "" || pg.From == "HEADER" && pg.Header == "" {
			v.addf(base, "CURSOR needs the path or header it reads")
		}
	case "NEXT_URL":
		if pg.Path == "" {
			v.addf(base+"/path", "NEXT_URL needs the path to the next URL")
		}
	}
	if _, err := parsePath(pg.Path); pg.Path != "" && err != nil {
		v.addf(base+"/path", "unsupported path: %v", err)
	}
	if pg.MaxPages < 0 {
		v.addf(base+"/maxPages", "must not be negative")
	}
	if pg.OnMaxPages != "" && pg.OnMaxPages != "TRUNCATE" {
		v.addf(base+"/onMaxPages", "must be TRUNCATE or empty")
	}
}
