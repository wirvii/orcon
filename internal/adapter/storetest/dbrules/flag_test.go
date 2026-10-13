// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import "flag"

// The conformance command (E-STO-1) runs every package under storetest with
// "-args -store=<name>". This package selects no store, but its test binary
// must accept the flag instead of failing on it.
var _ = flag.String("store", "auto", "accepted and ignored: the store flag belongs to the conformance suite")
