// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package issue declares the scope type the way internal/platform/orgscope/issue does.
package issue

// Org is the scope of one organization.
type Org struct{ id [16]byte }

// New returns a scope.
func New(id [16]byte) Org { return Org{id: id} }
