// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package provider is the canonical model-provider port of Orcon (ADR-0061).
// It is an area package (architecture-layers.md AL-1.1): it holds the request
// and response shapes every model adapter translates to and from, the error
// classes the engine uses to decide between a retry and a permanent failure,
// the tool-name rules, and the argument validation that runs before a tool is
// dispatched (ADR-0061 §3).
//
// Provider wire formats never appear here. Each adapter under
// internal/adapter/llm translates its own protocol to these types, and the
// canonical transcript never changes shape when the provider does.
//
// This package imports no encoding format: item data uses
// internal/platform/jsonvalue (AL-4.3).
package provider
