// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package connector holds the connector model of Orcon: the orcon/connector@1
// manifest and its validator, the reproducible package format, the installed
// connector store with organization grants, and the node-type catalog that
// validation level 5 asks about connector nodes (ADR-0029, ADR-0034, SPEC-003
// cut CN1).
//
// It is a business-area package. The manifest is JSON data, so the package
// reads it as a [jsonvalue.Value] tree; the published JSON Schema is embedded
// next to the validator (see [Schema]) and the Go validator is the one the
// product runs. Persistence lives behind the ports of store.go, implemented by
// the SQLite and PostgreSQL adapters.
//
// The trust, signing and catalog rules (ADR-0033) are not here: an installed
// package carries no seal yet, and the [Policy] port is where they plug in.
package connector
