// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package openai is the model adapter for the Chat Completions wire format
// (ADR-0061 §1). One adapter serves orcon.ai.openai and
// orcon.ai.openai-compatible, and Ollama through its OpenAI-compatible
// endpoint: they differ only in base URL and credential.
//
// The adapter is an adapter package (architecture-layers.md AL-1.2). It turns
// the canonical types of internal/ai/provider into the wire format and back,
// and it never decides business rules. It does not open connections itself:
// the composition root injects an *http.Client built from the egress client,
// so every request is checked by the egress policy (ADR-0029 §8).
package openai
