// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package flow holds the flow definition of Orcon: the orcon/flow@1 document
// model, its static validation, the canonical content hash, the upgrade chain
// for older formats and the draft, publication and snapshot rules of a flow
// (ADR-0007, ADR-0008 section 4, ADR-0010 section 11).
//
// It is a business-area package. The document is JSON data, so the package
// reads and writes it as a [jsonvalue.Value] tree; the bytes on the wire and
// the published JSON Schema belong to the codec in
// internal/adapter/codec/flowjson.
//
// Nothing here is safe for concurrent mutation: a [Flow] is owned by one
// caller at a time (the storage layer serializes writers), while [Document]
// values are plain data and [Version] and [Snapshot] values are immutable.
package flow
