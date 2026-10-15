// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package anthropic is the model adapter for the Anthropic Messages wire
// format (ADR-0061 §1, connector orcon.ai.anthropic).
//
// The adapter is an adapter package (architecture-layers.md AL-1.2): it maps
// the canonical types of internal/ai/provider to Messages requests and back,
// and it holds no business rules. The composition root injects an *http.Client
// built from the egress client, so every request is checked by the egress
// policy (ADR-0029 §8). Provider constraints, such as the rule that turns must
// alternate between user and assistant, are handled here so the canonical
// transcript never changes shape.
package anthropic
