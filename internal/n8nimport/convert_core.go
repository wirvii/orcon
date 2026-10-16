// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Orcon types of the core nodes the converter maps today. The format types
// (if, switch) and the engine types (merge, wait, the triggers) are defined by
// SPEC-001 and SPEC-002; the rest of the core table waits for its cut.
const (
	typeOrconIf       = "orcon.if@1"
	typeOrconSwitch   = "orcon.switch@1"
	typeOrconMerge    = "orcon.merge@1"
	typeOrconWait     = "orcon.wait@1"
	typeOrconManual   = "orcon.manual@1"
	typeOrconWebhook  = "orcon.webhook@1"
	typeOrconSchedule = "orcon.schedule@1"
)

// convertCore maps a core node. Core types outside the list below are kept as
// unsupported with the cut that delivers them.
func (c *converter) convertCore(n *Node, row CoreNode) convResult {
	switch n.Type {
	case basePrefix + "if":
		return c.convertIf(n)
	case basePrefix + "switch":
		return c.convertSwitch(n)
	case basePrefix + "merge":
		return c.convertMerge(n)
	case basePrefix + "wait":
		return c.convertWait(n)
	case basePrefix + "manualTrigger", basePrefix + "start":
		return c.convertManual(n)
	case basePrefix + "webhook":
		return c.convertWebhook(n)
	case basePrefix + "scheduleTrigger":
		return c.convertSchedule(n)
	case basePrefix + "noOp":
		return c.convertNoOp(n)
	case basePrefix + "set":
		return c.convertSet(n)
	case basePrefix + "httpRequest":
		return c.convertHTTP(n)
	}
	element := flow.ElementTask
	if row.Disposition == DispTrigger {
		element = flow.ElementStartEvent
	}
	target := row.Orcon
	if target == "" {
		target = "no Orcon counterpart"
	}
	msg := fmt.Sprintf("%s becomes %s, delivered by %s, which is not on main yet", n.Type, target, row.Cut)
	if row.Note != "" {
		msg += " (" + row.Note + ")"
	}
	return c.unsupported(n, []Finding{{Code: FindNotOnMain, NodeID: n.ID, Node: n.Name, Field: "type", Message: msg}}, element)
}

// filterFinding reports a condition the converter cannot translate. A
// comparison value that looks like a secret gets its own code and never shows
// the value.
func filterFinding(n *Node, field string, err error) Finding {
	if errors.Is(err, errSecretLiteral) {
		return Finding{Code: FindSecretValue, NodeID: n.ID, Node: n.Name, Field: field,
			Message: "a comparison value of " + field + " looks like a secret and is not kept; the node is unsupported until the value is a reference to a credential"}
	}
	return paramFinding(n, field, err.Error())
}

// paramFinding reports a parameter the converter cannot translate.
func paramFinding(n *Node, field, msg string) Finding {
	return Finding{Code: FindUnsupportedParameter, NodeID: n.ID, Node: n.Name, Field: field, Message: msg}
}

// params returns the parameters object of the node (an empty object when the
// workflow omits it).
func params(n *Node) jsonvalue.Value {
	if n.Parameters.IsZero() {
		return emptyObject()
	}
	return n.Parameters
}

// stringParam reads a literal string parameter. It reports false for a
// missing value and for an expression, which the converter cannot evaluate.
func stringParam(p jsonvalue.Value, key string) (string, bool) {
	v, ok := p.Get(key)
	if !ok {
		return "", false
	}
	s, ok := v.Str()
	if !ok || strings.HasPrefix(s, "=") {
		return "", false
	}
	return s, true
}

// routingNode builds an item-routing node with one rule per entry.
func routingNode(element flow.Element, typ string, routing flow.Routing, rules []jsonvalue.Value, ext jsonvalue.Value) flow.Node {
	pv, _ := jsonvalue.Object(jsonvalue.Member{Key: "rules", Value: jsonvalue.Array(rules...)})
	return flow.Node{Element: element, Type: typ, Parameters: pv, Routing: routing, Ext: ext}
}

// caseRule is one rule of an item-routing node: the port to take and the
// boolean that selects it.
func caseRule(port, js string) jsonvalue.Value {
	v, _ := jsonvalue.Object(
		jsonvalue.Member{Key: "port", Value: jsonvalue.String(port)},
		jsonvalue.Member{Key: "when", Value: jsonvalue.String(whenParam(js))},
	)
	return v
}

// convertIf maps an IF node of version 2 or later. Its true output is the
// "case:true" port; the false output is the default port.
func (c *converter) convertIf(n *Node) convResult {
	if ver, _ := strconv.ParseFloat(n.TypeVersion, 64); ver < 2 {
		return c.unsupported(n, []Finding{paramFinding(n, "typeVersion",
			"IF version 1 keeps its conditions in another shape; only version 2 and later are converted")}, flow.ElementTask)
	}
	cond, ok := params(n).Get("conditions")
	if !ok || cond.Kind() != jsonvalue.KindObject {
		return c.unsupported(n, []Finding{paramFinding(n, "parameters.conditions", "the IF node has no conditions object")}, flow.ElementTask)
	}
	js, err := filterJS(cond)
	if err != nil {
		return c.unsupported(n, []Finding{filterFinding(n, "parameters.conditions", err)}, flow.ElementTask)
	}
	rule := caseRule("case:true", js)
	return convResult{
		node:      routingNode(flow.ElementExclusiveGateway, typeOrconIf, flow.RoutingExclusive, []jsonvalue.Value{rule}, importMembers(n)),
		orconType: typeOrconIf,
		ports:     []string{"case:true", "main"},
		routed:    true,
	}
}

// convertSwitch maps a Switch node of version 3 in rules mode. Rule i takes
// the port "case:i"; with the fallback output on, the default port is the one
// after the last rule.
func (c *converter) convertSwitch(n *Node) convResult {
	if ver, _ := strconv.ParseFloat(n.TypeVersion, 64); ver < 3 {
		return c.unsupported(n, []Finding{paramFinding(n, "typeVersion",
			"only Switch version 3 and later are converted")}, flow.ElementTask)
	}
	p := params(n)
	if mode, ok := stringParam(p, "mode"); ok && mode != "" && mode != "rules" {
		return c.unsupported(n, []Finding{paramFinding(n, "parameters.mode",
			"only the rules mode is converted; the "+mode+" mode is not")}, flow.ElementTask)
	}
	rulesV, _ := p.Get("rules")
	values, _ := rulesV.Get("values")
	if values.Kind() != jsonvalue.KindArray || values.Len() == 0 {
		return c.unsupported(n, []Finding{paramFinding(n, "parameters.rules.values", "the Switch node has no rules")}, flow.ElementTask)
	}
	options, _ := p.Get("options")
	fallback, _ := stringParam(options, "fallbackOutput")
	if fallback != "" && fallback != "none" && fallback != "extra" {
		return c.unsupported(n, []Finding{paramFinding(n, "parameters.options.fallbackOutput",
			"only the none and extra fallbacks are converted; a numbered fallback is not")}, flow.ElementTask)
	}
	allMatching := false
	if v, ok := options.Get("allMatchingOutputs"); ok {
		allMatching, _ = v.Bool()
	}
	var rules []jsonvalue.Value
	ports := make([]string, 0, values.Len()+1)
	for i, item := range values.Items() {
		cond, ok := item.Get("conditions")
		if !ok || cond.Kind() != jsonvalue.KindObject {
			return c.unsupported(n, []Finding{paramFinding(n, fmt.Sprintf("parameters.rules.values[%d]", i),
				"the rule has no conditions object")}, flow.ElementTask)
		}
		js, err := filterJS(cond)
		if err != nil {
			return c.unsupported(n, []Finding{filterFinding(n, fmt.Sprintf("parameters.rules.values[%d].conditions", i), err)}, flow.ElementTask)
		}
		port := "case:" + strconv.Itoa(i)
		rules = append(rules, caseRule(port, js))
		ports = append(ports, port)
	}
	if fallback == "extra" {
		ports = append(ports, "main")
	}
	routing, element := flow.RoutingExclusive, flow.ElementExclusiveGateway
	if allMatching {
		routing, element = flow.RoutingInclusive, flow.ElementInclusiveGateway
	}
	return convResult{
		node:      routingNode(element, typeOrconSwitch, routing, rules, importMembers(n)),
		orconType: typeOrconSwitch,
		ports:     ports,
		routed:    true,
	}
}

// convertMerge maps the append mode of a Merge node to a join that fires for
// the branches that run. The other modes combine items by field or position
// and are not converted.
func (c *converter) convertMerge(n *Node) convResult {
	if mode, ok := stringParam(params(n), "mode"); ok && mode != "" && mode != "append" {
		return c.unsupported(n, []Finding{paramFinding(n, "parameters.mode",
			"only the append mode is converted; the "+mode+" mode combines items by field or position and is not")}, flow.ElementTask)
	}
	return convResult{
		node: flow.Node{
			Element: flow.ElementTask, Type: typeOrconMerge, Parameters: emptyObject(),
			Join: &flow.Join{Mode: flow.JoinActivated, Data: flow.JoinDataAppend},
			Ext:  importMembers(n),
		},
		orconType: typeOrconMerge,
		findings: []Finding{{Code: FindAdapted, NodeID: n.ID, Node: n.Name, Field: "parameters.mode",
			Message: "the join fires for the branches that run (ACTIVATED); compare the merged items with the n8n run for a branch that does not run"}},
	}
}

// convertWait maps a Wait node that resumes after a time interval. Resuming on
// a webhook or a form is not converted.
func (c *converter) convertWait(n *Node) convResult {
	p := params(n)
	if resume, ok := stringParam(p, "resume"); ok && resume != "" && resume != "timeInterval" {
		return c.unsupported(n, []Finding{paramFinding(n, "parameters.resume",
			"only the time interval resume is converted; the "+resume+" resume waits for an outside call")}, flow.ElementTask)
	}
	amount := 1.0
	if v, ok := p.Get("amount"); ok {
		f, isNum := v.Float64()
		if !isNum || f < 1 || f != math.Trunc(f) || f > 1e9 {
			return c.unsupported(n, []Finding{paramFinding(n, "parameters.amount",
				"the amount must be a whole number of at least 1 and not an expression")}, flow.ElementTask)
		}
		amount = f
	}
	unit, _ := stringParam(p, "unit")
	if unit == "" {
		unit = "hours"
	}
	count := strconv.FormatInt(int64(amount), 10)
	var duration string
	switch unit {
	case "seconds":
		duration = "PT" + count + "S"
	case "minutes":
		duration = "PT" + count + "M"
	case "hours":
		duration = "PT" + count + "H"
	case "days":
		duration = "P" + count + "D"
	default:
		return c.unsupported(n, []Finding{paramFinding(n, "parameters.unit",
			"the unit "+unit+" is not converted; seconds, minutes, hours and days are")}, flow.ElementTask)
	}
	return convResult{
		node: flow.Node{
			Element: flow.ElementIntermediateCatchEvent, Type: typeOrconWait, Parameters: emptyObject(),
			Wait: &flow.Wait{Kind: flow.WaitTimer, Timer: &flow.Timer{Duration: duration}},
			Ext:  importMembers(n),
		},
		orconType: typeOrconWait,
	}
}

// convertManual maps the manual trigger, and the start node of old n8n flows,
// to a manual run of the flow.
func (c *converter) convertManual(n *Node) convResult {
	return convResult{
		node: flow.Node{
			Element: flow.ElementStartEvent, Type: typeOrconManual, Parameters: emptyObject(),
			Trigger: &flow.Trigger{Family: flow.TriggerInternal},
			Ext:     importMembers(n),
		},
		orconType: typeOrconManual,
	}
}

// convertWebhook maps a webhook trigger that answers on receipt or after the
// last node. Answering from a Respond to Webhook node is not converted yet:
// that node is not on main.
func (c *converter) convertWebhook(n *Node) convResult {
	p := params(n)
	path, ok := stringParam(p, "path")
	if !ok || path == "" {
		path = n.WebhookID
	}
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return c.unsupported(n, []Finding{paramFinding(n, "parameters.path",
			"the webhook has no path, or the path is an expression")}, flow.ElementStartEvent)
	}
	methods := webhookMethods(p)
	mode, findings, ok := responseMode(n, p)
	if !ok {
		return c.unsupported(n, findings, flow.ElementStartEvent)
	}
	addr := flow.TriggerAddress{
		Family:          flow.AddressN8N,
		Path:            path,
		WebhookID:       n.WebhookID,
		Methods:         methods,
		ResponseProfile: flow.ProfileN8N,
	}
	return convResult{
		node: flow.Node{
			Element: flow.ElementStartEvent, Type: typeOrconWebhook, Parameters: emptyObject(),
			Trigger: &flow.Trigger{
				Family:    flow.TriggerWebhook,
				Addresses: []flow.TriggerAddress{addr},
				Response:  &flow.TriggerResponse{Mode: mode},
			},
			Ext: importMembers(n),
		},
		orconType: typeOrconWebhook,
		findings:  findings,
	}
}

// The methods of a webhook come from its httpMethod parameter.
// N8n uses GET when the workflow leaves it out.
func webhookMethods(p jsonvalue.Value) []string {
	v, ok := p.Get("httpMethod")
	if !ok {
		return []string{"GET"}
	}
	if s, isStr := v.Str(); isStr {
		return []string{strings.ToUpper(s)}
	}
	var out []string
	for _, item := range v.Items() {
		if s, isStr := item.Str(); isStr {
			out = append(out, strings.ToUpper(s))
		}
	}
	if len(out) == 0 {
		return []string{"GET"}
	}
	return out
}

// The response mode of a webhook maps to the Orcon mode.
// N8n answers on receipt by default.
func responseMode(n *Node, p jsonvalue.Value) (flow.ResponseMode, []Finding, bool) {
	mode, _ := stringParam(p, "responseMode")
	switch mode {
	case "", "onReceived":
		return flow.ResponseImmediate, nil, true
	case "lastNode":
		return flow.ResponseLastNode, nil, true
	case "responseNode":
		return flow.ResponseRespondNode, nil, true
	default:
		return "", []Finding{paramFinding(n, "parameters.responseMode",
			"the response mode "+mode+" is not converted; onReceived, lastNode and responseNode are")}, false
	}
}

// convertSchedule maps a schedule trigger with interval rules in seconds,
// minutes, hours and days, and cron expressions. Weekly and monthly rules
// are not converted.
func (c *converter) convertSchedule(n *Node) convResult {
	rule, ok := params(n).Get("rule")
	intervals, _ := rule.Get("interval")
	if !ok || intervals.Kind() != jsonvalue.KindArray || intervals.Len() == 0 {
		return c.unsupported(n, []Finding{paramFinding(n, "parameters.rule.interval", "the schedule has no rule")}, flow.ElementStartEvent)
	}
	var rules []flow.TriggerRule
	for i, item := range intervals.Items() {
		r, err := scheduleRule(item)
		if err != nil {
			return c.unsupported(n, []Finding{paramFinding(n, fmt.Sprintf("parameters.rule.interval[%d]", i), err.Error())}, flow.ElementStartEvent)
		}
		rules = append(rules, r)
	}
	return convResult{
		node: flow.Node{
			Element: flow.ElementStartEvent, Type: typeOrconSchedule, Parameters: emptyObject(),
			Trigger: &flow.Trigger{
				Family:   flow.TriggerSchedule,
				Schedule: &flow.TriggerScheduleSpec{TimeZone: c.scheduleZone(), Rules: rules},
			},
			Ext: importMembers(n),
		},
		orconType: typeOrconSchedule,
	}
}

// scheduleZone is the time zone of the workflow, empty when n8n used the zone
// of the instance.
func (c *converter) scheduleZone() string {
	if c.wf.timezone == "default" {
		return ""
	}
	return c.wf.timezone
}

// scheduleRule maps one interval item of a schedule trigger.
func scheduleRule(item jsonvalue.Value) (flow.TriggerRule, error) {
	field := strValue(item, "field")
	every := func(key string) (int, error) {
		v, ok := item.Get(key)
		if !ok {
			return 1, nil
		}
		f, isNum := v.Float64()
		if !isNum || f < 1 || f != math.Trunc(f) || f > 1e6 {
			return 0, fmt.Errorf("%s must be a whole number of at least 1", key)
		}
		return int(f), nil
	}
	clock := func() (string, error) {
		hour, minute := 0, 0
		if v, ok := item.Get("triggerAtHour"); ok {
			f, _ := v.Float64()
			hour = int(f)
		}
		if v, ok := item.Get("triggerAtMinute"); ok {
			f, _ := v.Float64()
			minute = int(f)
		}
		if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
			return "", fmt.Errorf("the time of day %d:%d is out of range", hour, minute)
		}
		return fmt.Sprintf("%02d:%02d", hour, minute), nil
	}
	switch field {
	case "seconds":
		n, err := every("secondsInterval")
		return flow.TriggerRule{Kind: flow.RuleSeconds, Every: n}, err
	case "minutes":
		n, err := every("minutesInterval")
		return flow.TriggerRule{Kind: flow.RuleMinutes, Every: n}, err
	case "hours":
		n, err := every("hoursInterval")
		if v, ok := item.Get("triggerAtMinute"); ok {
			if f, _ := v.Float64(); f != 0 {
				return flow.TriggerRule{}, errors.New("an hourly rule that starts at a minute other than 0 is not converted")
			}
		}
		return flow.TriggerRule{Kind: flow.RuleHours, Every: n}, err
	case "days":
		n, err := every("daysInterval")
		if err != nil {
			return flow.TriggerRule{}, err
		}
		at, err := clock()
		return flow.TriggerRule{Kind: flow.RuleDays, Every: n, At: at}, err
	case "cronExpression":
		expr, ok := stringParam(item, "expression")
		if !ok || expr == "" {
			return flow.TriggerRule{}, errors.New("the cron expression is missing or is an expression")
		}
		return flow.TriggerRule{Kind: flow.RuleCron, Cron: expr}, nil
	default:
		return flow.TriggerRule{}, fmt.Errorf("the interval %q is not converted; seconds, minutes, hours, days and cron are", field)
	}
}
