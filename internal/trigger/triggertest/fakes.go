// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package triggertest

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/trigger"
)

// Catalog is a [trigger.Catalog] backed by a map from node type to
// declaration.
type Catalog map[string]trigger.Declaration

// LookupTrigger implements [trigger.Catalog].
func (c Catalog) LookupTrigger(_ context.Context, nodeType string) (trigger.Declaration, bool) {
	d, ok := c[nodeType]
	return d, ok
}

// Node types the default catalog declares.
const (
	TypeWebhook   = "orcon.webhook@1"
	TypeSchedule  = "orcon.schedule@1"
	TypePoll      = "orcon.poll-feed@1"
	TypeLocalFile = "orcon.local-file@1"
	TypeAppEvent  = "orcon.app-event@1"
	TypeManual    = "orcon.manual@1"
	TypeCalled    = "orcon.called-by-flow@1"
)

// DefaultCatalog declares one valid trigger type per family.
func DefaultCatalog() Catalog {
	const mib = 1 << 20
	return Catalog{
		TypeWebhook:   {Family: flow.TriggerWebhook, Delivery: trigger.DeliveryAtLeastOnce, MaxPayloadBytes: 16 * mib},
		TypeSchedule:  {Family: flow.TriggerSchedule, Delivery: trigger.DeliveryCursorExact, MaxPayloadBytes: mib},
		TypePoll:      {Family: flow.TriggerPoll, Delivery: trigger.DeliveryCursorExact, MaxPayloadBytes: mib},
		TypeLocalFile: {Family: flow.TriggerListen, Delivery: trigger.DeliveryCursorExact, MaxPayloadBytes: mib},
		TypeAppEvent:  {Family: flow.TriggerAppEvent, Delivery: trigger.DeliveryAtLeastOnce, MaxPayloadBytes: 16 * mib, DedupeKeySource: flow.DedupeConnector},
		TypeManual:    {Family: flow.TriggerInternal, Delivery: trigger.DeliveryAtLeastOnce, MaxPayloadBytes: mib},
		TypeCalled:    {Family: flow.TriggerInternal, Delivery: trigger.DeliveryAtLeastOnce, MaxPayloadBytes: mib},
	}
}

// Credentials is a [trigger.Credentials] that reports a fixed list of
// credentials as missing.
type Credentials struct {
	mu      sync.Mutex
	missing []string
}

// SetMissing sets the credentials every trigger node reports as missing.
func (c *Credentials) SetMissing(names ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.missing = names
}

// Missing implements [trigger.Credentials].
func (c *Credentials) Missing(context.Context, identity.OrgID, identity.ProjectID, string, jsonvalue.Value) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.missing...), nil
}

// StartedRun records the manual runs a [RecordingStarter] was asked to start.
type StartedRun struct {
	Run   trigger.ManualRun
	Input []byte
}

// StartedTrigger is one execution a [RecordingStarter] started for an event.
type StartedTrigger struct {
	Run   trigger.TriggeredRun
	Input []byte
}

// RecordingStarter is a [trigger.Starter] that records the runs it is given.
type RecordingStarter struct {
	mu        sync.Mutex
	Runs      []StartedRun
	Triggered []StartedTrigger
}

// StartTriggered implements [trigger.Starter].
func (s *RecordingStarter) StartTriggered(_ context.Context, run trigger.TriggeredRun, input []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Triggered = append(s.Triggered, StartedTrigger{Run: run, Input: input})
	return fmt.Sprintf("triggered-%d", len(s.Triggered)), nil
}

// StartManual implements [trigger.Starter].
func (s *RecordingStarter) StartManual(_ context.Context, run trigger.ManualRun, input []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Runs = append(s.Runs, StartedRun{Run: run, Input: input})
	return fmt.Sprintf("execution-%d", len(s.Runs)), nil
}

// Node describes one node of a document built by [Build].
type Node struct {
	ID string
	// Type is the node type; [TypeWebhook] when empty.
	Type string
	// Block is the JSON text of the trigger block; none when empty.
	Block string
	// Parameters is the JSON text of the parameters object; {} when empty.
	Parameters string
	// Next is the id of the node the start event leads to; "end" when empty.
	Next string
}

// DocSpec describes a document for [Build].
type DocSpec struct {
	// Name is the name of the flow document. It changes the content hash.
	Name string
	// Imported marks the document as imported from n8n (rule T-V4).
	Imported bool
	// Starts are the start events.
	Starts []Node
	// Extra holds JSON texts of further nodes, with their edges in ExtraEdges.
	Extra      []string
	ExtraEdges []string
}

const documentID = "01930000-0000-7000-8000-0000000000aa"

// Build returns the JSON text of a valid flow document: the start events, an
// end event they all lead to, and the extras.
func Build(spec DocSpec) []byte {
	var nodes, edges []string
	for _, n := range spec.Starts {
		typ, params, next := n.Type, n.Parameters, n.Next
		if typ == "" {
			typ = TypeWebhook
		}
		if params == "" {
			params = "{}"
		}
		if next == "" {
			next = "end"
		}
		block := ""
		if n.Block != "" {
			block = `,"trigger":` + n.Block
		}
		nodes = append(nodes, fmt.Sprintf(`{"id":%q,"name":%q,"element":"startEvent","type":%q,"parameters":%s%s}`, n.ID, n.ID, typ, params, block))
		edges = append(edges, fmt.Sprintf(`{"id":"e-%s","from":{"node":%q,"port":"main"},"to":{"node":%q},"kind":"SEQUENCE"}`, n.ID, n.ID, next))
	}
	nodes = append(nodes, spec.Extra...)
	nodes = append(nodes, `{"id":"end","name":"End","element":"endEvent","type":"orcon.end@1","parameters":{}}`)
	edges = append(edges, spec.ExtraEdges...)
	settings := `{}`
	if spec.Imported {
		settings = `{"importedFrom":{"sourceVersion":"1.0.0","reportId":"r1"}}`
	}
	return []byte(fmt.Sprintf(`{"format":"orcon/flow@1","id":%q,"name":%q,"settings":%s,"variables":[],"nodes":[%s],"edges":[%s],"editor":{}}`,
		documentID, spec.Name, settings, strings.Join(nodes, ","), strings.Join(edges, ",")))
}

// WebhookBlock returns a trigger block with one native address.
func WebhookBlock(path string, extra ...string) string {
	tail := ""
	if len(extra) > 0 {
		tail = "," + strings.Join(extra, ",")
	}
	return fmt.Sprintf(`{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":%q}]%s}`, path, tail)
}
