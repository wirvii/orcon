// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package jsonvalue is the JSON data model of Orcon: an immutable tree of
// values that keeps numbers as decimal text, a strict parser, a compact
// encoder and the JSON Canonicalization Scheme (RFC 8785).
//
// It is a platform package: it imports no area or adapter. It is the only
// package outside the adapters that may import encoding/json (architecture
// layers AL-4.3), so areas read and write JSON through it.
package jsonvalue
