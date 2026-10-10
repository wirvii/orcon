# Orcon — Agent Implementation Contract

Status: **Accepted** | Version: 0.1

## Role separation

Product/architecture work authors requirements, ADRs, design specifications and brand assets. Implementation agents author code, tests and operational artifacts. Agents must not treat speculative documents as approved implementation instructions.

## Source-of-truth precedence

1. Accepted ADRs (specific architectural decisions).
2. Accepted product requirements and invariants.
3. Accepted API/schema/design-system specifications.
4. Proposed documents as research input only.
5. Existing code, which may be an incomplete scaffold.

If two accepted sources conflict, stop and request an explicit decision. Do not silently pick a preferred interpretation.

## Execution protocol

1. Identify requirement IDs, impacted contracts, risks and dependencies.
2. Propose a minimal implementation plan with testable acceptance criteria.
3. Implement only approved scope; preserve module boundaries and one-binary packaging.
4. Add unit, integration, crash-recovery, security and compatibility tests as appropriate.
5. Run the applicable tests and report commands, results, known limitations and unresolved risks.
6. Request an ADR when changing a foundational choice.

## Non-negotiable constraints

Apache-2.0 distribution; independent implementation; Go backend; embedded React editor; multi-role binary; SQLite local and PostgreSQL distributed as target architectures; no mandatory Temporal/Redis; no fake exactly-once guarantee; no undocumented enterprise-only features; no untrusted code in the host process without approved isolation.

## Quality gates

No claims of production readiness without successful CI, failure tests for state transitions, migration diagnostics, and documented security boundaries. Avoid expensive recursive review loops: bounded reviews proportional to risk, explicit pass/fail evidence, and escalation for unresolved decisions.

## Definition of done

Behavior matches acceptance tests; public interfaces documented; failure paths covered; security implications evaluated; examples updated; build and tests reported; no silent breaking changes.
