<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0035 — n8n community node converter

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-003.

## Context
K2: no runtime compatibility layer for n8n community nodes in V1. An automatic converter turns n8n declarative-style community nodes into Orcon declarative connectors; programmatic nodes are flagged with a suggested official equivalent or the generic HTTP node; opt-in measurement of which community nodes appear in imports guides official ports. The converter is a public tool anyone can run; its output preserves the original package's licence and authorship. n8n's own source is under the Sustainable Use License and must never be copied; Orcon is Apache-2.0.

## Decision

### 1. Command
`orcon n8n convert-node <package.tgz | directory | npm-name@version> --out <dir> [--report json|text]` in the `orcon` binary. Network is used only to fetch from npm when a name is given.

### 2. Reading node descriptions
- Read `package.json` and its documented `n8n` section (lists of node and credential files).
- Load each compiled node/credential file inside the QuickJS-WASM sandbox (ADR-0018 runtime, no network or filesystem) with Orcon-authored stand-in modules for `n8n-workflow` and similar imports, providing only the constants and type names that description objects need. Stand-ins are written exclusively from n8n's public documentation and observed behaviour (clean room); no n8n package is a dependency and no n8n code is read or copied.
- A legal review of the stand-in modules and of the converter's use must be approved before the converter is first released (owner decision, SPEC-003 Q9).
- Instantiate the classes and read their `description` objects.

### 3. Classification
- Declarative: description defines request defaults/routing and the class has no execute, poll, trigger or webhook methods.
- Programmatic: anything else.
- Credentials: declarative if authentication is generic (header/query/body/basic) with an optional test request; custom pre-authentication → reported.

### 4. Conversion (declarative)
Properties → parameters (type table in the reference doc); display conditions → `displayWhen`; request routing → `request`; parameter routing to query/body/headers → request templates; built-in output post-processing (root property, set key value, filter, limit, sort, binary data) → `response` operations; generic pagination → ADR-0030 §6 when it matches a strategy; credential authentication → ADR-0030 §4 scheme. Expressions are rewritten to ADR-0018 syntax; references that cannot be mapped are listed. Every write operation receives `safety.level = UNSAFE` and `proofLevel = DECLARED` unless the description proves idempotency (it normally cannot); the report asks for review. Result status: `FULL`, `PARTIAL` (list of unconverted elements; package still produced, failing parts omitted and marked) or `UNCONVERTIBLE`.

### 5. Programmatic nodes
Report per node: name, operations, base URL if detectable, suggested official connector (matched by API host and name against the catalog index) or the generic HTTP node.

### 6. Output and attribution
Package id `community.<sanitised npm name>`; publisher = original author(s); `license` = original SPDX expression; original `LICENSE` copied; `NOTICE` lists origin; `ATTRIBUTION.json { originalPackage, version, authors, repository, license, converterVersion, convertedAt (from package metadata, not wall clock), note: "Converted by the Orcon converter. Not affiliated with or endorsed by n8n." }`. Without a licence allowing redistribution, the output is marked `redistributable: false` (local use only; the catalog rejects it). The output never uses the `orcon.*` namespace or n8n branding and is never signed (seal `UNREVIEWED`, ADR-0033 §3).

### 7. Reproducibility
Same input package and converter version → byte-identical output package, so catalog review can re-run the conversion and compare digests.

### 8. Opt-in usage measurement
Owner decision, SPEC-003 Q10. Disabled by default. When importing workflows, the user is asked explicitly; if they accept, only `{ packageName, packageVersion, nodeTypes[] }` is sent to a documented, configurable endpoint; no workflow content, identifiers or credentials. Aggregates feed the wave roadmap (ADR-0037 §5).

## Consequences
Community authors' declarative nodes become reviewable Orcon packages with their licence and credit intact; Orcon never redistributes n8n code.

## Alternatives considered
Runtime compatibility layer executing n8n nodes (rejected by K2 for V1); static parsing of TypeScript sources only (fallback if legal review rejects sandbox execution; fails on computed descriptions).
