// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package bpmn is the BPMN 2.0 XML codec of the orcon/flow@1 flow format
// (ADR-0007 sections 5 and 8). [Export] writes a flow as BPMN 2.0 XML and
// [Import] reads back a document that Export produced, so that for every valid
// flow f the canonical form of Import(Export(f)) equals the canonical form of
// f byte for byte.
//
// # Mapping
//
// Everything BPMN has a place for is written in BPMN:
//
//   - the flow becomes one process; its id, name and description become the
//     id (prefixed "flow_"), name and documentation of the process;
//   - a node becomes the BPMN element named by its element member, with the
//     node id (prefixed "node_") and name;
//   - an edge becomes a sequenceFlow whose condition is a conditionExpression
//     in the language urn:orcon:expr:1; the id is prefixed "edge_";
//   - an item-routing node carries orcon:itemRouting with its routing value
//     (EXCLUSIVE or INCLUSIVE); a loop control node carries
//     orcon:loopControl; a FIRST join on a complexGateway carries an
//     activation condition of 1; an edge flagged default is also the default
//     of its source when it is the only one;
//   - a BPMNDI diagram is derived from the graph so that BPMN viewers can
//     draw the flow.
//
// Everything BPMN has no place for is written to extensionElements in the
// namespace urn:orcon:bpmn:1, with the same UPPER_SNAKE values as the JSON
// document. The extension form is a typed tree: orcon:str, orcon:num,
// orcon:bool, orcon:null, orcon:obj and orcon:arr, each object member and each
// node or edge block named by the attribute n. Numbers keep their written
// text, so nothing is rounded. The editor member, the settings, the variables,
// the extension members and the parameters of every node travel this way, and
// are the authoritative source on import.
//
// The derived parts (the BPMNDI diagram, activation conditions, default
// attributes, the orcon:loopControl marker) exist for other BPMN tools and are
// ignored on import.
//
// # Scope of this cut
//
// Nodes and blocks available up to the in-memory token interpreter (cut C3)
// are exported: activities without waits or calls, start and end events,
// exclusive, inclusive, parallel and complex gateways, item routing, joins,
// loop control, retry, timeouts, error routing, concurrency, compensation and
// attention. Waits, boundary events, intermediate events, event-based
// gateways, call activities and multi-instance blocks are refused with an
// [*UnsupportedError] until the cut that implements them (C9) defines their
// native BPMN form.
//
// The importer in V1 accepts only documents that carry urn:orcon:bpmn:1.
// Arbitrary third-party BPMN is out of scope.
package bpmn
