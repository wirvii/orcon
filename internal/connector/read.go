// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// The pattern sources below are shared with the published JSON Schema, which
// the tests compare against these constants so the two cannot drift. They use
// only constructs that RE2 and ECMAScript read the same way.
const (
	// IDPattern is the form of a connector id: "<publisher>.<name>".
	IDPattern = `^[a-z][a-z0-9-]{0,62}\.[a-z][a-z0-9-]{0,62}$`
	// NodeNamePattern is the form of a node name inside a connector.
	NodeNamePattern = `^[a-z][A-Za-z0-9]{0,63}$`
	// OperationNamePattern is the form of an operation name: "<resource>.<verb>".
	OperationNamePattern = `^[a-z][A-Za-z0-9]{0,31}\.[a-z][A-Za-z0-9]{0,31}$`
	// CredentialIDPattern is the form of a credential type id.
	CredentialIDPattern = `^[a-z][A-Za-z0-9]{0,63}$` //nolint:gosec // G101: the name mentions credentials; the value is a pattern, not a secret
	// FieldNamePattern is the form of a credential field name.
	FieldNamePattern = `^[A-Za-z][A-Za-z0-9_]{0,63}$`
	// RateLimitNamePattern is the form of a rate limit name.
	RateLimitNamePattern = `^[a-z][A-Za-z0-9]{0,63}$`
	// CommitPattern is the form of a source commit.
	CommitPattern = `^[0-9a-f]{7,64}$`
	// DatePattern is the form of a changelog date.
	DatePattern = `^\d{4}-\d{2}-\d{2}$`
	// LicensePattern is the character set of an SPDX license expression.
	LicensePattern = `^[A-Za-z0-9.+() -]{1,128}$`
	// MessageKeyPattern is the form of a key of the i18n catalogs.
	MessageKeyPattern = `^[A-Za-z][A-Za-z0-9_.-]{0,127}$`
	// PointerPattern is a JSON Pointer (RFC 6901) that is not empty.
	PointerPattern = `^(/([^/~]|~[01])*)+$`
	// DomainPattern is a granted host: an exact host or one leading wildcard label.
	DomainPattern = `^(\*\.)?[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`
	// SemverRangePattern is a SemVer range such as ">=1.0.0 <2.0.0" or "^1.2".
	SemverRangePattern = `^\s*` + rangeSet + `(\s*\|\|\s*` + rangeSet + `)*\s*$`

	rangeTerm = `(\^|~|>=|<=|>|<|=)?(0|[1-9][0-9]*)(\.(0|[1-9][0-9]*|[xX*]))?(\.(0|[1-9][0-9]*|[xX*]))?(-[0-9A-Za-z.-]+)?`
	rangeSet  = `(\*|` + rangeTerm + `)(\s+(\*|` + rangeTerm + `))*`

	maxLongText  = 4000
	maxShortText = 256
)

var (
	idPattern            = regexp.MustCompile(IDPattern)
	nodeNamePattern      = regexp.MustCompile(NodeNamePattern)
	operationNamePattern = regexp.MustCompile(OperationNamePattern)
	credentialIDPattern  = regexp.MustCompile(CredentialIDPattern)
	fieldNamePattern     = regexp.MustCompile(FieldNamePattern)
	rateLimitPattern     = regexp.MustCompile(RateLimitNamePattern)
	commitPattern        = regexp.MustCompile(CommitPattern)
	datePattern          = regexp.MustCompile(DatePattern)
	licensePattern       = regexp.MustCompile(LicensePattern)
	messageKeyPattern    = regexp.MustCompile(MessageKeyPattern)
	pointerPattern       = regexp.MustCompile(PointerPattern)
	semverRangePattern   = regexp.MustCompile(SemverRangePattern)
	domainRegexp         = regexp.MustCompile(DomainPattern)
	versionPattern       = regexp.MustCompile(flow.ConnectorVersionPattern)
	durationPattern      = regexp.MustCompile(flow.DurationPattern)
	portNamePattern      = regexp.MustCompile(flow.PortPattern)
	nodeTypeTail         = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// Options tunes how a manifest is read.
type Options struct {
	// Official allows ids in the reserved "orcon." namespace. Only the
	// composition root, for the connectors built into the binary, sets it;
	// packages installed from an archive never get it (ADR-0029 section 3).
	Official bool
}

// Parse reads a manifest from JSON text. The text must be one valid JSON value
// without duplicate object keys. A document that is not a valid manifest fails
// with a [*ManifestError].
func Parse(data []byte, opts Options) (Manifest, error) {
	v, err := jsonvalue.Parse(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse connector manifest: %w", err)
	}
	return Read(v, opts)
}

// Read maps an orcon/connector@1 JSON value to a [Manifest] and checks every
// rule of ADR-0029 sections 3 to 5 and 10: the shape, the closed UPPER_SNAKE
// value sets, the identity rules, the references between nodes, operations,
// credentials and permissions, and the safety contract of every effect
// operation. A problem is returned as a [*ManifestError] listing every finding.
func Read(v jsonvalue.Value, opts Options) (Manifest, error) {
	r := &reader{}
	m := r.manifest(v, opts)
	m.doc = v
	if len(r.issues) == 0 {
		r.checkManifest(&m)
	}
	if len(r.issues) > 0 {
		return Manifest{}, &ManifestError{Issues: r.issues}
	}
	return m, nil
}

func (r *reader) manifest(v jsonvalue.Value, opts Options) Manifest {
	o, ok := r.object("", v)
	if !ok {
		return Manifest{}
	}
	m := Manifest{Format: o.str("format", true)}
	if m.Format != "" && m.Format != FormatTag {
		r.failf(RuleValue, o.at("format"), "%q is not supported; this build reads %q", m.Format, FormatTag)
	}
	m.ID = o.pattern("id", "connector id", idPattern, true)
	r.checkIdentity(o, m.ID, opts)
	m.Version = o.pattern("version", "version", versionPattern, true)
	m.Kind = enumMember(o, "kind", Kinds(), true)
	m.DisplayName = o.text("displayName", maxShortText, true)
	m.Description = o.text("description", maxLongText, true)
	if c, ok := o.child("publisher", true); ok {
		m.Publisher = r.publisher(c)
		c.done()
	}
	m.License = o.pattern("license", "SPDX license expression", licensePattern, true)
	if c, ok := o.child("source", false); ok {
		m.Source = r.source(c)
		c.done()
	}
	if c, ok := o.child("compat", true); ok {
		m.Compat = r.compat(c, m.Kind)
		c.done()
	}
	o.each("nodes", true, func(el *obj, _ int) { m.Nodes = append(m.Nodes, r.node(el, &m)) })
	o.each("credentials", false, func(el *obj, _ int) { m.Credentials = append(m.Credentials, r.credential(el)) })
	if c, ok := o.child("permissions", true); ok {
		m.Permissions = r.permissions(c)
		c.done()
	}
	o.each("rateLimits", false, func(el *obj, _ int) { m.RateLimits = append(m.RateLimits, r.rateLimit(el)) })
	m.N8n = o.opaque("n8n")
	m.AI = o.opaque("ai")
	o.each("changelog", true, func(el *obj, _ int) { m.Changelog = append(m.Changelog, r.changelogEntry(el)) })
	o.done()
	return m
}

// text reads a non-empty string of at most maxRunes characters.
func (o *obj) text(key string, maxRunes int, required bool) string {
	s := o.str(key, required)
	if len([]rune(s)) > maxRunes {
		o.r.failf(RuleValue, o.at(key), "must have at most %d characters", maxRunes)
	}
	return s
}

// checkIdentity applies the reserved namespace rule (ADR-0029 section 3).
func (r *reader) checkIdentity(o *obj, id string, opts Options) {
	if id == "" || opts.Official {
		return
	}
	if strings.HasPrefix(id, "orcon.") {
		r.failf(RuleIdentity, o.at("id"), "id %q is in the reserved orcon. namespace, which only official connectors built into the binary may use", id)
	}
}

func (r *reader) publisher(o *obj) Publisher {
	return Publisher{
		Name:    o.text("name", maxShortText, true),
		URL:     o.webURL("url", false),
		Contact: o.text("contact", maxShortText, false),
	}
}

// webURL reads an absolute http or https URL.
func (o *obj) webURL(key string, required bool) string {
	s := o.str(key, required)
	if s == "" {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		o.r.failf(RuleValue, o.at(key), "%q is not an absolute http or https URL", s)
	}
	return s
}

func (r *reader) source(o *obj) *Source {
	return &Source{
		Repository:  o.webURL("repository", true),
		Commit:      o.pattern("commit", "commit", commitPattern, true),
		BuildRecipe: o.text("buildRecipe", maxLongText, false),
	}
}

func (r *reader) compat(o *obj, kind Kind) Compat {
	c := Compat{EngineAPI: o.pattern("engineApi", "engine API range", semverRangePattern, true)}
	abi, present := o.integer("wasmAbi", false, math.MaxInt32)
	c.WasmABI = int(abi)
	switch {
	case kind == KindWASM && !present:
		r.failf(RuleShape, o.at("wasmAbi"), "is required for a WASM connector")
	case kind != KindWASM && kind != "" && present:
		r.failf(RuleShape, o.at("wasmAbi"), "applies to WASM connectors only, not to %s", kind)
	}
	return c
}

func (r *reader) node(o *obj, m *Manifest) Node {
	n := Node{
		Name:        o.pattern("name", "node name", nodeNamePattern, true),
		Type:        o.str("type", true),
		DisplayName: o.text("displayName", maxShortText, true),
		Group:       enumMember(o, "group", Groups(), true),
	}
	r.checkNodeType(o, m, &n)
	o.each("operations", false, func(el *obj, _ int) { n.Operations = append(n.Operations, r.operation(el)) })
	if c, ok := o.child("trigger", false); ok {
		n.Trigger = r.trigger(c)
		c.done()
	}
	switch {
	case n.Group == GroupAction && len(n.Operations) == 0:
		r.failf(RuleShape, o.at("operations"), "an ACTION node needs at least one operation")
	case n.Group == GroupAction && n.Trigger != nil:
		r.failf(RuleShape, o.at("trigger"), "an ACTION node has operations, not a trigger")
	case n.Group == GroupTrigger && n.Trigger == nil:
		r.failf(RuleShape, o.at("trigger"), "a TRIGGER node needs a trigger declaration")
	case n.Group == GroupTrigger && len(n.Operations) > 0:
		r.failf(RuleShape, o.at("operations"), "a TRIGGER node has a trigger declaration, not operations")
	}
	n.Parameters = o.opaque("parameters")
	r.reservedParameters(o, n.Parameters)
	n.Outputs = o.portNames("outputs")
	n.SensitiveFields = o.pointers("sensitiveFields")
	if c, ok := o.child("n8n", false); ok {
		c.each("types", true, func(el *obj, _ int) { n.N8nTypes = append(n.N8nTypes, r.n8nType(el)) })
		c.done()
	}
	return n
}

// reservedParameterNames are the parameter names the platform keeps for
// operation selection (ADR-0029 section 4, SPEC-003 Amendment 8): "operation"
// holds the operation a node runs and "resource" is the part of its name before
// the dot, so a manifest cannot use either for another setting.
var reservedParameterNames = []string{"operation", "resource"}

// reservedParameters rejects a parameter schema that declares a reserved name.
func (r *reader) reservedParameters(o *obj, params jsonvalue.Value) {
	if params.IsZero() {
		return
	}
	for _, name := range reservedParameterNames {
		if _, ok := params.Get(name); ok {
			r.failf(RuleIdentity, o.at("parameters")+"/"+name, "parameter name %q is reserved for operation selection; choose another name", name)
		}
	}
}

// checkNodeType requires the type to read "<connector id>.<name>@<major>"
// (ADR-0029 section 3), the form flows use.
func (r *reader) checkNodeType(o *obj, m *Manifest, n *Node) {
	if n.Type == "" || n.Name == "" || m.ID == "" {
		return
	}
	prefix := m.ID + "." + n.Name + "@"
	if !strings.HasPrefix(n.Type, prefix) || !nodeTypeTail.MatchString(strings.TrimPrefix(n.Type, prefix)) {
		r.failf(RuleIdentity, o.at("type"), "node type %q must read %s<major>, for example %s1", n.Type, prefix, prefix)
	}
}

func (o *obj) portNames(key string) []string {
	names := o.strings(key)
	for i, name := range names {
		if !portNamePattern.MatchString(name) {
			o.r.failf(RuleValue, fmt.Sprintf("%s/%d", o.at(key), i), "port %q is not valid; it must match %s", name, portNamePattern)
		}
	}
	return names
}

func (o *obj) pointers(key string) []string {
	ptrs := o.strings(key)
	for i, p := range ptrs {
		if !pointerPattern.MatchString(p) {
			o.r.failf(RuleValue, fmt.Sprintf("%s/%d", o.at(key), i), "%q is not a JSON Pointer such as /customer/email", p)
		}
	}
	return ptrs
}

func (r *reader) n8nType(o *obj) N8nType {
	t := N8nType{Name: o.text("name", maxShortText, true)}
	items, ok := o.array("typeVersions", true)
	if ok && len(items) == 0 {
		r.failf(RuleValue, o.at("typeVersions"), "must list at least one type version")
	}
	for i, item := range items {
		f, isNum := item.Float64()
		if !isNum || f <= 0 || math.IsInf(f, 0) {
			r.failf(RuleValue, fmt.Sprintf("%s/%d", o.at("typeVersions"), i), "must be a number greater than zero")
			continue
		}
		t.TypeVersions = append(t.TypeVersions, f)
	}
	t.ParameterMap = o.opaque("parameterMap")
	return t
}

func (r *reader) trigger(o *obj) *Trigger {
	t := &Trigger{
		Family:          enumMember(o, "family", TriggerFamilies(), true),
		Delivery:        enumMember(o, "delivery", Deliveries(), true),
		DedupeKeySource: enumMember(o, "dedupeKeySource", DedupeSources(), false),
	}
	if n, ok := o.integer("maxPayload", false, jsonvalue.MaxSafeInteger); ok {
		t.MaxPayload = n
	}
	return t
}

func (r *reader) operation(o *obj) Operation {
	op := Operation{
		Name:        o.pattern("name", "operation name", operationNamePattern, true),
		DisplayName: o.text("displayName", maxShortText, true),
		Purity:      enumMember(o, "purity", OperationPurities(), true),
		ItemMode:    enumMember(o, "itemMode", ItemModes(), true),
		RateLimit:   o.pattern("rateLimit", "rate limit name", rateLimitPattern, false),
	}
	if c, ok := o.child("safety", false); ok {
		op.Safety = r.safety(c)
		c.done()
	}
	op.Retry = o.opaque("retry")
	switch {
	case op.Purity == flow.PurityEffect && op.Safety == nil:
		r.failf(RuleSafety, o.at("safety"), "operation %q has purity EFFECT and needs a safety contract (ADR-0029 section 5)", op.Name)
	case op.Purity != flow.PurityEffect && op.Purity != "" && op.Safety != nil:
		r.failf(RuleSafety, o.at("safety"), "operation %q has purity %s; only EFFECT operations carry a safety contract", op.Name, op.Purity)
	}
	return op
}

func (r *reader) safety(o *obj) *Safety {
	s := &Safety{
		Effect:      enumMember(o, "effect", flow.EffectKinds(), true),
		Level:       enumMember(o, "level", flow.SafetyLevels(), true),
		OnUncertain: enumMember(o, "onUncertain", flow.OnUncertains(), false),
		RetrySafe:   o.boolean("retrySafe"),
		ProofLevel:  enumMember(o, "proofLevel", ProofLevels(), true),
	}
	if c, ok := o.child("idempotency", false); ok {
		s.Idempotency = r.idempotency(c)
		c.done()
	}
	if c, ok := o.child("verification", false); ok {
		s.Verification = r.verification(c)
		c.done()
	}
	if c, ok := o.child("compensation", false); ok {
		s.Compensation = &Compensation{
			Operation:    c.pattern("operation", "operation name", operationNamePattern, true),
			Precondition: c.text("precondition", maxLongText, false),
			Verify:       c.pattern("verify", "operation name", operationNamePattern, false),
		}
		c.done()
	}
	if c, ok := o.child("batch", false); ok {
		maxItems, _ := c.integer("maxItems", true, math.MaxInt32)
		s.Batch = &Batch{MaxItems: int(maxItems), PerItemResults: c.boolean("perItemResults")}
		c.done()
	}
	if c, ok := o.child("manualCheck", false); ok {
		s.ManualCheck = &ManualCheck{Description: c.text("description", maxLongText, true), URL: c.text("url", maxLongText, false)}
		c.done()
	}
	return s
}

func (r *reader) idempotency(o *obj) *IdempotencyContract {
	c := &IdempotencyContract{
		Mode:  enumMember(o, "mode", flow.IdempotencyModes(), true),
		TTL:   o.pattern("ttl", "ISO 8601 duration", durationPattern, false),
		Scope: enumMember(o, "scope", KeyScopes(), false),
	}
	if l, ok := o.child("location", false); ok {
		place := &flow.KeyPlace{In: enumMember(l, "in", flow.KeyLocations(), true)}
		// A header or query key is named; a body key is a path into the body.
		place.Name = l.text("name", maxShortText, false)
		if path := l.text("path", maxShortText, false); path != "" {
			if place.Name != "" {
				r.failf(RuleShape, l.at("path"), "use name or path, not both")
			}
			place.Name = path
		}
		if place.Name == "" {
			r.failf(RuleShape, l.at("name"), "the location needs a name (header or query) or a path (body)")
		}
		l.done()
		c.Location = place
	}
	return c
}

func (r *reader) verification(o *obj) *VerificationContract {
	v := &VerificationContract{
		Operation:           o.pattern("operation", "operation name", operationNamePattern, true),
		Match:               o.text("match", maxLongText, true),
		NotFoundMeansFailed: o.boolean("notFoundMeansFailed"),
	}
	c, ok := o.get("consistency", true)
	if !ok {
		return v
	}
	if s, isStr := c.Str(); isStr {
		if s != "STRONG" {
			r.failf(RuleEnum, o.at("consistency"), "%q is not STRONG or an { eventualDelay } object", s)
		}
		v.Consistency = s
		return v
	}
	eo, isObj := r.object(o.at("consistency"), c)
	if !isObj {
		return v
	}
	v.Consistency = eo.pattern("eventualDelay", "ISO 8601 duration", durationPattern, true)
	eo.done()
	return v
}

func (r *reader) changelogEntry(o *obj) ChangelogEntry {
	e := ChangelogEntry{
		Version: o.pattern("version", "version", versionPattern, true),
		Date:    o.pattern("date", "date", datePattern, true),
	}
	if _, err := time.Parse(time.DateOnly, e.Date); e.Date != "" && err != nil {
		r.failf(RuleValue, o.at("date"), "%q is not a calendar date", e.Date)
	}
	for i, note := range o.strings("notes") {
		if !messageKeyPattern.MatchString(note) {
			r.failf(RuleValue, fmt.Sprintf("%s/%d", o.at("notes"), i), "%q is not a message key such as changelog.1_0_0.initial", note)
		}
		e.Notes = append(e.Notes, note)
	}
	if _, present := o.v.Get("notes"); !present {
		r.failf(RuleShape, o.at("notes"), "is required")
	}
	return e
}
