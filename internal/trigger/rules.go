// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // the zone database ships in the binary (ADR-0025 section 1)

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// Codes of the trigger rules T-V1..T-V9 (ADR-0024 section 2).
const (
	// CodeTV1 is rule T-V1: a web address is not taken by another trigger.
	CodeTV1 = "T-V1"
	// CodeTV2 is rule T-V2: schedule and poll rules and the time zone are valid.
	CodeTV2 = "T-V2"
	// CodeTV3 is rule T-V3: the credentials a trigger needs are present.
	CodeTV3 = "T-V3"
	// CodeTV4 is rule T-V4: imported n8n addresses stay unchanged unless the
	// author edits them explicitly.
	CodeTV4 = "T-V4"
	// CodeTV5 is rule T-V5: a RESPOND_NODE trigger reaches a respond node.
	CodeTV5 = "T-V5"
	// CodeTV6 is rule T-V6: a local-file trigger is enabled and inside the allowed paths.
	CodeTV6 = "T-V6"
	// CodeTV7 is rule T-V7: the callers policy is valid.
	CodeTV7 = "T-V7"
	// CodeTV8 is rule T-V8: the trigger declaration of the node type is valid.
	CodeTV8 = "T-V8"
	// CodeTV9 is rule T-V9: the deduplication window is from 1 minute to 90 days.
	CodeTV9 = "T-V9"
)

// Bounds of the deduplication window (R-ING-03, T-V9).
const (
	// MinDedupeWindow is the shortest deduplication window.
	MinDedupeWindow = time.Minute
	// MaxDedupeWindow is the longest deduplication window.
	MaxDedupeWindow = 90 * 24 * time.Hour
)

const typeRespond = "orcon.respond"

// ruleContext carries what the document-level rules compare the draft with.
type ruleContext struct {
	org     identity.OrgID
	project identity.ProjectID
	// previous is the live version, for T-V4; nil on the first publication.
	previous *flow.Document
	// allowImportedAddressChange is the author's explicit confirmation of T-V4.
	allowImportedAddressChange bool
}

// documentIssues applies the rules T-V2..T-V9, which need only the document,
// the catalog, the credential store and the instance settings. T-V1 needs the
// registry and runs inside the publication transaction.
func (s *Service) documentIssues(ctx context.Context, doc *flow.Document, rc *ruleContext) ([]flow.Issue, error) {
	var issues []flow.Issue
	for i := range doc.Nodes {
		n := &doc.Nodes[i]
		if n.Trigger == nil {
			continue
		}
		base := fmt.Sprintf("/nodes/%d/trigger", i)
		add := func(sev flow.Severity, code, at, format string, args ...any) {
			issues = append(issues, flow.Issue{Severity: sev, Code: code, Path: base + at, Message: fmt.Sprintf(format, args...)})
		}
		s.declarationIssues(ctx, n, add)
		scheduleIssues(n.Trigger, add)
		if err := s.credentialIssues(ctx, rc, n, add); err != nil {
			return nil, err
		}
		importedAddressIssues(doc, n, rc, add)
		respondIssues(doc, n, add)
		s.localFileIssues(n.Trigger, add)
		callersIssues(n.Trigger, add)
		dedupeWindowIssues(n.Trigger, add)
	}
	return issues, nil
}

type addIssue func(sev flow.Severity, code, at, format string, args ...any)

// declarationIssues is T-V8 (ADR-0020 section 3).
func (s *Service) declarationIssues(ctx context.Context, n *flow.Node, add addIssue) {
	t := n.Trigger
	decl, ok := s.catalog.LookupTrigger(ctx, n.Type)
	if !ok {
		add(flow.SeverityError, CodeTV8, "", "node type %q declares no trigger, so it cannot be published as a %s trigger", n.Type, t.Family)
		return
	}
	if decl.Family != t.Family {
		add(flow.SeverityError, CodeTV8, "/family", "node type %q is a %s trigger, not %s", n.Type, decl.Family, t.Family)
	}
	if !slices.Contains(flow.TriggerFamilies(), decl.Family) {
		add(flow.SeverityError, CodeTV8, "", "the declaration of %q has the unknown family %q", n.Type, decl.Family)
	}
	if !slices.Contains(Deliveries(), decl.Delivery) {
		add(flow.SeverityError, CodeTV8, "", "the declaration of %q must state a delivery guarantee (one of %s)", n.Type, joinNames(Deliveries()))
	}
	if decl.DedupeKeySource != "" && !slices.Contains(flow.DedupeSources(), decl.DedupeKeySource) {
		add(flow.SeverityError, CodeTV8, "", "the declaration of %q has the unknown deduplication key source %q", n.Type, decl.DedupeKeySource)
	}
	if decl.MaxPayloadBytes < 1 {
		add(flow.SeverityError, CodeTV8, "", "the declaration of %q must state a maximum payload of at least 1 byte", n.Type)
	}
	if d := t.Dedupe; d != nil && decl.DedupeKeySource != "" && d.Source != decl.DedupeKeySource {
		add(flow.SeverityError, CodeTV8, "/dedupe/source", "node type %q fixes the deduplication key source to %s, not %s", n.Type, decl.DedupeKeySource, d.Source)
	}
}

// scheduleIssues is T-V2: rules and time zone of schedules and polls.
func scheduleIssues(t *flow.Trigger, add addIssue) {
	if s := t.Schedule; s != nil {
		if s.TimeZone != "" {
			if s.TimeZone == "Local" {
				add(flow.SeverityError, CodeTV2, "/schedule/timeZone", "the time zone must be an IANA name, not Local")
			} else if _, err := time.LoadLocation(s.TimeZone); err != nil {
				add(flow.SeverityError, CodeTV2, "/schedule/timeZone", "%q is not an IANA time zone", s.TimeZone)
			}
		}
		for i := range s.Rules {
			ruleIssues(&s.Rules[i], fmt.Sprintf("/schedule/rules/%d", i), add)
		}
	}
	if p := t.Poll; p != nil {
		for i := range p.Rules {
			ruleIssues(&p.Rules[i], fmt.Sprintf("/poll/rules/%d", i), add)
		}
	}
}

func ruleIssues(r *flow.TriggerRule, at string, add addIssue) {
	bad := func(format string, args ...any) { add(flow.SeverityError, CodeTV2, at, format, args...) }
	switch r.Kind {
	case flow.RuleSeconds, flow.RuleMinutes, flow.RuleHours:
		if r.At != "" || len(r.Weekdays) > 0 || r.DayOfMonth != 0 {
			bad("a %s rule runs every N units and takes no time of day, weekday or day of month", r.Kind)
		}
	case flow.RuleDays:
		if len(r.Weekdays) > 0 || r.DayOfMonth != 0 {
			bad("a DAYS rule takes no weekday or day of month")
		}
	case flow.RuleWeeks:
		if len(r.Weekdays) == 0 {
			bad("a WEEKS rule needs at least one weekday")
		}
		if r.DayOfMonth != 0 {
			bad("a WEEKS rule takes no day of month")
		}
	case flow.RuleMonths:
		if r.DayOfMonth == 0 {
			bad("a MONTHS rule needs a day of the month")
		}
		if len(r.Weekdays) > 0 {
			bad("a MONTHS rule takes no weekday")
		}
	case flow.RuleCron:
		if r.Every != 0 || r.At != "" || len(r.Weekdays) > 0 || r.DayOfMonth != 0 {
			bad("a CRON rule takes only the expression")
		}
		if err := validateCron(r.Cron); err != nil {
			bad("%v", err)
		}
	}
}

// credentialIssues is T-V3.
func (s *Service) credentialIssues(ctx context.Context, rc *ruleContext, n *flow.Node, add addIssue) error {
	if !slices.Contains([]flow.TriggerFamily{flow.TriggerPoll, flow.TriggerListen, flow.TriggerAppEvent}, n.Trigger.Family) {
		return nil
	}
	missing, err := s.credentials.Missing(ctx, rc.org, rc.project, n.Type, n.Parameters)
	if err != nil {
		return fmt.Errorf("check the credentials of node %q: %w", n.ID, err)
	}
	for _, name := range missing {
		add(flow.SeverityError, CodeTV3, "", "the %s trigger needs the credential %q, which the project cannot use", n.Trigger.Family, name)
	}
	return nil
}

// importedAddressIssues is T-V4: an n8n address of a flow imported from n8n is
// kept as it was published unless the author confirms the change.
func importedAddressIssues(doc *flow.Document, n *flow.Node, rc *ruleContext, add addIssue) {
	if rc.previous == nil || doc.Settings.ImportedFrom == nil || rc.allowImportedAddressChange {
		return
	}
	var old *flow.Node
	for i := range rc.previous.Nodes {
		if rc.previous.Nodes[i].ID == n.ID {
			old = &rc.previous.Nodes[i]
			break
		}
	}
	if old == nil || old.Trigger == nil {
		return
	}
	for _, oa := range old.Trigger.Addresses {
		if oa.Family != flow.AddressN8N {
			continue
		}
		kept := slices.ContainsFunc(n.Trigger.Addresses, func(a flow.TriggerAddress) bool { return sameN8NAddress(oa, a) })
		if !kept {
			add(flow.SeverityError, CodeTV4, "/addresses", "the imported n8n address /%s was removed or changed; confirm the change explicitly to publish it", oa.Path)
		}
	}
}

func sameN8NAddress(a, b flow.TriggerAddress) bool {
	return b.Family == flow.AddressN8N && a.Path == b.Path && a.WebhookID == b.WebhookID &&
		slices.Equal(methodsOf(a), methodsOf(b)) && profileOf(a) == profileOf(b)
}

func methodsOf(a flow.TriggerAddress) []string {
	if len(a.Methods) == 0 {
		return []string{"POST"}
	}
	m := slices.Clone(a.Methods)
	slices.Sort(m)
	return m
}

func profileOf(a flow.TriggerAddress) flow.ResponseProfile {
	if a.ResponseProfile == "" {
		return flow.ResponseProfile(a.Family)
	}
	return a.ResponseProfile
}

// respondIssues is T-V5: a warning, because a respond node placed behind a
// condition may still be reached at run time.
func respondIssues(doc *flow.Document, n *flow.Node, add addIssue) {
	r := n.Trigger.Response
	if r == nil || r.Mode != flow.ResponseRespondNode {
		return
	}
	next := map[flow.NodeID][]flow.NodeID{}
	for _, e := range doc.Edges {
		next[e.From.Node] = append(next[e.From.Node], e.To)
	}
	byID := map[flow.NodeID]*flow.Node{}
	for i := range doc.Nodes {
		byID[doc.Nodes[i].ID] = &doc.Nodes[i]
	}
	seen := map[flow.NodeID]bool{n.ID: true}
	queue := []flow.NodeID{n.ID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, to := range next[cur] {
			if seen[to] {
				continue
			}
			seen[to] = true
			if node := byID[to]; node != nil && typeName(node.Type) == typeRespond {
				return
			}
			queue = append(queue, to)
		}
	}
	add(flow.SeverityWarning, CodeTV5, "/response/mode", "the trigger answers with a respond node but no %s node follows it, so every request waits until the time limit", typeRespond)
}

func typeName(nodeType string) string {
	name, _, _ := strings.Cut(nodeType, "@")
	return name
}

// localFileIssues is T-V6 (R-LSN-07).
func (s *Service) localFileIssues(t *flow.Trigger, add addIssue) {
	l := t.Listen
	if l == nil || l.LocalPath == "" {
		return
	}
	bad := func(format string, args ...any) {
		add(flow.SeverityError, CodeTV6, "/listen/localPath", format, args...)
	}
	if !s.settings.LocalFileEnabled {
		bad("the local-file trigger is disabled on this instance; an operator must enable it and list the allowed paths")
		return
	}
	if slices.Contains(strings.Split(l.LocalPath, "/"), "..") || !strings.HasPrefix(l.LocalPath, "/") {
		bad("the path %q must be absolute and without .. segments", l.LocalPath)
		return
	}
	clean := path.Clean(l.LocalPath)
	for _, allowed := range s.settings.LocalFileAllowedPaths {
		a := path.Clean(allowed)
		if clean == a || strings.HasPrefix(clean, strings.TrimSuffix(a, "/")+"/") {
			return
		}
	}
	bad("the path %q is outside the paths the operator allows for local-file triggers", l.LocalPath)
}

// callersIssues is T-V7.
func callersIssues(t *flow.Trigger, add addIssue) {
	c := t.Callers
	if c == nil || c.Project {
		return
	}
	if len(c.Flows) == 0 {
		add(flow.SeverityError, CodeTV7, "/callers", "the callers list is empty; use PROJECT to allow every flow of the project")
	}
	seen := map[flow.ID]bool{}
	for i, id := range c.Flows {
		if !flowIDPattern.MatchString(string(id)) {
			add(flow.SeverityError, CodeTV7, fmt.Sprintf("/callers/%d", i), "%q is not a flow id", id)
		}
		if seen[id] {
			add(flow.SeverityError, CodeTV7, fmt.Sprintf("/callers/%d", i), "flow %s is listed twice", id)
		}
		seen[id] = true
	}
}

var flowIDPattern = regexp.MustCompile(flow.UUIDPattern)

// dedupeWindowIssues is T-V9.
func dedupeWindowIssues(t *flow.Trigger, add addIssue) {
	if t.Dedupe == nil || t.Dedupe.Window == "" {
		return
	}
	window, err := ParseDuration(t.Dedupe.Window)
	if err != nil {
		return // the shape error is the flow area's finding
	}
	if window < MinDedupeWindow || window > MaxDedupeWindow {
		add(flow.SeverityError, CodeTV9, "/dedupe/window", "the deduplication window %s must be from 1 minute to 90 days", t.Dedupe.Window)
	}
}

func joinNames[T ~string](items []T) string {
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = string(it)
	}
	return strings.Join(names, ", ")
}

// ParseDuration reads an ISO 8601 duration of weeks, days, hours, minutes and
// seconds, the form the flow format allows (years and months vary in length).
func ParseDuration(s string) (time.Duration, error) {
	m := durationParts.FindStringSubmatch(s)
	if m == nil || s == "P" || strings.HasSuffix(s, "T") {
		return 0, fmt.Errorf("%q is not an ISO 8601 duration of weeks, days, hours, minutes and seconds", s)
	}
	units := []time.Duration{7 * 24 * time.Hour, 24 * time.Hour, time.Hour, time.Minute, time.Second}
	var total float64
	for i, u := range units {
		if m[i+1] == "" {
			continue
		}
		v, err := strconv.ParseFloat(m[i+1], 64)
		if err != nil {
			return 0, fmt.Errorf("parse duration %q: %w", s, err)
		}
		total += v * float64(u)
	}
	return time.Duration(total), nil
}

var durationParts = regexp.MustCompile(`^P(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)

// validateCron checks the fields of a cron expression of 5 fields (minute,
// hour, day of month, month, day of week) or 6 (seconds first). It accepts
// numbers, names of months and weekdays, ranges, lists, steps and the star; the
// schedule engine of cut T5 may accept more.
func validateCron(expr string) error {
	fields := strings.Fields(expr)
	if len(fields) != 5 && len(fields) != 6 {
		return fmt.Errorf("a cron expression has 5 or 6 fields, found %d", len(fields))
	}
	limits := []struct {
		name     string
		min, max int
		names    []string
	}{
		{"seconds", 0, 59, nil},
		{"minutes", 0, 59, nil},
		{"hours", 0, 23, nil},
		{"day of month", 1, 31, nil},
		{"month", 1, 12, []string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}},
		{"day of week", 0, 7, []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}},
	}
	if len(fields) == 5 {
		limits = limits[1:]
	}
	for i, f := range fields {
		if err := cronField(f, limits[i].min, limits[i].max, limits[i].names); err != nil {
			return fmt.Errorf("cron %s field %q: %w", limits[i].name, f, err)
		}
	}
	return nil
}

func cronField(field string, lo, hi int, names []string) error {
	for _, part := range strings.Split(field, ",") {
		rangePart, step, hasStep := strings.Cut(part, "/")
		if hasStep {
			n, err := strconv.Atoi(step)
			if err != nil || n < 1 {
				return errors.New("the step must be a positive number")
			}
		}
		if rangePart == "*" {
			continue
		}
		from, to, isRange := strings.Cut(rangePart, "-")
		a, err := cronValue(from, lo, hi, names)
		if err != nil {
			return err
		}
		if !isRange {
			continue
		}
		b, err := cronValue(to, lo, hi, names)
		if err != nil {
			return err
		}
		if a > b {
			return errors.New("the range runs backwards")
		}
	}
	return nil
}

func cronValue(s string, lo, hi int, names []string) (int, error) {
	if i := slices.Index(names, strings.ToUpper(s)); i >= 0 {
		if len(names) == 12 {
			return i + 1, nil
		}
		return i, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < lo || n > hi {
		return 0, fmt.Errorf("%q must be a number from %d to %d", s, lo, hi)
	}
	return n, nil
}
