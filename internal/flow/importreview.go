// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// ImportExtKey is the key of a node's ext member that records where an
// imported node came from (ADR-0034 section 8, amendment 9 of SPEC-003). It is
// the only key of the reserved orcon namespace that a flow may carry, and
// only in a node.
const ImportExtKey = "orcon.import"

// Review states of an imported node.
const (
	// ReviewNeeds marks an imported node with an UNSAFE contract that a person
	// has not reviewed.
	ReviewNeeds = "NEEDS_REVIEW"
	// ReviewDone marks the node as reviewed; the reasons stay.
	ReviewDone = "REVIEWED"
)

// Reasons an imported node needs review.
const (
	// ReasonHTTPWrite: an HTTP request with a literal write method.
	ReasonHTTPWrite = "HTTP_WRITE"
	// ReasonHTTPMethodExpression: an HTTP request whose method is an expression.
	ReasonHTTPMethodExpression = "HTTP_METHOD_EXPRESSION"
	// ReasonUnsafeOperation: a connector operation whose contract is UNSAFE.
	ReasonUnsafeOperation = "UNSAFE_OPERATION"
)

// typeUnsupported is the node type that keeps an n8n node Orcon cannot run.
const typeUnsupported = "orcon.unsupported"

// checkImportMark checks the closed shape of ext["orcon.import"] (level 1).
func checkImportMark(r *collector, path string, v jsonvalue.Value, n *Node) {
	if v.Kind() != jsonvalue.KindObject {
		r.errorf(CodeStructure, path, "must be an object, found %s", v.Kind())
		return
	}
	for _, m := range v.Members() {
		switch m.Key {
		case "n8nType", "n8nNodeId":
			if s, ok := m.Value.Str(); !ok || s == "" {
				r.errorf(CodeValue, path+"/"+m.Key, "must be a non-empty string")
			}
		case "n8nTypeVersion":
			if m.Value.Kind() != jsonvalue.KindNumber {
				r.errorf(CodeValue, path+"/"+m.Key, "must be a number")
			}
		case "reviewStatus":
			s, _ := m.Value.Str()
			if s != ReviewNeeds && s != ReviewDone {
				r.errorf(CodeEnum, path+"/"+m.Key, "%v is not one of %s, %s", m.Value, ReviewNeeds, ReviewDone)
			}
		case "reviewReasons":
			checkReviewReasons(r, path+"/"+m.Key, m.Value)
		case "original":
			if m.Value.Kind() != jsonvalue.KindObject {
				r.errorf(CodeValue, path+"/"+m.Key, "must be an object")
			} else if typeName(n.Type) != typeUnsupported {
				r.errorf(CodeBlock, path+"/"+m.Key, "the original n8n node is kept only by %s nodes", typeUnsupported)
			}
		default:
			r.errorf(CodeExt, path, "unknown member %q in %s", m.Key, ImportExtKey)
		}
	}
	for _, req := range []string{"n8nType", "n8nTypeVersion"} {
		if _, ok := v.Get(req); !ok {
			r.errorf(CodeStructure, path, "missing required member %q", req)
		}
	}
	_, hasStatus := v.Get("reviewStatus")
	_, hasReasons := v.Get("reviewReasons")
	if hasStatus != hasReasons {
		r.errorf(CodeValue, path, "reviewStatus and reviewReasons must appear together")
	}
}

func checkReviewReasons(r *collector, path string, v jsonvalue.Value) {
	if v.Kind() != jsonvalue.KindArray || v.Len() == 0 {
		r.errorf(CodeValue, path, "must be a non-empty array")
		return
	}
	var seen []string
	for i, item := range v.Items() {
		s, _ := item.Str()
		switch s {
		case ReasonHTTPWrite, ReasonHTTPMethodExpression, ReasonUnsafeOperation:
		default:
			r.errorf(CodeEnum, fmt.Sprintf("%s/%d", path, i), "%v is not a review reason", item)
		}
		if slices.Contains(seen, s) {
			r.errorf(CodeValue, fmt.Sprintf("%s/%d", path, i), "repeats the reason %q", s)
		}
		seen = append(seen, s)
	}
}

// reviewStatusOf returns the review status of a node, or "".
func reviewStatusOf(n *Node) string {
	imp, ok := n.Ext.Get(ImportExtKey)
	if !ok {
		return ""
	}
	st, ok := imp.Get("reviewStatus")
	if !ok {
		return ""
	}
	s, _ := st.Str()
	return s
}

// PendingImportReview returns the ids of the nodes marked NEEDS_REVIEW. It is
// the rule that activation applies (ADR-0024); publishing does not fail on it.
func PendingImportReview(doc *Document) []NodeID {
	var out []NodeID
	for i := range doc.Nodes {
		if reviewStatusOf(&doc.Nodes[i]) == ReviewNeeds {
			out = append(out, doc.Nodes[i].ID)
		}
	}
	return out
}

// ErrImportReviewRequired marks a flow with imported nodes nobody reviewed.
var ErrImportReviewRequired = errors.New("flow: imported nodes need review before activation")

// ImportReviewError names the nodes that still need review.
type ImportReviewError struct {
	Nodes []NodeID
}

// Error names the nodes.
func (e *ImportReviewError) Error() string {
	ids := make([]string, len(e.Nodes))
	for i, id := range e.Nodes {
		ids[i] = string(id)
	}
	return fmt.Sprintf("%v: %s", ErrImportReviewRequired, strings.Join(ids, ", "))
}

// Unwrap lets errors.Is match [ErrImportReviewRequired].
func (e *ImportReviewError) Unwrap() error { return ErrImportReviewRequired }

// CheckImportReview fails with an [*ImportReviewError] while any node is
// marked NEEDS_REVIEW. The activation operation calls it.
func CheckImportReview(doc *Document) error {
	if pending := PendingImportReview(doc); len(pending) > 0 {
		return &ImportReviewError{Nodes: pending}
	}
	return nil
}

// MarkImportReviewed sets the review status of the node to REVIEWED and keeps
// the reasons. It reports false when the node does not exist or is not marked
// NEEDS_REVIEW. Editing the draft this way needs the edit permission and a new
// publication, because ext is part of the content hash.
func MarkImportReviewed(doc *Document, id NodeID) bool {
	for i := range doc.Nodes {
		n := &doc.Nodes[i]
		if n.ID != id || reviewStatusOf(n) != ReviewNeeds {
			continue
		}
		imp, _ := n.Ext.Get(ImportExtKey)
		n.Ext = n.Ext.With(ImportExtKey, imp.With("reviewStatus", jsonvalue.String(ReviewDone)))
		return true
	}
	return false
}
