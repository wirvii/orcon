// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// ReviewError lists the imported nodes that still wait for a person.
type ReviewError struct {
	Nodes []flow.NodeID
}

// Error names the nodes.
func (e *ReviewError) Error() string {
	ids := make([]string, len(e.Nodes))
	for i, id := range e.Nodes {
		ids[i] = string(id)
	}
	return fmt.Sprintf("%v: %s", ErrReviewRequired, strings.Join(ids, ", "))
}

// Unwrap lets errors.Is match [ErrReviewRequired].
func (e *ReviewError) Unwrap() error { return ErrReviewRequired }

// PendingReview returns the ids of the nodes of an imported flow that carry a
// review mark nobody has cleared.
func PendingReview(doc *flow.Document) []flow.NodeID {
	var out []flow.NodeID
	for i := range doc.Nodes {
		if reviewRequired(doc.Nodes[i].Ext) {
			out = append(out, doc.Nodes[i].ID)
		}
	}
	return out
}

// CheckActivation fails with a [*ReviewError] while any node of the flow is
// marked for review. A person clears the mark with [MarkReviewed] after
// reading the node (SPEC-003 Q11, R-IMP-03).
//
// The activation endpoint of the flow lifecycle (ADR-0024) must call it; until
// that cut exists nothing else enforces the mark.
func CheckActivation(doc *flow.Document) error {
	if pending := PendingReview(doc); len(pending) > 0 {
		return &ReviewError{Nodes: pending}
	}
	return nil
}

// MarkReviewed records that a person reviewed the node, clearing its mark.
// It reports false when the node does not exist or carries no mark.
func MarkReviewed(doc *flow.Document, id flow.NodeID) bool {
	for i := range doc.Nodes {
		n := &doc.Nodes[i]
		if n.ID != id || !reviewRequired(n.Ext) {
			continue
		}
		imp, _ := n.Ext.Get(ImportExtKey)
		review, _ := imp.Get("review")
		cleared := review.With("required", jsonvalue.Bool(false))
		n.Ext = n.Ext.With(ImportExtKey, imp.With("review", cleared))
		return true
	}
	return false
}

func reviewRequired(ext jsonvalue.Value) bool {
	imp, ok := ext.Get(ImportExtKey)
	if !ok {
		return false
	}
	review, ok := imp.Get("review")
	if !ok {
		return false
	}
	req, ok := review.Get("required")
	if !ok {
		return false
	}
	b, _ := req.Bool()
	return b
}
