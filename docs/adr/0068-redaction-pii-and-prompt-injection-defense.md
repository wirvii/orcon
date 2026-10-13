<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0068 — Data redaction with placeholders, PII detection, untrusted content and approval policy

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: vault table per ADR-0077; UPPER_SNAKE values per ADR-0080 §3)

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
A5: per-agent redaction ON by default with stable placeholders (e.g. `[CEDULA_1]`) for fields marked sensitive (connector- or user-declared, the D17 marks); placeholders re-hydrated only inside Orcon when executing tool calls; optional PII detector OFF by default; all external content tagged untrusted; a sensitive tool invoked after untrusted content requires human approval (configurable); local models may disable redaction.

## Decision

### 1. Sensitive value set
Per execution (and per conversation): values of connector `sensitiveFields`, user-marked paths, the credential masking set (SPEC-004 R-CRED-04), and PII detections when enabled.

### 2. Two-layer redaction
(a) Template provenance: the expression engine (ADR-0018) in "render for model" mode returns the placeholder when it resolves a sensitive path. (b) Final sweep: an Aho–Corasick automaton over all known sensitive values rewrites the complete request and every tool result before it reaches the model. Placeholders are `[KIND_n]`, `n` stable per value within the scope.

### 3. Vault and re-hydration
`ai_redaction_entry(org_id, id, scope_kind: execution|conversation, execution_id?, ai_conversation_id?, placeholder, value_ciphertext, kind)`, with a check that exactly the column named by `scope_kind` is set, encrypted under the execution DEK (crypto-shredded at termination, ADR-0017 §3) or the conversation DEK (ADR-0064 §6). Before dispatching a tool call, the worker substitutes placeholders in arguments with real values; unknown placeholders stay literal; recorded steps keep placeholder form.

### 4. Reveal rule
Text shown to the end user re-hydrates only placeholders whose values that user supplied in the same conversation or that come from fields marked `revealToUser`; third-party data the agent retrieved stays in placeholder form. Node outputs keep placeholders unless `rehydrateOutput: true`.

### 5. Provenance and taint
Every content part carries `SYSTEM | USER | EXTERNAL`. `EXTERNAL` covers tool results from external systems, documents, web pages, emails, knowledge chunks, MCP outputs and outputs of tainted sub-agents. `USER` is untrusted for **public input**: any message whose sender is not a person authenticated as an Orcon user (chat with `NONE` or `BASIC` authentication, chat with a host-site `SIGNED_SESSION_TOKEN`, messaging channels, MCP server trigger with none/bearer/header authentication, public forms and webhooks); such a run is tainted from its first step, and the public-input mark propagates to flow tools and sub-agents it calls. `USER` is trusted by default for persons authenticated as Orcon users and for MCP server trigger calls made with Orcon API keys (ADR-0056). A run is tainted while untrusted content is in its context. With `requireApprovalAfterUntrusted` (default on), every sensitive tool call in a tainted run requires approval (ADR-0066 §5); per-tool exemption is explicit and audited.
**Public-input floor (normative).** A flow may be stricter than the default (e.g. `userInputTrust: UNTRUSTED` also distrusts authenticated Orcon users) but never more permissive for public input. In a run with public input: every sensitive tool call requires human approval; `approval: NEVER`, `exemptFromTaintPolicy` and disabling `requireApprovalAfterUntrusted` do not apply; the `CHAT_USER` channel is not sufficient (the approver would be the untrusted sender) — approval comes through `ORCON` or `LINK`; no setting marks public input as trusted. Non-sensitive tools run without approval. Publication rejects configurations that try to relax the floor on flows with public triggers, and the engine enforces it at runtime regardless of stored configuration. Untrusted content is wrapped in randomized delimiters with a do-not-follow notice (defence in depth only).

### 6. PII detector (off by default)
Deterministic Go detectors: email, phone (E.164 and national formats), payment cards (Luhn), IBAN (mod 97), IP addresses, secrets (known key formats incl. `orcon_*`, high entropy), national identifiers with checksum validation where defined: CO cédula and NIT, US SSN, MX CURP/RFC, BR CPF/CNPJ, ES DNI/NIE, AR DNI/CUIT, CL RUT, PE DNI/RUC; custom regex; optional model-based detector via connector.

### 7. Local models
`redaction: OFF` is allowed per agent when every model and fallback is local (Ollama or OpenAI-compatible on an approved private host); audited.

### 8. Guardrails node
`orcon.ai.guardrails@1`: deterministic checks (PII, secrets, URL allowlist, keywords, length) and model-based checks (jailbreak, topical alignment, NSFW, rubric) in `CHECK` (pass/fail ports) or `SANITIZE` mode.

### 9. Containment outside the model
Egress domain lists, credential host binding, project permissions, spend and step caps and approvals bound the damage of any successful injection; none can be changed from a prompt.

## Consequences
Providers never receive marked sensitive values; injection cannot trigger sensitive effects in tainted runs without a person. Accepted cost: some answer-quality loss, configurable per agent.

## Alternatives considered
Reversible encryption tokens instead of placeholders (rejected: meaningless to the model, leaks length/structure); redaction by the model itself (rejected: the provider sees the data); trusting delimiters alone (rejected: principle 8).
