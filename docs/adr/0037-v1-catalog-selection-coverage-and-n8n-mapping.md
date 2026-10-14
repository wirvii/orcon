<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0037 — V1 catalog selection, published coverage, n8n node mapping and host-access nodes

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12; 2026-10-13 (editor test runs, ADR-0082)

Accepted by the owner on 2026-10-12 via SPEC-003.

## Context
K5: V1 covers 100 % of n8n core logic/utility nodes; integration connectors are chosen by measured usage in n8n's public template library with the target that at least 90 % of those templates import completely; fallback is the generic HTTP node plus the K2 converter; a public roadmap of waves continues until 100 % of n8n built-in connectors, with coverage percentage published. Corpus licensing is still open (SPEC-001 P8). Some core nodes act on the worker's host (commands, files), which conflicts with the default of no host access unless explicitly controlled.

## Decision

### 1. Corpus
The public n8n template library at a pinned snapshot date, associated with the pinned n8n version (SPEC-001 P8). Fetched at measurement time; only template id hashes, node-type counts and scores are committed until the fixture-licence review concludes.

### 2. Units and definitions
- Usage unit: `(nodeType, typeVersion, resource, operation)` plus credential types, triggers, expressions, Code node languages and libraries.
- A template is **fully importable** when the importer classifies every construct as exactly supported or supported with adaptation, with no unsupported or manual-validation-blocking item: every node and operation mapped (ADR-0034 §8), every expression accepted by the ADR-0018 analyzer, every trigger mapped (SPEC-002), Code languages and libraries available. Activation still requires review (D7b).

### 3. Measurement tool
`orcon n8n coverage --corpus <dir> --catalog <installed|planned.json>` outputs per-template status and aggregate metrics: unweighted template %, view-weighted template %, % with and without AI/LangChain nodes (AI coverage depends on the AI design), % of n8n built-in node types, % of used operations. Machine-readable JSON plus `docs/reference/connector-coverage.md`.

### 4. Selection and release gate
- Core logic/utility nodes: 100 % mapped (SPEC-003 Annex B; the tool verifies the list against the pinned version; any new core type blocks the gate until mapped).
- Integrations: greedy selection — repeatedly add the connector (with the operation subset needed) that maximises the marginal number of fully importable templates per estimated effort (declarative 1, native 3, new trigger family 3), ties broken by node-type popularity, until ≥ 90 %.
- Release gate: **unweighted** fully-importable ≥ 90 % (every template counts equally; owner decision, SPEC-003 Q15), measured with the release's catalog. View-weighted and with/without-AI figures are published for information only.

### 5. Waves and roadmap
Wave 1 = the selection above. Later waves are ordered by marginal coverage, then by opt-in community usage data (ADR-0035 §8), then by the remaining n8n built-in list, until 100 % of n8n built-in connectors. Each wave is its own specification. `docs/reference/connector-roadmap.md` lists waves and the coverage after each; regenerated per release. The n8n Data Table node is out of this ADR: tabular storage gets its own backlog item (SPEC-003 Q14).

### 6. Mapping rules
Every `(n8n type, typeVersion)` maps to an Orcon node type with a declared parameter map; unknown parameters are reported with node and field; nothing is substituted silently. Credentials map by type through `credentials[].n8nTypes`; secrets are never imported. HTTP Request write methods without an inferable safety contract are imported with `UNSAFE` and flagged for review; the flow cannot be activated until each flagged node has been reviewed (SPEC-003 Q11). Community nodes resolve to installed converted packages when present; otherwise they are reported with a suggestion.

### 7. Host-access nodes
Owner decision, SPEC-003 Q5. Execute Command, Read/Write Files from Disk, local SSH and local Git are implemented as official `orcon.host.*@1` nodes and are **disabled by default** in every installation.
- Only the instance admin enables them, declaring: an allowlist of commands (exact executable paths) for Execute Command; an allowlist of path prefixes (checked after resolving symbolic links) for file and local Git access; and a host label restricting which workers may run them (same mechanism as SPEC-002 Q10). The configuration is stored in the instance table `host_node_policy` (ADR-0029 §8; allowlists as child rows or a document, never arrays, per ADR-0077) and changes are audited.
- While disabled, publishing a flow that uses them fails with a message naming the setting to enable; any command or path outside the allowlist fails the item with a permanent error naming it.
- Execute Command runs as a child process with a deadline, output caps, an environment limited to admin-declared variables, and arguments passed as a list without a shell; the command allowlist matches the exact executable path (owner decision SPEC-003 N1). An admin may add a shell (`/bin/sh`, `bash`, …) to the allowlist only after an explicit warning in settings and in the editor that this allows any command on those workers; the change is confirmed and audited (`host_node_policy.changed` with `shellAllowed`, SPEC-004 Annex B). A crash or hang of the child process fails only that execution.
- The same treatment applies to MCP `stdio` servers (SPEC-006 Q16).

## Consequences
Connector work is prioritised by evidence; coverage claims are reproducible and published (principle 12); host access exists for parity but never without an explicit, narrow admin decision.

## Alternatives considered
Hand-picked "top N" list (rejected: not evidence-based); node-type-level counting only (rejected: overstates coverage when only some operations exist); view-weighted release gate (rejected, SPEC-003 Q15); excluding host-access nodes from V1 (rejected, SPEC-003 Q5: breaks 100 % core coverage).

## Amendments
Changes applied after acceptance; each entry names its source.

- **SPEC-014 (2026-10-13, owner decision OQ1).** §6: an n8n node that has no mapping is imported as the reserved built-in node type `orcon.unsupported@1`, in full import and in paste conversion alike. The node is inert: always disabled and impossible to enable; it keeps the original n8n node JSON and the reason in `ext["orcon.import"]`, and keeps its connections. Publishing a flow that contains it fails with a message naming the node. The importer gains a fragment entry point for paste conversion: the same mapping rules, at most 200 nodes (otherwise `fragment-too-large`), nothing persisted, and n8n `pinData` returned beside the fragment, never inside the flow document.
- **SPEC-003 (Amendment 9, CN2).** §3: `--catalog` takes `installed` (the release's catalog: core nodes, the packages registered in the binary read through their `n8n.types[]`, and the AI nodes the binary maps), `planned` (core nodes, the AI mapping planned in SPEC-006 Annex B, and the greedy selection of §4 computed in the same run) or the path of a planned catalog file in the format the tool writes. `docs/reference/connector-coverage.md` and its JSON are produced with `installed`; planned figures appear only in `docs/reference/connector-roadmap.md` and are labelled as planned. §4: the release gate counts a template as fully importable only if the measured catalog maps every node, AI nodes included; the selection stops on the `planned` figure. The core list is checked against the node inventory of the pinned n8n version, which travels with the corpus; a missing inventory fails the check. n8n sticky notes are editor data (ADR-0007 §2), not nodes, and are left out of node and type counts.
- **SPEC-003 (Amendment 10, CN2).** The node inventory of the pinned n8n version is read from a locally running instance of that version (the node-type list it serves to its editor) or, failing that, from the dated public core-node index of docs.n8n.io; no n8n package is downloaded or opened. A core type of the inventory may lack a mapping only if a row of SPEC-003 Annex B names it (today: Data Table, and n8n's undocumented internal nodes `e2eTest`, `e2eTestPollingTrigger`, `simulate`, `simulateTrigger`); there is no other exemption list. The planned AI mapping classifies every AI type of the inventory from SPEC-006 Annex B (supported, adapted, unsupported, or explicitly listed as not named), and an AI type that is not classified fails the check.
