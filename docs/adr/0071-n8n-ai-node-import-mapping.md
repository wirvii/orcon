<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0071 — n8n AI/LangChain node import mapping

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: UPPER_SNAKE values per ADR-0080 §3)

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
Owner rule: V1 covers at least every n8n capability including its AI node set. ADR-0037 defines coverage measurement; ADR-0035 forbids using n8n code. The inventory comes from public documentation only (SPEC-006 Annex B).

## Decision
1. Every n8n AI node type (root and sub-nodes) maps to an Orcon node type, an option of an Orcon node, a connector kind, or is reported `UNSUPPORTED` with a suggestion (SPEC-006 Annex B). Cluster sub-nodes attached to a root node become blocks of the Orcon node (`ai.model`, `ai.memory`, `ai.tools[]`, `ai.outputParser`, retrieval config), never separate executable nodes.
2. Classification per node: `SUPPORTED`, `ADAPTED` (behaviour difference reported, e.g. Simple Memory becomes persisted memory; streaming chunk granularity), `UNSUPPORTED` (e.g. LangChain Code node, which runs arbitrary LangChain JavaScript).
3. Credentials map by type and name only (ADR-0051); models referenced by name are validated against the provider's capability table; unknown models import with a warning.
4. `$fromAI()` expressions are preserved; tool descriptions preserved; agent "max iterations" → `maxSteps`; system message → instructions; "return intermediate steps" → output option; human review on tools → `approval: ALWAYS` with channel mapping (n8n Chat → `CHAT_USER`; Slack/Teams/Telegram/Gmail/WhatsApp/Google Chat/Outlook/Discord → `LINK` via a generated notification flow). When the imported flow has a public trigger, sensitive tools mapped to `CHAT_USER` additionally require `ORCON` approval (ADR-0068 §5 floor), reported as a behaviour difference. Option values of n8n nodes are translated to the UPPER_SNAKE spelling of `orcon/flow@1` (for example a recursive character splitter becomes `RECURSIVE_CHARACTER`); the n8n chat protocol values are not part of the flow document and are kept.
5. Imported flows stay drafts and inactive (D7b); redaction defaults ON for imported agents (behaviour difference reported).
6. The coverage harness (ADR-0037) reports AI node coverage separately; the gate requires zero n8n AI node types without a mapping entry for the pinned n8n version.

## Consequences
n8n AI workflows import with explicit differences; nothing is silently substituted.

## Alternatives considered
Embedding a LangChain runtime (rejected: second runtime, licence and isolation); keeping n8n's value spelling inside Orcon flow documents (rejected by ADR-0080 §3: one spelling for every public value).
