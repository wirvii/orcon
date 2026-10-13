// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package flowjson is the JSON codec of the orcon/flow@1 flow format
// (ADR-0007): it turns bytes into a [flow.Document] and back, and publishes
// the normative JSON Schema of the format.
//
// It is an adapter package. It holds no business rules: reading upgrades
// older formats and checks the shape of the document, while the rules of a
// valid flow live in [flow.Validate]. Decode therefore returns documents that
// may still be invalid; callers validate before publishing.
package flowjson
