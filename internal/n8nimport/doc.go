// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package n8nimport holds what Orcon knows about n8n flows: which n8n node
// types it maps and how, how well a catalog covers a corpus of public n8n
// templates, and which connectors a first wave should build to reach the
// release target (ADR-0037, SPEC-003 cut CN2).
//
// # Coverage harness
//
// [LoadCorpus] reads a corpus directory written by tools/n8ncorpus. [Measure]
// classifies every template against a [Catalog] and returns the metrics of
// ADR-0037 section 3. A template is fully importable when every node maps
// (core table or catalog), every operation the node selects is in the
// catalog, no selector is an expression, and every expression passes the
// analyzer of ADR-0018. [SelectWave] picks, greedily and deterministically,
// the connectors that raise that percentage the most per unit of effort.
//
// The corpus is community content of unstated license. It is read from a
// directory outside version control; a [Report] holds only counts, hashes and
// scores, never a node name, a parameter or an expression.
//
// # Core nodes
//
// [CoreNodes] is the table of n8n core logic and utility nodes (SPEC-003
// Annex B) with the Orcon type and the cut that delivers each. It is data
// that covers the type level only: translating the parameters of a core node
// belongs to the cut that implements it.
//
// # Node mapping
//
// [MapNode] maps one n8n node. It never substitutes silently (D7b): a type or
// typeVersion with no declared mapping, an operation pair outside the
// catalog's parameter map, or an expression where a literal is needed is
// reported with the node and the field, and the node is not mapped. The n8n
// resource and operation pair becomes the single reserved parameter
// operation, resource.verb (SPEC-003 amendment 8). HTTP Request nodes that
// write are marked for review, and [flow.CheckImportReview] refuses a flow whose
// marked nodes were not reviewed.
//
// The package is an area: it reads files through io/fs and JSON through
// internal/platform/jsonvalue, and holds no adapter.
package n8nimport
