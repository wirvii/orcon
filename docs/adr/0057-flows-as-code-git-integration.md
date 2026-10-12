<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0057 — Flows as code: repository layout, Git sync modes, environments and credential binding

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: owner answers Q1–Q14, human-attention surface; ADR-0080 enum form; ADR-0077 schema rules)

Accepted by the owner on 2026-10-12 via SPEC-005.

## Context
P3: flows-as-code with Git in V1 (free), stored as `orcon/flow@1` files; per project, "Orcon leads" (every publish committed automatically; conflicts reported, never overwritten) or "Git leads" (GitOps: production applies only what is approved in Git; production editor read-only); environments (dev/staging/prod) with per-environment credential binding by name, never secrets in Git; CLI pull/push/diff for CI/CD. Credentials are write-only and project-owned (ADR-0043); external secret references `$secrets.<alias>.<name>` are allowed only inside credential fields (ADR-0044).

## Decision

### 1. Environment model
An environment is a **project bound to a Git branch** with an environment label (`dev`, `staging`, `prod`, or custom) and a `protected` flag (owner decision Q3). Both are project fields (amendment to ADR-0038 §2); `protected` defaults to `true` for every new project (owner decision Q11) and is changed with `project:update`. Environment projects may live in the same organization or in different organizations/instances; the organization remains the isolation boundary (ADR-0038). Credentials are bound by name in each environment's project (§4). Non-secret per-environment variables (`environments/<env>.yaml`) are exposed to expressions as `$vars` (amendment to ADR-0018). Promotion is a merge between branches in Git. No new tenancy entity.

### 2. Repository layout `orcon/repo@1`
```
orcon.yaml                            # format: orcon/repo@1; projects: [{ slug, path }]
projects/<project-slug>/
  project.yaml                        # name, description, tags, activation map { <flow-slug>: active|inactive }
  flows/<flow-slug>.flow.json         # orcon/flow@1 (deterministic pretty serialization)
  credentials/<credential-name>.yaml  # name, type, description; per environment, optional field values that are ONLY $secrets references; never literal secrets
  variables.yaml                      # non-secret project variables (defaults)
  environments/<env>.yaml             # per-environment non-secret overrides
```
Flow slugs are stored in the flow (`ext."orcon.git".slug`) so renaming a flow does not move its file unless requested. Pinned test data, sample events and execution data are never exported. This is a file format, not an API representation; its values keep the form shown.

### 3. Deterministic serialization
UTF-8, LF, 2-space indented JSON, keys in schema order (unknown `ext` keys sorted), arrays in document order, trailing newline. `write(read(file))` is byte-identical for files Orcon wrote. `contentHash` remains SHA-256 over JCS (ADR-0007 §6).

### 4. Credential binding by name
Nodes reference credentials by name (`credentials: { <slot>: { name } }`, additive to ADR-0007). At dispatch the name resolves to the credential with that name owned by, or shared to, the execution's project (ADR-0041 §5, ADR-0043). Each environment's project holds its own values (entered there, or as `$secrets` references to that environment's external store). Applying or publishing a flow with missing or type-mismatched names fails validation with the list of missing bindings.

### 5. Binding and storage
`git_binding(org_id, id, project_id, repository_url, branch, path, mode: orcon_leads|git_leads, credential_id, committer_name, committer_email, poll_interval_seconds, provider_hook_secret_ciphertext, status, revision)` (API values `ORCON_LEADS`, `GIT_LEADS`); `git_sync_state(org_id, id, git_binding_id, last_synced_commit, flow_hash_document, flow_hash_document_format, last_attempt_at, last_error, behind_version_count)`; `credential_fingerprint(org_id, id, project_id, credential_id, field_name, hmac)` (HMAC keyed by a key derived from the organization key, ADR-0042). Sync runs are `api_operation` rows. Managed with `git:manage`; manual sync and conflict resolution with `git:sync`. Git access uses `go-git` (Apache-2.0) in process. The Git credential (SSH deploy key or HTTPS token) is a project credential (ADR-0043) whose `allowedHosts` is the repository host. One sync at a time per binding (lease). The binding reads `environment` and `protected` from its project (§1); the binding screen warns that a protected Git-leads project makes flows read-only in the editor.

### 6. Mode "Orcon leads"
- Publish, activation change, rollback and archival enqueue a commit job in the same transaction (outbox, ADR-0013 §8). The worker fetches, writes changed files, commits (author = acting user or service account; committer = binding identity; message `orcon: publish <project>/<flow> v<N> (<contentHash[:12]>)`) and pushes fast-forward only. Never force push.
- If the remote advanced: if no file Orcon will write changed since `last_synced_commit`, commit on top of the new head. Otherwise those flows enter `CONFLICT` (base = last exported, ours = Orcon, theirs = Git), nothing is written for them, `git.conflict` is emitted and the editor shows the three-way difference. Explicit, audited resolutions: `ADOPT_GIT` (Git content becomes the draft; publishing it commits normally) or `KEEP_ORCON` (commit Orcon content on top of Git's head). Push races retry a bounded number of times.
- If Git is unreachable, publishing still succeeds; commits stay queued in the outbox with backoff; the binding shows "Git behind by N versions" (owner decision Q4).
- Draft saves are not committed.

### 7. Mode "Git leads" (GitOps)
- Orcon fetches on a signed provider webhook (`POST /api/v1/git-bindings/{gitBindingId}/provider-hook`, a `protocol`-tagged endpoint verified with the GitHub/GitLab presets of ADR-0022 §5) and by polling (default 60 s).
- Per new commit: parse changed files; validate every changed flow (ADR-0007 §9 plus §4 bindings) and the activation map; any failure → nothing applied, `git.sync.failed` with file, node and reason. Otherwise, in one organization-scoped transaction: publish each changed flow as a new version (publisher = binding identity, commit id and author recorded), apply activation changes (ADR-0024), archive flows whose files were deleted (deactivate; history kept).
- In a `protected` Git-leads project, writes to flows, credential metadata and variables return `409 project-managed-by-git`; test runs, execution operations and credential **values** remain available per permissions. Non-protected Git-leads projects may allow drafts, but publication only comes from Git.

### 8. Secret leakage prevention
- No field exists for secret values; credential files allow only metadata and `$secrets` references.
- Every file is scanned before any commit and by the CLI before push/validate: known token patterns (including `orcon_*`, cloud keys, private key blocks), high-entropy literals in node parameters, and equality with any credential value of the project via `credential_fingerprint` (values are never compared in clear outside the credential service). A finding blocks the commit with file and JSON Pointer; a holder of `git:manage` may mark a specific finding as a false positive (audited `git.secret_finding_overridden`).
- Pinned/test data and execution data are excluded by construction.

### 9. CI usage
- Pull-request checks: `orcon flows validate ./projects` (offline), `orcon flows diff --against <context>`, optional `--check-bindings` against the target environment with a read-only key.
- Deployment: for Git-leads targets, merging deploys; CI may run `orcon git sync --wait` to apply immediately and fail on apply errors. For projects without a binding: `orcon flows push --publish --expect-revision`.
- Authentication: federated workload identity (ADR-0056 §9, recommended: no Orcon secret stored in CI), or service-account keys / OAuth client credentials (`private_key_jwt`).

## Consequences
Reviewable, promotable flows without secrets in Git; production can be locked to Git; Git outages never stop production in Orcon-leads mode.

## Alternatives considered
Environment = separate instance only (still possible, not mandated); committing drafts (noisy, not deployable state); shelling out to `git` (breaks the self-contained binary); last-write-wins between Git and Orcon (rejected by P3).
