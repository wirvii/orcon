// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

// ID identifies a flow. It is stable across versions of the flow.
type ID string

// NodeID identifies a node inside one flow document. It is stable across the
// versions of the flow.
type NodeID string

// EdgeID identifies an edge inside one flow document.
type EdgeID string

// FormatMajor is the newest orcon/flow major version this build understands.
const FormatMajor = 1

// FormatTag is the value of the top-level "format" field of the current
// major version.
const FormatTag = "orcon/flow@1"

// formatPrefix precedes the major version number in a format tag.
const formatPrefix = "orcon/flow@"
