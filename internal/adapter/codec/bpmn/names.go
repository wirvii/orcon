// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

// Namespaces of the exported document.
const (
	// NamespaceOrcon is the namespace of the Orcon extension elements. A
	// document must declare it to be imported.
	NamespaceOrcon = "urn:orcon:bpmn:1"
	// ExpressionLanguage is the language of every conditionExpression.
	ExpressionLanguage = "urn:orcon:expr:1"

	nsModel  = "http://www.omg.org/spec/BPMN/20100524/MODEL"
	nsBPMNDI = "http://www.omg.org/spec/BPMN/20100524/DI"
	nsDC     = "http://www.omg.org/spec/DD/20100524/DC"
	nsDI     = "http://www.omg.org/spec/DD/20100524/DI"
	nsXSI    = "http://www.w3.org/2001/XMLSchema-instance"
)

// Id prefixes. XML ids must be NCNames and unique in the document, while flow
// ids are UUIDs and node and edge ids may start with a digit or a hyphen.
const (
	idFlow = "flow_"
	idNode = "node_"
	idEdge = "edge_"
)

// Local names of the typed value elements of the extension namespace.
const (
	kindStr  = "str"
	kindNum  = "num"
	kindBool = "bool"
	kindNull = "null"
	kindObj  = "obj"
	kindArr  = "arr"
)

// Member names of the flow document that have a native BPMN place.
const (
	memberID          = "id"
	memberName        = "name"
	memberDescription = "description"
	memberElement     = "element"
	memberRouting     = "routing"
	memberCondition   = "condition"
	memberFrom        = "from"
	memberTo          = "to"
	memberNode        = "node"
	memberPort        = "port"
	memberFormat      = "format"
	memberNodes       = "nodes"
	memberEdges       = "edges"
	memberLoop        = "loop"
	memberJoin        = "join"
	memberDefault     = "default"
)
