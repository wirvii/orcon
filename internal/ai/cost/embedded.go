// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cost

import "time"

// EmbeddedVersion names the default price table compiled into the binary.
const EmbeddedVersion = "embedded-2026-10-14"

// Embedded returns the default price table. It ships with no prices on
// purpose: a price is only used once it is confirmed against the provider's
// published price list, and the confirmed table arrives as a signed catalog
// artifact or an override. Until then every model call has an unknown cost,
// which a budget with unknownPrice BLOCK refuses.
func Embedded() Table {
	t, err := NewTable(EmbeddedVersion, time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		panic(err)
	}
	return t
}
