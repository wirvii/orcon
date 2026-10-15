// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package orgscope names the type every organization-scoped store method takes
// (ADR-0046 section 2, orcon-db TI-2): [Org], the proof that the caller acts for
// one organization.
//
// The package exports no constructor and no function that returns an [Org]. A
// scope comes from a resolved authorization context or from the rows an engine
// function returns, and both obtain it from the sub-package issue, which only
// those callers (and tests) may import. The method set here is read-only, so a
// store can bind the scope to its org_id parameter but nobody can widen it.
package orgscope

import "github.com/wirvii/orcon/internal/platform/orgscope/issue"

// Org is the scope of one organization; see the package documentation.
type Org = issue.Org
