<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0064 — Conversation memory: persisted, ordered, retrievable, retained

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: table names per ADR-0077; UPPER_SNAKE values per ADR-0080 §3)

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
A3: conversation memory persisted in Orcon's database (survives restarts, multi-server, D17 retention and masking), fixing n8n's in-process "Simple Memory"; recency decay and importance weighting reused from mneme (`internal/scoring/decay.go`, `importance.go`, `relevance.go`).

## Decision

### 1. Model
`ai_conversation(org_id, id, project_id, memory_scope, session_key_hash, user_reference?, created_at, last_active_at, retained_until_at, summary_reference, token_count, data_key_wrapped, status)`; `ai_message(org_id, id, ai_conversation_id, sequence_number, role, provenance, content_ciphertext, token_count, importance, is_pinned, execution_id, ai_agent_run_id, created_at)`; FTS and vector indexes on message text as in ADR-0065 (same code). Key `(org_id, memoryScope, sessionKey)`; `memoryScope` = `FLOW` (default) | `PROJECT` | `NAMED`.

### 2. Strategies
`WINDOW` (last N messages or T tokens; default 20 messages), `SUMMARY` (rolling summary when the window overflows; recorded, costed call), `RETRIEVAL` (window + hybrid retrieval of older messages, §5), `NONE`.

### 3. Determinism
Reads record the watermark `(conversationId, maxSeq)` and retrieved ids as nondeterminism; recomputation reads the same window. Writes are internal effects (ADR-0008 §4.4) in the step's confirmation point keyed `H(conversationId, executionId, runId, stepNo, idx)`.

### 4. Ordering
Chat and channel triggers set `concurrencyKey = conversationId`; the engine admits one RUNNING execution per key in FIFO order (ADR-0014 amendment). `PARALLEL` opt-out for memory-less flows.

### 5. Retrieval scoring
For candidate messages outside the window: `score = RRF(lexical, vector) × (0.5 + 0.5 × importance) × exp(−λ × ageDays)`, with `λ` per agent (default 0.05/day), importance defaults pinned/user-fact 0.8, user 0.6, assistant 0.5, tool result 0.3. Recency uses message time (no write-on-read). Unlike mneme's `service/search.go`, the BM25 magnitude is not added to the RRF score; signals are multiplicative on RRF.

### 6. Retention, encryption, access
Retention per agent/flow, default 90 days after last activity, configurable per agent within an instance-wide floor and ceiling. Content encrypted with a per-conversation DEK wrapped by the organization key (ADR-0042); deletion or expiry destroys the DEK and purges in batches. Sensitive values are stored in placeholder form with their values in the conversation-scoped redaction vault (ADR-0068). API access to content requires `conversation:read-data` and is audited (coalesced 15 min).

### 7. External memories and manager node
`kind: MEMORY` connectors (Postgres, Redis, MongoDB, Motorhead, Xata, Zep) implement `Load(window) / Append / Clear`; `orcon.ai.memoryManager@1` gets, inserts, pins, deletes and clears messages.

### 8. Logical schema additions
`ai_conversation`, `ai_message`, `ai_message_fts5` (SQLite FTS5 external-content table) / `tsv` column (PostgreSQL), `ai_message_vector`.

## Consequences
Memory is durable, ordered and isolated; retrieval memory reuses the knowledge retrieval code.

## Alternatives considered
In-process memory (rejected by A3); unordered concurrent writes (rejected: interleaved context); access-time recency like mneme (rejected: write on every read).
