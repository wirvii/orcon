<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR 0083: Operations foundations — distribution, configuration, first run, backups, upgrades, no phone-home, observability and user documentation

Status: Proposed

## Context
Orcon promises an easy first run (principle 3), one artifact with many roles (principle 4, ADR 0002), operational minimalism (principle 5) and observability by design (principle 9). ADR 0004 leaves backups and disaster recovery open. The maintainers decided (BL-017, 2026-10-12): four official install paths verified on every release; flags over environment over a YAML file over secure defaults, with every secret readable from a file; a web first-run wizard and a headless equivalent; built-in encrypted backups with key material kept separately; no telemetry; a versioned English and Spanish documentation site; and a monitoring kit. Related decisions: ADR 0015 (storage, migrations, compatibility floor), ADR 0016 and 0017 (blobs, retention), ADR 0033 (connector catalog trust), ADR 0042 (key hierarchy and installation ceremony), ADR 0048 (audit), ADR 0052 (routes outside /api/v1), ADR 0073 (human-attention notifications), ADR 0075 and 0076 (layout and libraries), ADR 0081 (signed releases).

## Decision

### 1. Distribution
1. Four official install paths: the single binary (linux, darwin, windows; amd64, arm64) with one-line installers that verify the cosign signature of the checksum file and the SHA-256 of the archive before installing (fetching a pinned cosign with an embedded checksum when none is available; there is no option to skip verification); the multi-architecture image, non-root, with a data volume at /var/lib/orcon; docker-compose examples for SQLite and for PostgreSQL with two workers; and a Helm chart for PostgreSQL that deploys api, scheduler and worker separately.
2. Every release runs an install-and-run test for every path before publishing (on the release pull request) and again against the published assets. The same suite upgrades from the previous release.
3. The Helm chart never accepts secret values; it references existing Secrets mounted as files. Pods meet the Kubernetes "restricted" Pod Security profile.

### 2. Configuration
1. One schema in internal/platform/config defines every server key. Precedence per key: command-line flag, then ORCON_* environment variable, then the YAML file given by --config or ORCON_CONFIG, then the default. Defaults are secure: loopback listener, metrics and exporters off, update check off.
2. Environment names derive from the key path in UPPER_SNAKE, except three names fixed by ADR 0042 and ADR 0039 (ORCON_PUBLIC_URL, ORCON_MASTER_KEY, ORCON_MASTER_KEY_BACKUP_ACK). Names used by the CLI client (ADR 0058) are reserved.
3. Every secret key accepts a file variant (key File in YAML, NAME_FILE in the environment, --path-file as a flag). Secret values are never accepted as flags.
4. `orcon config check` prints the effective value and the source of every key, with secrets redacted, and every validation error, without network access unless --probe is given.

### 3. First run
1. Until setup is complete the server accepts only the setup operations, protected by a one-time setup token that is shown on an interactive terminal or retrieved with `orcon setup token`, never written to logs.
2. The wizard runs the installation ceremony of ADR 0042 §5 first (generate or validate the master key, download the recovery kit only over HTTPS or loopback, type the last eight characters of the fingerprint), then creates the owner account and the default organization. No organization data is written before the keys are verified.
3. The headless equivalent is `orcon key init`, the acknowledgement variable of ADR 0042 §5, and `orcon setup owner` or the setup.owner.* keys consumed at first start.

### 4. Backups, restore and migration to PostgreSQL
1. A backup is a manifest, an encrypted data artifact (database and filesystem blobs, zstd inside age, the file key wrapped by the master key and by the optional recovery key) and a separate key artifact (master key material encrypted to operator age recipients, or the KMS reference), written to a different target. Data without keys is useless.
2. SQLite: one read transaction copied with the SQLite online backup API in a single step; the writer is never blocked. PostgreSQL: one REPEATABLE READ READ ONLY snapshot, copied per organization with the transaction-local organization context so row-level security stays enforced and no BYPASSRLS role is needed; an engine-plane function counts rows in the same snapshot to prove completeness. The snapshot is spooled locally and uploaded after the transaction ends, so no transaction is held across an external call. Blob files referenced by the snapshot are protected from deletion while a backup runs. S3 blobs are listed, not copied.
3. Backups run inside the server (SQLite allows one process), on demand through the API or on a schedule, to a local directory or S3-compatible storage, with grandfather-father-son retention. Verify restores into a temporary location and checks integrity, schema checksums, row counts, key canaries and blob hashes.
4. Restore is a local command only, requires typed confirmation, is audited, and leaves the instance on hold: no work is claimed and nothing fires until an instance administrator releases it. ADR 0073 §13 gains the operational alert type INSTANCE_RESTORED. External operations that were in flight follow the normal journal recovery after release; Orcon never claims exactly-once effects across a restore. Audit anchors newer than the backup are archived and the discontinuity is recorded, so verification does not report it as tampering.
5. Instance-level targets (backups, S3 blobs) may use the server's ambient cloud identity; organizations still need the audited grant of ADR 0044 §2.
6. `orcon store migrate-to-postgres` (ADR 0015 §7) copies a stopped SQLite instance into an empty PostgreSQL database in one transaction, verifies per-table counts and content checksums and every blob hash, and leaves the source untouched as the way back.

### 5. Upgrades
1. `orcon store schema plan` reports the resulting compatibility floor and whether every live process stays at or above it (rolling) or not (full stop). Startup migration applies only in the rolling case. Every process checks the effective floor on each heartbeat and drains and exits with code 75 when it falls below. This amends ADR 0015 §4 additively.
2. Rolling back is running the previous binary when it is at or above the floor; otherwise it is restoring the pre-upgrade backup. SQLite takes a local backup automatically before migrating.

### 6. No phone-home
1. Orcon never sends usage data or installation identifiers. Every outbound connection is caused by operator configuration that names the destination, by user flows and credentials, or by an explicit administrator action. Each outbound-capable component is registered with a purpose from a closed list, and the network connections page is generated from that registry.
2. Remote catalog metadata (ADR 0033) is fetched on every install or update from a remote catalog and, only while at least one package installed from a remote catalog by an administrator remains installed, refreshed periodically. Packages preinstalled in an image or installed from a bundle file never schedule a refresh. An instance that has never installed from a remote catalog never contacts one.
3. An update check exists only when the operator enables it; it fetches the public release list without sending anything about the installation.
4. Automated tests prove that a fresh install makes no unsolicited outbound connection: a packet capture of the release image in an isolated network, a browser request log, and an in-process run with the clock advanced 48 hours.

### 7. Health and the operations listener
1. /healthz reports liveness and never depends on the database; /readyz reports readiness per role (store reachable, schema compatible, keys verified, not draining).
2. Every role may serve an operations listener on a separate address that exposes only /healthz, /readyz and /metrics. It is not part of the api router of ADR 0052 §1 and never serves the API, the UI or trigger ingress.
3. SIGTERM makes /readyz fail at once, stops new work, returns leased work and exits within the configured shutdown timeout.

### 8. Observability export
1. Metrics are exported in the Prometheus exposition format on the operations listener and/or over OTLP; traces and logs over OTLP when configured; logs are JSON lines on standard output, redacted.
2. Instrument names use dot-separated segments with underscores inside a segment; the Prometheus name replaces dots with underscores and adds the unit suffix and _total for counters. Metric labels never carry organization, flow, execution or user identifiers.
3. One catalog in internal/platform/telemetry lists every metric; a test fails when a registered metric is missing from it. The monitoring kit (Grafana dashboards as JSON, Prometheus alert rules with unit tests) is checked against the generated catalog.

### 9. User documentation
1. The user documentation site is Astro Starlight with Pagefind search, built from content in docs/user and generated references in docs/reference, with no external resources. Guides and the configuration reference are published in English and Spanish; the generated CLI and metrics references are English only.
2. It is versioned per minor release, built from the release tag, and published on GitHub Pages from a gh-pages branch with /next/ from main and /latest/. Broken internal links and anchors fail the build.

### 10. Support and capacity
1. A support matrix with tiers is published from one data file that also drives the install test matrix. PostgreSQL 14 to 17 are supported in version 1; PostgreSQL 14 leaves the matrix with the first minor release published after 2027-05-12.
2. Capacity guidance quotes only measured results of the release's published benchmark; until a release is measured it shows the performance budgets as targets.

## Consequences
- Operators get the same behaviour from every install path, and every release proves it.
- A backup taken while the instance runs is consistent to one snapshot on both stores, encrypted, and restorable only with the key material kept elsewhere.
- Restores are safe by default at the cost of a manual release step.
- Installations that never use a remote catalog make no catalog connections; they receive connector revocations only with their next bundle or release.
- New packages: internal/instance, internal/backup, internal/adapter/otel, internal/adapter/backuparchive, internal/adapter/backuptarget/{fs,s3}, internal/adapter/codec/configyaml. New instance tables: backup_run, instance_hold, instance_setup. New engine-plane function: orcon_engine.backup_row_counts.
- Large PostgreSQL installations are pointed to native backup tooling for the database part.
- The test suites are heavier; the install and no-egress tests run on release candidates, after publishing and nightly, not on every pull request.

## Alternatives considered
- A privileged BYPASSRLS role for logical backups: rejected; it weakens the isolation guarantees and needs a superuser to create.
- VACUUM INTO for SQLite: valid; the online backup API was chosen because it copies pages without rewriting the database.
- Copying S3 blobs into every backup: rejected for V1; cost and time grow with the bucket, and bucket versioning already provides durability.
- Restoring straight into a running state: rejected; effects sent after the backup could be repeated without anyone deciding.
- Opt-in usage statistics: rejected by the maintainers.
- Refreshing remote catalog metadata on every installation regardless of use: rejected by the maintainers; an instance that does not use the catalog must not contact it.
- A docs versioning plugin or rebuilding every version on each release: rejected; a gh-pages branch with one folder per minor release is simpler and reproducible.
- Helm values with inline secrets: rejected; they persist in the release history.
