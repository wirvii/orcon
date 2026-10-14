<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0034 — Connector versioning, per-node pinning and migration descriptors

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-003.

## Context
K4: each node instance stores the connector version it was created with; multiple versions are installed side by side; existing flows are unaffected by new versions; new nodes use the latest; the editor offers per-node one-click upgrades with automatic migration when mechanical and highlights otherwise; a version is retired only when no flow references it; publishers provide machine-readable change descriptions between versions, enforced by review; n8n `typeVersion` is preserved on import. ADR-0079 §6 (Proposed) amends §4 and §5: a rename is a major version.

## Decision

### 1. Pinning
Each flow node carries `connector { version: semver, digest: "sha256:…" }` (additive field in `orcon/flow@1`, included in `contentHash`). Validation level 5 requires the pinned version installed (or installable from a configured catalog), permitted by policy, granted, and not `CRITICAL`-revoked (unless a supervised override is active, ADR-0033 §6).

### 2. Side by side
Installed versions coexist in `connector_package`; runtime caches (wasm compilation, declarative plans) are keyed by digest. New nodes use the latest installed version permitted by policy. Installing a version changes no existing node.

### 3. Upgrade in the editor
For each node whose pin is older than the latest permitted version, the editor shows "new version available" with the concatenated changelog. "Upgrade" applies the composed migration chain to the node in the **draft**: if every step is mechanical it is applied and shown as a diff; otherwise mechanical steps are applied and the remaining items are highlighted for review. Publishing remains explicit (ADR-0024). Downgrade is offered when every inverse step is mechanical. A CLI `orcon connector upgrade --flow … --dry-run` gives the same result in bulk.

### 4. Change descriptors `orcon/connector-changes@1`
File `changes/<from>..<to>.json` for every consecutive released pair: `{ from, to, changes: [ … ] }` with operations (operation names are identifiers):
additive — `addNode{node}`, `addOperation{node, operation}`, `addCredentialType{credential}` (ADR-0079 §6);
mechanical — `renameParameter{node, path, to}`, `moveParameter{from, to}`, `addParameter{path, default, existingValue}`, `removeParameter{path, ifValue?}`, `mapValues{path, map}`, `changeDefault{path, old, new}` (materialises `old` into existing nodes), `renameOperation{from, to}`, `renamePort{from, to}`;
review — `removeOperation`, `outputShapeChanged{paths, description}`, `safetyChanged{operation, before, after}`, `credentialTypeChanged`, `behaviorChanged{description}`;
admin — `permissionsChanged{before, after}` (requires a new grant, ADR-0033 §7).
Mechanical operations declare their inverse. "Mechanical" describes how a node is migrated, not the size of the version bump (§5). `orcon connector diff <a> <b>` computes the schema difference between two manifests; `orcon connector diff --classify` reports its semantic-version class and `--verify-bump` rejects a release whose declared bump differs from the detected one; catalog review rejects a version whose difference is not fully covered by descriptors. Migration functions are pure, total and unit-tested (same discipline as ADR-0007 §7).

### 5. SemVer (as amended by ADR-0079 §6)
- Patch: no difference in the manifest or schemas (body fix, documentation, tests, icon).
- Minor: additions only — node, operation, optional parameter, credential type, output port, option value, wider permissions, new trigger.
- Major: any removal or rename (including mechanical renames such as `renameParameter`, `renameOperation`, `renamePort`), a type change, a new required parameter without default, a removed option value or narrower accepted value set, or a weaker safety level. A major release also changes `@major` in the node type.
`orcon connector diff --verify-bump` rejects a mismatch in either direction. Every version has a changelog entry in English and Spanish (ADR-0029 §4). Versions are pinned exactly. Catalogs keep old versions; retirement follows §6.

### 6. Retirement
Owner decision, SPEC-003 Q4. A version is referenced if any of these pins it: a flow draft; a flow's active published version; a flow version pinned by a non-terminal execution; a flow version pinned by a sub-flow call with a fixed version. Older, inactive published flow versions do not count. Pins are recorded as real rows in the organization-scoped table `connector_reference(org_id, id, flow_id, flow_version_id, connector_id, connector_version)` (`flow_version_id` NULL for the draft), written in the same transaction that saves the draft or publishes the version, so the count never reads inside a flow document (Go rulebook DA-3 as amended; ADR-0077 §9). The count across organizations runs through an engine-plane function (ADR-0047). Retiring requires zero references (checked transactionally); afterwards, republishing an older flow version that pins a retired connector requires upgrading those nodes first. Automatic retirement after N unreferenced days is optional (default off).

### 7. Native versions in the binary
Each Orcon release declares the native connector versions it supports. At startup, if any referenced pin names a native version the binary does not support, the binary refuses to start with a list of affected flows and a suggested `orcon connector upgrade` plan; releases keep supporting referenced native versions for at least one major Orcon release after superseding them.

### 8. n8n typeVersion
Imported nodes keep `ext["orcon.import"] = { n8nType, n8nTypeVersion, n8nNodeId }`. The manifest's `n8n.types[].typeVersions` and per-typeVersion `parameterMap` define the conversion; an unmapped typeVersion is reported, never approximated.

## Consequences
Upgrades are deliberate per node, never surprise changes; rollback works at node and flow level; catalog review enforces honest change descriptions; a renamed parameter or operation is always announced as a major version.

## Alternatives considered
Global connector upgrade for all flows (rejected by K4); floating "latest compatible" pins (rejected: silent behaviour change); counting every historical flow version as a reference (rejected, SPEC-003 Q4: nothing could ever be retired); mechanical renames in minor releases (rejected by ADR-0079 §6: users see a rename as breaking); counting references with a query inside flow documents (rejected: DA-3 as amended).

## Amendments
Changes applied by specifications the owner approved; each entry names its source.

- **SPEC-003 (Amendment 9, CN2).** §8: `ext["orcon.import"]` is the closed object `{ n8nType: string, n8nTypeVersion: number, n8nNodeId?: string, reviewStatus?: "NEEDS_REVIEW" | "REVIEWED", reviewReasons?: ["HTTP_WRITE" | "HTTP_METHOD_EXPRESSION" | "UNSAFE_OPERATION"], original?: object }`. `reviewReasons` is present, non-empty and without repeats exactly when `reviewStatus` is present. The importer sets `reviewStatus: "NEEDS_REVIEW"` on every node it imports with an `UNSAFE` contract (ADR-0037 §6): an HTTP Request with a write method (`HTTP_WRITE`) or with a method given by an expression (`HTTP_METHOD_EXPRESSION`), and a connector operation whose safety level is `UNSAFE` (`UNSAFE_OPERATION`). A person who reviews the node sets `REVIEWED` in the draft; the reasons stay. Activation (ADR-0024) fails, naming the nodes, while any node of the version being activated is `NEEDS_REVIEW`; publication does not fail. `original` holds the original n8n node and appears only on `orcon.unsupported@1` nodes (SPEC-014 OQ1). Values are UPPER_SNAKE (ADR-0007 §11).
