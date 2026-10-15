// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package declarative is the interpreter of the orcon/declarative@1 format
// (ADR-0030): a connector described as data, whose operations are sent over
// HTTP by the host, never by code the connector ships.
//
// The package owns the format types ([Definition] and friends), the validator
// that rejects a definition the interpreter cannot run safely ([Validate]),
// the authentication schemes applied by the host ([Auth]), the response
// mapping and the ambiguity classification of ADR-0079 §4–§5 ([Execute]). It
// is an area package: it imports neither net/http nor encoding/json. The
// network is a port ([Sender]) implemented by the egress adapter, and the JSON
// document is decoded by the codec adapter internal/adapter/codec/declarativejson.
//
// Scope of this cut (CN3-decl, SPEC-003 Enmienda 7): the HTTP description, the
// eight authentication schemes except OAUTH2 and SESSION_TOKEN, the five
// pagination strategies, the error mapping with the closed list of fourteen
// codes, the safety rules of writes and bounded retries. Declarative triggers,
// rate-limit buckets and the dispatch wiring of nodes arrive with the later
// CN3 cuts; OAUTH2 and SESSION_TOKEN are reported by [Validate] as not yet
// supported rather than silently ignored.
package declarative
