// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import "strings"

// ListFilter selects flows of a listing (SPEC-005 listFlows). A zero field matches everything.
type ListFilter struct {
	// NamePrefix keeps flows whose name starts with it, ignoring case.
	NamePrefix string
	// ActivationState keeps flows in exactly this state.
	ActivationState ActivationState
}

// Match reports whether the flow passes the filter.
func (f ListFilter) Match(r Record) bool {
	return strings.HasPrefix(strings.ToLower(r.Name), strings.ToLower(f.NamePrefix)) &&
		(f.ActivationState == "" || r.ActivationState == f.ActivationState)
}
