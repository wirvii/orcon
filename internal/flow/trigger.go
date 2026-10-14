// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// TriggerFamily is the family of a trigger (ADR-0020 section 1).
type TriggerFamily string

// Trigger families, the spelling the API exposes (ADR-0080 section 3).
const (
	// TriggerWebhook is inbound HTTP: generic webhook, hosted form, chat, MCP.
	TriggerWebhook TriggerFamily = "WEBHOOK"
	// TriggerAppEvent is inbound HTTP whose endpoint Orcon registers at a provider.
	TriggerAppEvent TriggerFamily = "APP_EVENT"
	// TriggerSchedule is a calendar or interval rule.
	TriggerSchedule TriggerFamily = "SCHEDULE"
	// TriggerPoll is a periodic pull with a durable cursor.
	TriggerPoll TriggerFamily = "POLL"
	// TriggerListen is a persistent connection to a broker, stream or mailbox.
	TriggerListen TriggerFamily = "LISTEN"
	// TriggerInternal is a manual run, a call from a flow, an error flow, a
	// lifecycle event or an evaluation.
	TriggerInternal TriggerFamily = "INTERNAL"
)

// TriggerFamilies returns every valid [TriggerFamily].
func TriggerFamilies() []TriggerFamily {
	return []TriggerFamily{TriggerWebhook, TriggerAppEvent, TriggerSchedule, TriggerPoll, TriggerListen, TriggerInternal}
}

// AddressFamily is the family of a web address (ADR-0022 section 1).
type AddressFamily string

// Address families.
const (
	// AddressN8N is the n8n-compatible path form.
	AddressN8N AddressFamily = "N8N"
	// AddressNative is the Orcon path form, which carries the project.
	AddressNative AddressFamily = "NATIVE"
)

// AddressFamilies returns every valid [AddressFamily].
func AddressFamilies() []AddressFamily { return []AddressFamily{AddressN8N, AddressNative} }

// ResponseProfile is the response behavior of one address (ADR-0023 section 1).
type ResponseProfile string

// Response profiles.
const (
	// ProfileNative answers like Orcon: 202 and the execution id.
	ProfileNative ResponseProfile = "NATIVE"
	// ProfileN8N answers like the pinned n8n version.
	ProfileN8N ResponseProfile = "N8N"
)

// ResponseProfiles returns every valid [ResponseProfile].
func ResponseProfiles() []ResponseProfile { return []ResponseProfile{ProfileNative, ProfileN8N} }

// ResponseMode says when and with what a web trigger answers.
type ResponseMode string

// Response modes.
const (
	// ResponseImmediate answers 202 with the execution id once the event is stored.
	ResponseImmediate ResponseMode = "IMMEDIATE"
	// ResponseLastNode waits for the result of the last node.
	ResponseLastNode ResponseMode = "LAST_NODE"
	// ResponseRespondNode waits for an orcon.respond node.
	ResponseRespondNode ResponseMode = "RESPOND_NODE"
	// ResponseStream streams fragments of the result.
	ResponseStream ResponseMode = "STREAM"
)

// ResponseModes returns every valid [ResponseMode].
func ResponseModes() []ResponseMode {
	return []ResponseMode{ResponseImmediate, ResponseLastNode, ResponseRespondNode, ResponseStream}
}

// DedupeSource says where the deduplication key of an event comes from.
type DedupeSource string

// Deduplication sources.
const (
	// DedupeHeader reads the key from a request header.
	DedupeHeader DedupeSource = "HEADER"
	// DedupeJSONPath reads the key from a field of the JSON body.
	DedupeJSONPath DedupeSource = "JSON_PATH"
	// DedupeConnector lets the connector derive the key.
	DedupeConnector DedupeSource = "CONNECTOR"
)

// DedupeSources returns every valid [DedupeSource].
func DedupeSources() []DedupeSource {
	return []DedupeSource{DedupeHeader, DedupeJSONPath, DedupeConnector}
}

// MissedPolicy says what a schedule does with a slot it missed (ADR-0025 section 4).
type MissedPolicy string

// Missed-slot policies.
const (
	// MissedSkip runs nothing for a missed slot.
	MissedSkip MissedPolicy = "SKIP"
	// MissedCatchUpOnce runs one execution for the latest recent missed slot.
	MissedCatchUpOnce MissedPolicy = "CATCH_UP_ONCE"
)

// MissedPolicies returns every valid [MissedPolicy].
func MissedPolicies() []MissedPolicy { return []MissedPolicy{MissedSkip, MissedCatchUpOnce} }

// RuleKind is the kind of a schedule or poll rule (ADR-0025 section 1).
type RuleKind string

// Rule kinds.
const (
	// RuleSeconds fires every N seconds.
	RuleSeconds RuleKind = "SECONDS"
	// RuleMinutes fires every N minutes.
	RuleMinutes RuleKind = "MINUTES"
	// RuleHours fires every N hours.
	RuleHours RuleKind = "HOURS"
	// RuleDays fires every N days at a time of day.
	RuleDays RuleKind = "DAYS"
	// RuleWeeks fires every N weeks on given weekdays at a time of day.
	RuleWeeks RuleKind = "WEEKS"
	// RuleMonths fires every N months on a day of the month at a time of day.
	RuleMonths RuleKind = "MONTHS"
	// RuleCron fires by a cron expression of 5 or 6 fields.
	RuleCron RuleKind = "CRON"
)

// RuleKinds returns every valid [RuleKind].
func RuleKinds() []RuleKind {
	return []RuleKind{RuleSeconds, RuleMinutes, RuleHours, RuleDays, RuleWeeks, RuleMonths, RuleCron}
}

// Weekday is a day of the week in a weekly rule.
type Weekday string

// Weekdays.
const (
	// Monday is the first day of the ISO week.
	Monday Weekday = "MONDAY"
	// Tuesday follows Monday.
	Tuesday Weekday = "TUESDAY"
	// Wednesday follows Tuesday.
	Wednesday Weekday = "WEDNESDAY"
	// Thursday follows Wednesday.
	Thursday Weekday = "THURSDAY"
	// Friday follows Thursday.
	Friday Weekday = "FRIDAY"
	// Saturday follows Friday.
	Saturday Weekday = "SATURDAY"
	// Sunday ends the ISO week.
	Sunday Weekday = "SUNDAY"
)

// Weekdays returns every valid [Weekday].
func Weekdays() []Weekday {
	return []Weekday{Monday, Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday}
}

// HTTP methods an address may accept. They are fixed by the HTTP standard and
// keep its spelling (R-DB-02).
var httpMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"}

// HTTPMethods returns the methods an address may accept.
func HTTPMethods() []string { return slices.Clone(httpMethods) }

// Trigger is the optional trigger block of a startEvent node (ADR-0020
// section 2). Only the sub-blocks of its family are present.
type Trigger struct {
	Family       TriggerFamily
	Addresses    []TriggerAddress
	Response     *TriggerResponse
	Dedupe       *TriggerDedupe
	Schedule     *TriggerScheduleSpec
	Poll         *TriggerPollSpec
	Listen       *TriggerListenSpec
	Registration *TriggerRegistrationSpec
	// InputSchema describes the input of a called-by-flow trigger; the format
	// leaves its content open.
	InputSchema jsonvalue.Value
	Callers     *TriggerCallers
}

// TriggerAddress is one web address of a trigger. The production address is
// derived from it; the test address of an open test session is derived too.
type TriggerAddress struct {
	Family AddressFamily
	// Path is the address path without a leading slash. Segments that start
	// with a colon are route parameters.
	Path string
	// WebhookID is the identifier n8n shows for paths with route parameters; it
	// is kept from the import so the address stays the same.
	WebhookID string
	// Suffix is the optional unguessable last segment (128 bits, base62) of a
	// native address.
	Suffix string
	// Methods lists the accepted HTTP methods; empty means POST.
	Methods         []string
	ResponseProfile ResponseProfile
}

// TriggerResponse selects how a web trigger answers.
type TriggerResponse struct {
	Mode ResponseMode
	// WaitTimeout is the ISO 8601 longest wait of the waiting modes.
	WaitTimeout string
}

// TriggerDedupe configures deduplication of events.
type TriggerDedupe struct {
	Source DedupeSource
	// Name is the header name or the JSON path, depending on Source.
	Name string
	// Window is an ISO 8601 duration from one minute to 90 days; empty means 24 hours.
	Window string
}

// TriggerRule is one rule of a schedule or a poll.
type TriggerRule struct {
	Kind RuleKind
	// Every is the interval of the interval and calendar rules; empty means 1.
	Every int
	// At is the local time of day "HH:MM" of the calendar rules.
	At         string
	Weekdays   []Weekday
	DayOfMonth int
	// Cron is the expression of a CRON rule.
	Cron string
}

// TriggerScheduleSpec is the schedule block.
type TriggerScheduleSpec struct {
	// TimeZone is an IANA zone name; empty means the zone of the instance.
	TimeZone string
	Rules    []TriggerRule
	Missed   *TriggerMissed
}

// TriggerMissed configures missed slots.
type TriggerMissed struct {
	Policy MissedPolicy
	// MarginHours bounds how old a missed slot may be to be caught up.
	MarginHours int
}

// TriggerPollSpec is the poll block.
type TriggerPollSpec struct {
	Rules []TriggerRule
}

// TriggerListenSpec is the listen block.
type TriggerListenSpec struct {
	// MaxInFlight bounds the executions in progress of the trigger.
	MaxInFlight int
	// HostTag restricts the workers that may hold the connection.
	HostTag string
	// LocalPath is the watched path of a local-file trigger.
	LocalPath string
}

// TriggerRegistrationSpec marks a trigger whose endpoint Orcon registers at a
// provider (ADR-0027). It has no members yet.
type TriggerRegistrationSpec struct{}

// TriggerCallers says who may call a called-by-flow trigger.
type TriggerCallers struct {
	// Project allows every flow of the project; Flows is then empty.
	Project bool
	// Flows lists the allowed callers when Project is false.
	Flows []ID
}

func (a TriggerAddress) value() jsonvalue.Value {
	var b builder
	b.str("family", string(a.Family))
	b.str("path", a.Path)
	b.str("webhookId", a.WebhookID)
	b.str("suffix", a.Suffix)
	b.strs("methods", a.Methods)
	b.str("responseProfile", string(a.ResponseProfile))
	return b.build()
}

func (r TriggerRule) value() jsonvalue.Value {
	var b builder
	b.str("kind", string(r.Kind))
	b.integer("every", r.Every)
	b.str("at", r.At)
	if r.Weekdays != nil {
		days := make([]string, len(r.Weekdays))
		for i, d := range r.Weekdays {
			days[i] = string(d)
		}
		b.strs("weekdays", days)
	}
	b.integer("dayOfMonth", r.DayOfMonth)
	b.str("cron", r.Cron)
	return b.build()
}

func (t *Trigger) value() jsonvalue.Value {
	var b builder
	b.str("family", string(t.Family))
	if t.Addresses != nil {
		b.val("addresses", jsonvalue.Array(valuesOf(t.Addresses, TriggerAddress.value)...))
	}
	if t.Response != nil {
		var r builder
		r.str("mode", string(t.Response.Mode))
		r.str("waitTimeout", t.Response.WaitTimeout)
		b.val("response", r.build())
	}
	if t.Dedupe != nil {
		var d builder
		d.str("source", string(t.Dedupe.Source))
		d.str("name", t.Dedupe.Name)
		d.str("window", t.Dedupe.Window)
		b.val("dedupe", d.build())
	}
	if t.Schedule != nil {
		var s builder
		s.str("timeZone", t.Schedule.TimeZone)
		s.val("rules", jsonvalue.Array(valuesOf(t.Schedule.Rules, TriggerRule.value)...))
		if m := t.Schedule.Missed; m != nil {
			var mb builder
			mb.str("policy", string(m.Policy))
			mb.integer("marginHours", m.MarginHours)
			s.val("missed", mb.build())
		}
		b.val("schedule", s.build())
	}
	if t.Poll != nil {
		var p builder
		p.val("rules", jsonvalue.Array(valuesOf(t.Poll.Rules, TriggerRule.value)...))
		b.val("poll", p.build())
	}
	if t.Listen != nil {
		var l builder
		l.integer("maxInFlight", t.Listen.MaxInFlight)
		l.str("hostTag", t.Listen.HostTag)
		l.str("localPath", t.Listen.LocalPath)
		b.val("listen", l.build())
	}
	if t.Registration != nil {
		b.val("registration", (&builder{}).build())
	}
	b.val("inputSchema", t.InputSchema)
	if c := t.Callers; c != nil {
		if c.Project {
			b.val("callers", jsonvalue.String("PROJECT"))
		} else {
			ids := make([]string, len(c.Flows))
			for i, f := range c.Flows {
				ids[i] = string(f)
			}
			b.strs("callers", ids)
		}
	}
	return b.build()
}

func valuesOf[T any](items []T, fn func(T) jsonvalue.Value) []jsonvalue.Value {
	out := make([]jsonvalue.Value, len(items))
	for i, item := range items {
		out[i] = fn(item)
	}
	return out
}

// TriggerConfigurationHash returns the configuration hash of the trigger of a
// node: SHA-256 over the canonical form of the trigger block together with the
// node type and parameters (ADR-0020 section 2). It is empty for a node
// without a trigger block. Two versions with the same hash keep the state of
// the trigger (ADR-0024 section 3).
func (n *Node) TriggerConfigurationHash() (string, error) {
	if n.Trigger == nil {
		return "", nil
	}
	var b builder
	b.val("trigger", n.Trigger.value())
	b.str("type", n.Type)
	b.val("parameters", orEmptyObject(n.Parameters))
	canonical, err := jsonvalue.Canonical(b.build())
	if err != nil {
		return "", fmt.Errorf("canonicalize trigger of node %q: %w", n.ID, err)
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// decoding

func (d *decoder) trigger(o *obj) *Trigger {
	t := &Trigger{Family: TriggerFamily(o.str("family", true))}
	if items, ok := o.array("addresses", false); ok {
		t.Addresses = make([]TriggerAddress, 0, len(items))
		for i, item := range items {
			if a, ok := d.object(fmt.Sprintf("%s/%d", o.at("addresses"), i), item); ok {
				t.Addresses = append(t.Addresses, d.address(a))
			}
		}
	}
	if c, ok := o.child("response", false); ok {
		t.Response = &TriggerResponse{Mode: ResponseMode(c.str("mode", true)), WaitTimeout: c.str("waitTimeout", false)}
		c.done()
	}
	if c, ok := o.child("dedupe", false); ok {
		t.Dedupe = &TriggerDedupe{Source: DedupeSource(c.str("source", true)), Name: c.str("name", false), Window: c.str("window", false)}
		c.done()
	}
	if c, ok := o.child("schedule", false); ok {
		t.Schedule = d.schedule(c)
	}
	if c, ok := o.child("poll", false); ok {
		t.Poll = &TriggerPollSpec{Rules: d.rules(c)}
		c.done()
	}
	if c, ok := o.child("listen", false); ok {
		t.Listen = &TriggerListenSpec{MaxInFlight: c.integer("maxInFlight", false), HostTag: c.str("hostTag", false), LocalPath: c.str("localPath", false)}
		c.done()
	}
	if c, ok := o.child("registration", false); ok {
		t.Registration = &TriggerRegistrationSpec{}
		c.done()
	}
	t.InputSchema = o.opaque("inputSchema", false)
	if _, ok := o.v.Get("callers"); ok {
		t.Callers = d.callers(o)
	}
	return t
}

func (d *decoder) address(o *obj) TriggerAddress {
	a := TriggerAddress{
		Family:          AddressFamily(o.str("family", true)),
		Path:            o.str("path", true),
		WebhookID:       o.str("webhookId", false),
		Suffix:          o.str("suffix", false),
		Methods:         o.strings("methods"),
		ResponseProfile: ResponseProfile(o.str("responseProfile", false)),
	}
	o.done()
	return a
}

func (d *decoder) schedule(o *obj) *TriggerScheduleSpec {
	s := &TriggerScheduleSpec{TimeZone: o.str("timeZone", false), Rules: d.rules(o)}
	if c, ok := o.child("missed", false); ok {
		s.Missed = &TriggerMissed{Policy: MissedPolicy(c.str("policy", true)), MarginHours: c.integer("marginHours", false)}
		c.done()
	}
	o.done()
	return s
}

func (d *decoder) rules(o *obj) []TriggerRule {
	items, ok := o.array("rules", true)
	if !ok {
		return nil
	}
	out := make([]TriggerRule, 0, len(items))
	for i, item := range items {
		r, ok := d.object(fmt.Sprintf("%s/%d", o.at("rules"), i), item)
		if !ok {
			continue
		}
		rule := TriggerRule{
			Kind:       RuleKind(r.str("kind", true)),
			Every:      r.integer("every", false),
			At:         r.str("at", false),
			DayOfMonth: r.integer("dayOfMonth", false),
			Cron:       r.str("cron", false),
		}
		for _, day := range r.strings("weekdays") {
			rule.Weekdays = append(rule.Weekdays, Weekday(day))
		}
		r.done()
		out = append(out, rule)
	}
	return out
}

func (d *decoder) callers(o *obj) *TriggerCallers {
	v, _ := o.get("callers", false)
	if s, ok := v.Str(); ok {
		if s != "PROJECT" {
			d.failf(o.at("callers"), "must be \"PROJECT\" or a list of flow ids, found %q", s)
		}
		return &TriggerCallers{Project: s == "PROJECT"}
	}
	ids := o.strings("callers")
	if ids == nil {
		d.failf(o.at("callers"), "must be \"PROJECT\" or a non-empty list of flow ids")
		return &TriggerCallers{}
	}
	c := &TriggerCallers{}
	for _, id := range ids {
		c.Flows = append(c.Flows, ID(id))
	}
	return c
}

// validation of the shape of the block; the publication rules T-V1..T-V9 that
// need the instance, the catalog or other flows live in internal/trigger.

var (
	addressPathPattern  = regexp.MustCompile(`^[A-Za-z0-9._~:-]+(/[A-Za-z0-9._~:-]+)*$`)
	timeOfDayPattern    = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
	suffixPattern       = regexp.MustCompile(`^[0-9A-Za-z]{22,}$`)
	hostTagPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	cronFieldCharacters = regexp.MustCompile(`^[0-9A-Za-z*/,?#-]+$`)
)

// Issue codes of the trigger block.
const (
	// CodeTrigger marks a trigger block that is misplaced or inconsistent with its family.
	CodeTrigger = "TRIGGER"
)

func (v *validator) trigger(base string, n *Node) {
	t, r := n.Trigger, v.r
	path := base + "/trigger"
	if n.Element != ElementStartEvent {
		r.errorf(CodeBlock, path, "the trigger block applies to startEvent nodes only, not to %q", n.Element)
		return
	}
	checkEnum(r, path+"/family", t.Family, TriggerFamilies())
	v.triggerBlocksMatchFamily(path, t)
	for i := range t.Addresses {
		v.triggerAddress(fmt.Sprintf("%s/addresses/%d", path, i), &t.Addresses[i])
	}
	if t.Response != nil {
		checkEnum(r, path+"/response/mode", t.Response.Mode, ResponseModes())
		checkDuration(r, path+"/response/waitTimeout", t.Response.WaitTimeout)
	}
	if t.Dedupe != nil {
		checkEnum(r, path+"/dedupe/source", t.Dedupe.Source, DedupeSources())
		if t.Dedupe.Source != DedupeConnector && t.Dedupe.Name == "" {
			r.errorf(CodeValue, path+"/dedupe/name", "name is required when source is %s or %s", DedupeHeader, DedupeJSONPath)
		}
		if t.Dedupe.Source == DedupeConnector && t.Dedupe.Name != "" {
			r.errorf(CodeValue, path+"/dedupe/name", "name does not apply when source is %s", DedupeConnector)
		}
		checkDuration(r, path+"/dedupe/window", t.Dedupe.Window)
	}
	if s := t.Schedule; s != nil {
		v.triggerRules(path+"/schedule/rules", s.Rules)
		if m := s.Missed; m != nil {
			checkEnum(r, path+"/schedule/missed/policy", m.Policy, MissedPolicies())
			if m.Policy == MissedCatchUpOnce && m.MarginHours == 0 {
				r.errorf(CodeValue, path+"/schedule/missed/marginHours", "marginHours is required when policy is %s", MissedCatchUpOnce)
			}
			if m.Policy == MissedSkip && m.MarginHours != 0 {
				r.errorf(CodeValue, path+"/schedule/missed/marginHours", "marginHours applies only when policy is %s", MissedCatchUpOnce)
			}
		}
	}
	if t.Poll != nil {
		v.triggerRules(path+"/poll/rules", t.Poll.Rules)
	}
	if l := t.Listen; l != nil && l.HostTag != "" {
		checkPattern(r, path+"/listen/hostTag", "host tag", l.HostTag, hostTagPattern)
	}
	checkObject(r, path+"/inputSchema", t.InputSchema)
}

// triggerBlocksMatchFamily rejects sub-blocks that do not belong to the
// family and requires the ones the family needs.
func (v *validator) triggerBlocksMatchFamily(path string, t *Trigger) {
	type block struct {
		name    string
		present bool
		// allowed lists the families that may carry the block.
		allowed []TriggerFamily
		// requiredBy is the family that cannot do without it; empty if none.
		requiredBy TriggerFamily
	}
	web := []TriggerFamily{TriggerWebhook, TriggerAppEvent}
	blocks := []block{
		{"addresses", len(t.Addresses) > 0, web, TriggerWebhook},
		{"response", t.Response != nil, web, ""},
		{"schedule", t.Schedule != nil, []TriggerFamily{TriggerSchedule}, TriggerSchedule},
		{"poll", t.Poll != nil, []TriggerFamily{TriggerPoll}, TriggerPoll},
		{"listen", t.Listen != nil, []TriggerFamily{TriggerListen}, TriggerListen},
		{"registration", t.Registration != nil, []TriggerFamily{TriggerAppEvent}, TriggerAppEvent},
		{"inputSchema", !t.InputSchema.IsZero(), []TriggerFamily{TriggerInternal}, ""},
		{"callers", t.Callers != nil, []TriggerFamily{TriggerInternal}, ""},
	}
	for _, b := range blocks {
		if b.present && !slices.Contains(b.allowed, t.Family) {
			v.r.errorf(CodeTrigger, path+"/"+b.name, "%s does not apply to a %s trigger", b.name, t.Family)
		}
		if !b.present && b.requiredBy == t.Family {
			v.r.errorf(CodeTrigger, path+"/"+b.name, "a %s trigger needs %s", t.Family, b.name)
		}
	}
	if t.Dedupe != nil && t.Family != TriggerWebhook && t.Family != TriggerAppEvent && t.Family != TriggerListen {
		v.r.errorf(CodeTrigger, path+"/dedupe", "dedupe applies to WEBHOOK, APP_EVENT and LISTEN triggers, not to %s", t.Family)
	}
}

func (v *validator) triggerAddress(path string, a *TriggerAddress) {
	r := v.r
	checkEnum(r, path+"/family", a.Family, AddressFamilies())
	if !addressPathPattern.MatchString(a.Path) || strings.Contains(a.Path, "..") {
		r.errorf(CodeValue, path+"/path", "address path %q must be segments of letters, digits and . _ ~ - : separated by slashes, without a leading slash", a.Path)
	}
	checkOptEnum(r, path+"/responseProfile", a.ResponseProfile, ResponseProfiles())
	if a.Suffix != "" {
		if a.Family != AddressNative {
			r.errorf(CodeValue, path+"/suffix", "a random suffix applies to %s addresses only", AddressNative)
		}
		checkPattern(r, path+"/suffix", "random suffix", a.Suffix, suffixPattern)
	}
	if a.WebhookID != "" && a.Family != AddressN8N {
		r.errorf(CodeValue, path+"/webhookId", "webhookId applies to %s addresses only", AddressN8N)
	}
	seen := map[string]struct{}{}
	for i, m := range a.Methods {
		if !slices.Contains(httpMethods, m) {
			r.errorf(CodeEnum, fmt.Sprintf("%s/methods/%d", path, i), "%q is not one of %s", m, strings.Join(httpMethods, ", "))
		}
		if _, dup := seen[m]; dup {
			r.errorf(CodeValue, fmt.Sprintf("%s/methods/%d", path, i), "method %s is listed twice", m)
		}
		seen[m] = struct{}{}
	}
}

func (v *validator) triggerRules(path string, rules []TriggerRule) {
	r := v.r
	if len(rules) == 0 {
		r.errorf(CodeValue, path, "at least one rule is required")
	}
	for i := range rules {
		rule, at := &rules[i], fmt.Sprintf("%s/%d", path, i)
		checkEnum(r, at+"/kind", rule.Kind, RuleKinds())
		for j, d := range rule.Weekdays {
			checkEnum(r, fmt.Sprintf("%s/weekdays/%d", at, j), d, Weekdays())
		}
		if rule.At != "" {
			checkPattern(r, at+"/at", "time of day", rule.At, timeOfDayPattern)
		}
		if rule.DayOfMonth > 31 {
			r.errorf(CodeValue, at+"/dayOfMonth", "must be from 1 to 31")
		}
		if rule.Kind == RuleCron {
			v.cron(at+"/cron", rule)
			continue
		}
		if rule.Cron != "" {
			r.errorf(CodeValue, at+"/cron", "cron applies to %s rules only", RuleCron)
		}
	}
}

// cron checks the shape of a cron expression: 5 or 6 fields made of the
// characters cron uses. Whether a field is in range is the schedule engine's
// job (cut T5); a wrong shape is caught here at publication.
func (v *validator) cron(path string, rule *TriggerRule) {
	fields := strings.Fields(rule.Cron)
	if len(fields) != 5 && len(fields) != 6 {
		v.r.errorf(CodeValue, path, "a cron expression has 5 or 6 fields, found %d", len(fields))
		return
	}
	for _, f := range fields {
		if !cronFieldCharacters.MatchString(f) {
			v.r.errorf(CodeValue, path, "cron field %q has characters cron does not use", f)
		}
	}
}
