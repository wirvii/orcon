// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package redact is a fixture stand-in for internal/platform/redact.
package redact

// Secret is a string whose printed forms are redacted.
type Secret string

// String implements fmt.Stringer.
func (Secret) String() string { return "[REDACTED]" }
