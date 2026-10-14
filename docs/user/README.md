<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Orcon user documentation

This directory holds the pages of the Orcon user documentation site, written by hand in English and Spanish. The site tool, languages, versioning and publication are decided in ADR 0083 §9. This file is the guide for people who write or review these pages.

## Layout

| Path | Contents |
| --- | --- |
| `docs/user/en/` | Pages in English. The English page is the source. |
| `docs/user/es/` | The same pages in Spanish, one file for every English file, at the same path. |
| `docs/user/support-matrix.yaml` | The support matrix. It generates the page `support/matrix` and feeds the install test matrix. |
| `docs/reference/` | Pages generated from code or contracts (configuration, command line, metrics, events, problem codes). They are not written here. |

The site itself (`apps/docs`, built with Astro Starlight and Pagefind) copies these pages at build time. Nothing in this directory is built or published yet, so the pages are checked by reading until the site exists.

## Information architecture

The pages below are routes under `docs/user/en/` and, mirrored, under `docs/user/es/`. A route marked **generated** is produced by a tool and is not written by hand.

1. Start: `index.md`, `getting-started/quickstart.md`, `getting-started/concepts.md`.
2. Install: `install/overview.md`, `install/binary.md` (the one-line installer and the systemd unit), `install/container.md`, `install/docker-compose.md`, `install/kubernetes-helm.md`, `install/verifying-releases.md` (from `docs/releases/verifying.md`), `install/uninstall.md`.
3. Configuration: `configuration/overview.md`, `configuration/reference.md` (**generated**, English and Spanish), `configuration/secrets.md`, `configuration/first-run.md`, `configuration/master-key-and-kms.md`, `configuration/tls-and-reverse-proxy.md`, `configuration/network-connections.md` (**generated**).
4. Building flows: `building-flows/` with `editor`, `triggers`, `nodes-and-data`, `expressions`, `errors-and-retries`, `waits-and-human-tasks`, `sub-flows`, `versions-and-publishing`, `flows-as-code` and `ai-agents`. The slice that delivers each feature writes its page.
5. Reference: `reference/` with connectors, API, command line, events, problem codes and metrics. All are generated; the command line and metrics references are English only.
6. Operations: `operations/upgrade-and-rollback.md`, `operations/backup-and-restore.md`, `operations/sqlite-to-postgresql.md`, `operations/monitoring.md`, `operations/alerts.md` (one `## Orcon…` heading per alert), `operations/scaling-and-topology.md`, `operations/capacity.md`, `operations/retention-and-storage.md`, `operations/high-availability.md`, `operations/troubleshooting.md`.
7. Security: `security/overview.md`, `security/authentication-and-sso.md`, `security/authorization.md`, `security/credentials-and-secret-stores.md`, `security/encryption-and-keys.md`, `security/multi-organization.md`, `security/audit-and-siem.md`, `security/connector-trust.md`, `security/hardening-checklist.md`, `security/reporting-vulnerabilities.md`.
8. Migrating from n8n: `migrating-from-n8n/overview.md`, `importing-flows.md`, `expressions.md`, `credentials-and-users.md`, `community-nodes.md`, `ai-nodes.md`, `activation-checklist.md`, `faq.md`.
9. Support: `support/matrix.md` (**generated** from `support-matrix.yaml`), `support/versioning-policy.md`, `support/getting-help.md`.

## Pages that exist today

These pages exist in English and Spanish, together with `support-matrix.yaml`:

- Start: `getting-started/quickstart.md`.
- Install: `install/overview.md`, `install/binary.md`, `install/container.md`, `install/docker-compose.md`, `install/kubernetes-helm.md`, `install/verifying-releases.md` and `install/uninstall.md`.
- Configuration: `configuration/overview.md`, `configuration/secrets.md`, `configuration/first-run.md`, `configuration/master-key-and-kms.md`, `configuration/tls-and-reverse-proxy.md` and `configuration/network-connections.md`.
- Operations: `operations/upgrade-and-rollback.md`, `operations/backup-and-restore.md` and `operations/alerts.md`.
- Security: `security/overview.md`, `security/authentication-and-sso.md`, `security/authorization.md`, `security/credentials-and-secret-stores.md`, `security/encryption-and-keys.md`, `security/multi-organization.md`, `security/audit-and-siem.md`, `security/connector-trust.md`, `security/hardening-checklist.md` and `security/reporting-vulnerabilities.md`.
- Building flows: `building-flows/editor.md`.
- Migrating from n8n: `migrating-from-n8n/overview.md`.
- Support: `support/matrix.md`, `support/versioning-policy.md` and `support/getting-help.md`.

Two of these pages are hand-written stand-ins for pages that the site will generate: `configuration/network-connections.md` (from the registry of outgoing connections) and `support/matrix.md` (from `support-matrix.yaml`). Each says so in its opening note, and the generated page replaces it when the tool exists.

The other routes are written by the slices that deliver their features.

## Front matter

Every page starts with YAML front matter, followed by the SPDX header as an HTML comment.

```markdown
---
title: Backup and restore
description: One sentence saying what the page lets the reader do.
sources:
  - ADR-0083
sourceHash: <only in docs/user/es>
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
```

- `title` and `description` are required.
- `sources` lists the ADRs that back the page. It can be an empty list for a page that describes no behavior.
- `sourceHash` is required in `docs/user/es/` only. It is the SHA-256 of the English file the Spanish page was translated from, computed over the whole file as committed. When the English file changes, the hash no longer matches and the site shows a "translation outdated" notice. This is a warning, not an error.

## Languages

- Hand-written guides exist in English and Spanish. Every page `docs/user/en/x.md` has a page `docs/user/es/x.md`; a missing Spanish page is an error.
- The configuration reference is published in English and Spanish.
- The command line and metrics references are published in English only. The API, events and problem-code references are generated in English and are English only too. The Spanish site shows them with the "page not translated" notice and the parity check skips them through an explicit list.
- Spanish is written only in `docs/user/es/` (and in the product's translation catalogs). Every other file in the repository is English.

## Style

- Plain language. Say what the reader can do before explaining how it works.
- One task per page. Split a page that answers two questions.
- Commands are copyable: one command per line, no prompt characters, no placeholders left unexplained. A placeholder is written in angle brackets and explained right after the block.
- Name no AI model provider outside `docs/reference/connectors/ai/**`. Describe model features generically.
- Use the name n8n only to say what it is, as a nominative reference. Do not use its logos or screenshots of its interface.
- A page describes only behavior that the version of the site brings. Until a feature ships, its page begins with a note saying that the page describes specified behavior that is not implemented yet. The slice that implements the feature removes the note.
- Numbers about performance come only from a published benchmark result under `docs/benchmarks/`, and the page cites it. Until a release is measured, the page shows the performance budgets under the title "Targets, not measurements".

## Changing user-visible behavior

A change that users can see updates its pages in `docs/user/en` and `docs/user/es` in the same pull request.
