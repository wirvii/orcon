<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0033 — Connector trust, signing, catalog and trust policy

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-003.

## Context
K3: official connectors are bundled; Orcon runs a public catalog with strict code and behaviour review where every submission stays "in review" until approved; approved connectors are signed server-side by Orcon's catalog key after review (never by the public converter); the runtime verifies signatures and shows a seal (official / verified-by-Orcon / unreviewed); permissions are shown and approved at install and enforced; versions are pinned with updates offered and rollback available; private catalogs are supported; admins may allow their own catalogs and unsigned connectors. Every engine capability is open source under Apache-2.0 and none is paid or withheld (`vision.md`, `principles.md`, owner decision SPEC-003 Q1). Any deployment — including a separate platform built on top of Orcon — may choose a stricter policy (only Orcon-signed connectors from Orcon's catalog) purely through configuration. Enumerated values follow ADR-0029 §10.

## Decision

### 1. Repository format: TUF
Catalogs follow The Update Framework specification (roles, thresholds, expirations, consistent snapshots), implemented with `go-tuf/v2` (Apache-2.0; licence confirmed in the implementing slice). Mirrors are untrusted transport (HTTP server or static directory).

Orcon public catalog roles (TUF role names keep their standard spelling):
- `root`: offline keys, threshold 2 of 3, hardware tokens held by distinct Orcon maintainers; expiry 1 year.
- `targets` with delegations `official` (key in KMS/HSM, used only by Orcon's release pipeline for Orcon-authored packages) and `verified` (key in KMS/HSM, used only by the catalog service after an approved review).
- `snapshot` (expiry 7 days) and `timestamp` (expiry 1 day), online KMS keys.
Target custom metadata: `{ connectorId, version, kind, seal, reviewId?, reviewedAt?, manifestDigest, permissionsDigest, license, sourceRepository, sourceCommit, buildRecipeDigest }`, with `kind` and `seal` in UPPER_SNAKE.

### 2. Trust roots
The binary embeds the initial `root.json` of the Orcon public catalog. Private catalogs are added by the admin with their own `root.json` and a display name. Root rotation follows TUF (each new root signed by the previous root's threshold).

### 3. Seals
Exactly three seals (owner decision, SPEC-003 Q2):
- `OFFICIAL`: target signed through the `official` delegation of the Orcon public catalog.
- `VERIFIED`: target signed through the `verified` delegation of the Orcon public catalog.
- `UNREVIEWED`: everything else — packages from private catalogs (shown as "not reviewed by Orcon" with the label "signed by <catalog name>"), local unsigned files (labelled "local file"), converter output. Native connectors bundled in the binary are `OFFICIAL`.
The public converter and all local tooling never hold Orcon keys and never produce signatures of the `official`/`verified` delegations.

### 4. Trust policy (configuration, not code)
```
connectors.trust: {
  profile: "OPEN" | "STRICT" | "CUSTOM",
  allowedSeals: ["OFFICIAL", "VERIFIED", "UNREVIEWED"],
  catalogs: ["orcon-public", ...],
  allowUnsigned: bool, allowLocalFiles: bool,
  metadataRefresh: duration (default 6h),
  locked: bool
}
```
- `OPEN` (default): all seals; catalogs = Orcon public + admin-added private ones; `allowUnsigned=false` and `allowLocalFiles=false` until the admin enables them (K3 admin setting).
- `STRICT`: `allowedSeals = [OFFICIAL, VERIFIED]`, `catalogs = [orcon-public]`, `allowUnsigned=false`, `allowLocalFiles=false`, `locked=true`.
- `locked=true` means the policy is read only from the configuration file/environment at startup and is read-only in the UI and API.
Catalog names (`orcon-public`) are identifiers. Both profiles ship in the same Apache-2.0 binary and are free for every installation; no engine capability is gated by edition, licence key or payment. The policy is evaluated at install and again at publication and dispatch; tightening it makes non-conforming installed packages "blocked by policy" (nodes fail validation and dispatch with an explicit message). Revocation behaviour is fixed by §6 and is not a policy option.

### 5. Verification
- Install: full TUF client update (root chain, timestamp, snapshot, targets, delegations), target length and hashes, custom metadata consistency with the package manifest, policy check, permission approval (§7).
- Load: workers verify the package SHA-256 against the recorded verified digest before compiling or interpreting it.
- Freshness: expired timestamp/snapshot metadata blocks new installs and updates (freeze-attack protection) except offline mode (§8).

### 6. Revocation
Owner decision, SPEC-003 Q3. Revoking removes the target from `targets` and adds an entry to a signed `revocations` target (`connectorId, version, severity: CRITICAL|ADVISORY, reason, advisoryUrl, fixedIn?`). Instances refresh metadata every `metadataRefresh` and on install, and record the entry in the instance table `connector_revocation` (ADR-0029 §8).
- `CRITICAL`: dispatch of the affected node is blocked. Flows stay published and active (they are not deactivated); each dispatch fails with the permanent error `ConnectorRevoked`, whose message names the connector, the revoked version, the reason and the version to upgrade to (`fixedIn`, when present). The admin may record a supervised override per package and version (actor, reason, expiry), stored in `connector_revocation_override` and audited; it lapses at expiry.
- A `CRITICAL` revocation that affects installed packages raises, for each affected organization, the ADR-0073 §13 operational alert `CONNECTOR_REVOKED` and the public event `connector.revoked` (ADR-0055 §2): immediate notice to the organization's Owners and Admins and to the owners of each affected project, listing the affected flows and the fixed version, with escalation until acknowledged (owner decision SPEC-003 N2).
- `ADVISORY`: warning in editor and dashboard; dispatch continues.
There is no instance-wide setting that downgrades `CRITICAL` to a warning.

### 7. Permission approval
At install the admin sees seal, publisher, licence, proof levels of safety contracts, network domains, credential types, raw credential access, sensitive fields and resources; approval is stored in `connector_grant` with `permissions_digest`. An update whose permissions digest differs requires a new approval showing the difference; until approved, nodes stay on the previous version.

### 8. Offline install
`orcon connector install --bundle <file.orconbundle>`: the bundle carries the package and the full TUF metadata chain. Verification uses configured roots. Expired freshness metadata is accepted only with `--offline` and admin confirmation, recorded with the bundle's metadata versions; the UI shows that revocation information is only as fresh as the bundle.

### 9. Submission and review (Orcon catalog service)
The catalog service is open source (Apache-2.0) in this repository as `cmd/orcon-catalog`, a separate binary from `orcon` (not a runtime role); anyone may run their own (owner decision, SPEC-003 Q8).
1. Publisher authenticates (verified code-hosting identity) and submits `{ source repository, commit, build recipe }` or a declarative package. State `IN_REVIEW`.
2. Automated stage: hermetic reproducible build (digest must equal the submitted package if one is provided), manifest/schema validation, SPDX licence allowlist and redistribution check, wasm import allowlist, conformance kit, change-descriptor completeness against the previous version (ADR-0034 §4), permission sanity checks, and the authoring checks of ADR-0079 §7 with the same rule identifiers as Orcon CI: `orcon connector lint --admission`, `orcon connector docs --check`, the secret and personal-data detector over recorded tests, and `orcon connector diff --verify-bump`.
3. Human stage: code review (wasm source) and behaviour review — run in a sandbox with traffic capture against the declared domains and a provider test account where available; safety contracts checked against provider documentation; `proofLevel = VERIFIED` where demonstrated.
4. Decision recorded with reasons (`APPROVED` or `REJECTED`). Approval → the service signs the target through the `verified` delegation via KMS; rejection → reasons returned. Nothing is published unsigned. No review-time SLA is promised.

### 10. Key management
Online keys never leave KMS/HSM; access limited to the release pipeline (`official`) and the catalog service (`verified`, `snapshot`, `timestamp`); scheduled rotation yearly and on incident; compromise procedure: root ceremony revokes the key, delegates to a new one, re-signs affected targets; short timestamp expiry bounds exposure.

### 11. Private catalogs
`orcon catalog init` (creates root/targets/snapshot/timestamp with file keys or PKCS#11), `orcon catalog add <pkg>`, `orcon catalog sign`, `orcon catalog serve` (static HTTP). Organisations may run their own review process; their packages carry the `UNREVIEWED` seal with the label "signed by <catalog name>" (SPEC-003 Q2).

## Consequences
Trust is explicit, revocable and verifiable offline; the strict policy is free, open configuration in the same code; known-malicious versions stop running while flows remain intact and point to the fix; key compromise is recoverable; catalog review and Orcon CI enforce the same authoring rules.

## Alternatives considered
Plain detached signatures (rejected: no rotation, freshness or revocation model); Sigstore keyless signing alone (rejected for runtime trust: depends on online transparency services and identity providers; may complement review provenance later); build-time edition flag or paid unlock for `STRICT` (rejected: creates a fork and contradicts the no-paid-features principle); a fourth "organisation-trusted" seal (rejected, SPEC-003 Q2: dilutes the meaning of seals); revocation that only warns (rejected, SPEC-003 Q3: lets known-malicious code keep running).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-004.** §4: the trust policy is instance-level; an organization may only restrict it further (`allowedSeals` must be a subset). `connector_grant.project_id` becomes `connector_grant(org_id, project_id?, …)`: the table gains `org_id` (ADR-0077 §5).
