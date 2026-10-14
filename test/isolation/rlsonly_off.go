// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !isolation_rls_only

package isolation

// RLSOnly is true when the battery runs with the filter of wall 1 switched off
// so that PostgreSQL row-level security (wall 3) stands alone. Build tag:
// isolation_rls_only.
const RLSOnly = false
