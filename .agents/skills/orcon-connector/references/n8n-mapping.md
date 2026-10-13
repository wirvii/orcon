<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# n8n mapping

Status: Accepted

This file covers how a connector declares the n8n nodes and credentials it replaces, so the importer can convert flows one to one (ADR-0034 §8, ADR-0037 §6). Rule IDs (`N8-…`) are stable. Precedence is in `kinds.md`.

## Rules

- **N8-1** `n8n.types[]` lists every `(n8n type, typeVersion)` the connector replaces (ADR-0034 §8, ADR-0037 §6).
- **N8-2** `parameterMap` covers every n8n parameter of each mapped typeVersion; unmapped ones are reported by the importer, never approximated.
- **N8-3** n8n `getAll` maps to `getMany`; other n8n verbs map to the vocabulary verb with the same meaning; the mapping lives in `parameterMap`. Enumerated n8n values that land in Orcon manifest fields are translated to their UPPER_SNAKE form (NM-13).
- **N8-4** Credential types map through `credentials[].n8nTypes`; secrets are never imported.
- **N8-5** Write operations imported without an inferable contract are `UNSAFE` and flagged for review (ADR-0029 §5).
- **N8-6** Use n8n public documentation and observed behaviour only; never n8n source, text or images.
