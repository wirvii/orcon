// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"regexp"
	"slices"
	"strings"
)

// Patterns of the connector pin, shared with the published JSON Schema (the
// flowjson tests compare the two). They use only constructs that RE2 and
// ECMAScript read the same way.
const (
	// ConnectorVersionPattern is a SemVer 2.0 version (semver.org, section 2).
	ConnectorVersionPattern = `^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-((0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`
	// ConnectorDigestPattern is the digest of a connector package: "sha256:"
	// and 64 lowercase hexadecimal digits (ADR-0029 section 2).
	ConnectorDigestPattern = `^sha256:[0-9a-f]{64}$`
)

var (
	connectorVersionPattern = regexp.MustCompile(ConnectorVersionPattern)
	connectorDigestPattern  = regexp.MustCompile(ConnectorDigestPattern)
)

// ConnectorNodeTypes is implemented by a node-type catalog that knows
// installed connector packages. Validation level 5 prefers it to
// [NodeTypes.Lookup] for connector nodes, because only it can answer for the
// package version a node pins and for the operation it selects.
type ConnectorNodeTypes interface {
	NodeTypes
	// ResolveNode describes a node type as the pinned package version defines
	// it. pin is the connector pin of the node, nil when it has none;
	// operation is the operation the node selects with a literal "operation"
	// parameter, "" when it names none.
	ResolveNode(ctx context.Context, nodeType string, pin *ConnectorPin, operation string) NodeResolution
}

// NodeResolution is the answer of [ConnectorNodeTypes.ResolveNode].
type NodeResolution struct {
	// Type is what validation needs to know about the node type; meaningful
	// when Known and UnknownOperation is false.
	Type NodeType
	// Known is false when no installed package defines the node type.
	Known bool
	// UnknownOperation is true when the package has the type but not the
	// selected operation.
	UnknownOperation bool
	// Problems has one message per reason the pin cannot be used: the version
	// is not installed, the digest differs, the organization has not granted
	// the package, the policy blocks it, or the node has no pin although its
	// type belongs to a package.
	Problems []string
}

// ConnectorReference is one connector version a flow document uses, as the
// storage layer records it. The references of a draft and of every published
// version are rows, so retiring a connector version never has to read inside
// a flow document (ADR-0034 section 6).
type ConnectorReference struct {
	// ID is the row identity, created by the caller as a UUIDv7.
	ID ConnectorReferenceID
	// ConnectorID is "<publisher>.<name>", the identity of the package.
	ConnectorID string
	// ConnectorVersion is the pinned SemVer version.
	ConnectorVersion string
}

// ConnectorIDOf returns the connector package a node type belongs to: the
// first two dot-separated segments of "<publisher>.<name>.<node>@<major>"
// (ADR-0029 section 3). It reports false for a type with fewer than three
// segments, such as "orcon.http@1", which no package owns.
func ConnectorIDOf(nodeType string) (string, bool) {
	segments := strings.Split(typeName(nodeType), ".")
	if len(segments) < 3 {
		return "", false
	}
	return segments[0] + "." + segments[1], true
}

// ConnectorUse is a connector version a document pins, without a row identity.
type ConnectorUse struct {
	ConnectorID      string
	ConnectorVersion string
}

// ConnectorUses lists the distinct connector versions the nodes of a document
// pin, ordered by connector id and then version. Nodes without a pin, or with
// a type no package can own, contribute nothing.
func ConnectorUses(d *Document) []ConnectorUse {
	var uses []ConnectorUse
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if n.Connector == nil {
			continue
		}
		id, ok := ConnectorIDOf(n.Type)
		if !ok {
			continue
		}
		use := ConnectorUse{ConnectorID: id, ConnectorVersion: n.Connector.Version}
		if !slices.Contains(uses, use) {
			uses = append(uses, use)
		}
	}
	slices.SortFunc(uses, func(a, b ConnectorUse) int {
		if c := strings.Compare(a.ConnectorID, b.ConnectorID); c != 0 {
			return c
		}
		return strings.Compare(a.ConnectorVersion, b.ConnectorVersion)
	})
	return uses
}

// connectorPin runs the shape rules of a pin (validation level 1).
func (v *validator) connectorPin(base string, n *Node) {
	if n.Connector == nil {
		return
	}
	path := base + "/connector"
	checkPattern(v.r, path+"/version", "connector version", n.Connector.Version, connectorVersionPattern)
	checkPattern(v.r, path+"/digest", "connector digest", n.Connector.Digest, connectorDigestPattern)
	if isFormatType(n.Type) {
		v.r.errorf(CodeConnector, path, "node %q has type %q, which the format defines; only connector nodes carry a connector pin", n.ID, n.Type)
		return
	}
	if _, ok := ConnectorIDOf(n.Type); !ok {
		v.r.errorf(CodeConnector, path, "node %q has type %q, which no connector package can own; a connector node type reads <publisher>.<name>.<node>@<major>", n.ID, n.Type)
	}
}

// operationOf returns the operation a node selects with its "operation"
// parameter, or "" when it does not name one literally.
func operationOf(n *Node) string {
	if n.Parameters.IsZero() {
		return ""
	}
	p, ok := n.Parameters.Get("operation")
	if !ok {
		return ""
	}
	s, ok := p.Str()
	if !ok || strings.HasPrefix(s, "=") {
		return ""
	}
	return s
}
