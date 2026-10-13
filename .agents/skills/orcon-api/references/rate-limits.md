<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Rate limits

Status: Accepted

This document defines the rate class of every operation and the rate-limit headers. It refines the rate-limit convention of ADR-0053 §6. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`RATE-…`) are stable.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md), then this rulebook, then reviewer preference. The check that enforces each rule is named in brackets (an `orcon/<rule>` lint rule, a make target, or `review`).

## Rules

- **RATE-1** Every operation declares `x-orcon-rate-class`. [`rate-class`]
- **RATE-2** `GET` → `read`; `text/event-stream` and the MCP endpoint → `stream`; operations that start executions, test runs, retries, repeats, redeliveries or replays → `run`; other mutations → `write`. [`rate-class`]
- **RATE-3** Responses carry `RateLimit` and `RateLimit-Policy`; `429 rate-limited` carries `Retry-After`. [`rate-class`]
- **RATE-4** Defaults and floors are configuration (ADR-0053 §6), never per-operation numbers in the contract. [review]
