---
name: orcon-connector
description: Applies the Orcon connector authoring standards - declarative-first kind choice with written native justification, resource and operation naming with a closed verb vocabulary, manifest, credentials and egress, per-operation EOSC safety, pagination, rate limits, error codes, recorded and fault tests, nightly live tests, semantic versioning, generated docs, icon, n8n mapping and catalog admission. Use when creating, changing or reviewing an Orcon connector or a catalog submission.
license: Apache-2.0
compatibility: Rules in references/ are self-contained; scripts/ require an Orcon repository checkout with Go, GNU make and the orcon binary; third-party authors need only the orcon binary.
metadata:
  version: 1.1.0
  pinned: true
  owner: wirvii
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Orcon Connector

The complete rules are in `references/`. This file is the procedure and the checklist; open the reference named in each step when the step applies. Rules marked [official] apply only to `orcon.*` connectors; the rest also apply to catalog submissions.

Native Go code also follows the `orcon-go` skill (`../orcon-go/SKILL.md`); the connector icon frame and user-visible wording follow `orcon-ui` (`../orcon-ui/SKILL.md`).

Precedence: accepted ADRs > accepted `orcon-go` rules > connector ADRs 0029-0037 and 0079 > `references/` > reviewer preference.

| Reference | Rules |
|---|---|
| `references/kinds.md` | KD - declarative first, native justification with a gap id, gap registry |
| `references/naming.md` | NM - ids, singular resources, the 17 verbs, parameters, getMany, ids, dates, i18n, value casing |
| `references/manifest.md` | MF - required fields, permissions, domains, sensitive fields, changelog |
| `references/credentials-and-egress.md` | CR - credential handle, OAuth, test operation, raw access |
| `references/operation-safety.md` | SF - safety level per operation, manualCheck, proof level |
| `references/pagination-and-rate-limits.md` | PG - pagination strategies, page caps, provider limits |
| `references/errors.md` | ER - the 14 error codes, status mapping, class by safety level |
| `references/testing.md` | TE - cassettes, scrubbing, fault matrix, conformance, live tests |
| `references/versioning.md` | VR - patch, minor, major, bump check, change descriptors, changelog |
| `references/documentation.md` | DC - generated page, connection guide, n8n equivalence |
| `references/icon.md` | IC - icon file rules |
| `references/n8n-mapping.md` | N8 - n8n types, parameter maps, credentials |
| `references/catalog-admission.md` | CA - automated and human admission |
| `references/checks.md` | every automated check, where it runs, inherited checks, mneme |

## When To Use

- You are about to create or change a connector under `internal/adapter/connectors/` or a connector package for the catalog.
- You add an operation, parameter, credential type, trigger, cassette or version.
- You review a connector pull request or a catalog submission.
- A design asks whether a service should be declarative or native.

## Critical Rules

1. Declarative by default; native only with `authoring.json` naming a gap from `docs/reference/declarative-gaps.md` [official] (KD-2, KD-3).
2. Operations are `<resource>.<verb>` with a singular resource and one of the 17 vocabulary verbs; any other verb needs a written reason (NM-3 to NM-5).
3. Every `getMany` offers `returnAll` and `limit`; the record id parameter is `<resource>Id`; outputs have a top-level `id` (NM-7, NM-8, NM-10).
4. Connector-defined dates are RFC 3339 in UTC; provider fields are returned unchanged (NM-11).
5. Every user-visible string exists in `i18n/en.json` and `i18n/es.json` (NM-12).
6. Network access is only to exact domains from the provider documentation, over HTTPS (MF-5 to MF-7).
7. Credentials reach the connector as a handle; every credential type has a test operation; OAuth scopes are minimal (CR).
8. Every write declares its safety level (`READ_ONLY`, `IDEMPOTENCY_KEY`, `RECONCILE`, `UNSAFE`); `UNSAFE` writes declare `manualCheck` (SF-1, SF-4).
9. Errors use the 14 codes; a server error, timeout or cut after sending a write without an idempotency key is `UNCERTAIN` and never retried blindly; a 429 is retried (ER-4). Enumerated values in manifests and package files are UPPER_SNAKE; identifiers and recorded tests keep their form (NM-13).
10. Every operation has a recorded success case marked as example; cassettes are scrubbed and pass the detector (TE-2 to TE-4).
11. Fault tests prove retries only when safe and UNCERTAIN for ambiguous writes (TE-6).
12. Every version bump matches the detected change, has change descriptors and a changelog entry in English and Spanish (VR).
13. Documentation is generated; the connection guide is hand-written in both languages (DC).
14. Icon: square SVG, no scripts or external references, at most 8 KiB, never n8n artwork (IC).
15. Map every replaced n8n type and typeVersion completely; never copy n8n code, text or images (N8).
16. Comments, docs, commit messages and pull request text never name build assistants or carry AI authorship trailers; AI-provider connectors and their pages live only under approved provenance paths; mneme may be named.
17. Edit this skill only under `.agents/skills/orcon-connector/` and re-run `scripts/project.sh`; never run `mneme skills pin` on it.

## Automated Checks

| Check | What it verifies | How to fix |
|---|---|---|
| `orcon connector lint --admission` naming | vocabulary verbs, singular resources, getMany parameters, id parameter, value casing through schema validation | Rename, write the value in UPPER_SNAKE, or record a reason in `authoring.json` |
| `orcon connector lint --admission` i18n | every string in both catalogs, English catalog equals the manifest | Add the missing keys |
| `orcon connector lint --admission` icon | square, no scripts or external references, at most 8 KiB | Clean the SVG |
| `orcon connector lint --admission` docs | both connection guides exist with their headings, every operation has an example | Write the guide; record an example cassette |
| `orcon connector lint --admission` kind | native connectors cite a known gap; gap threshold | Use declarative, cite the gap or open a backlog item |
| `orcon connector lint --admission` safety and errors | manualCheck on `UNSAFE` writes, error codes from the list | Add manualCheck; use a listed code |
| `orcon connector lint --admission` credentials | OAuth test operation, raw access and network reasons | Add the test operation or the reason |
| `orcon connector lint --admission` cassettes | no unscrubbed secret, e-mail, name, phone, card or IBAN | Re-record; add a reasoned allow entry only for non-personal values |
| `go test` `TestConformance` | the common conformance suite and replay of every cassette | Fix the manifest or the recording |
| `go test` `TestFaults` | the fault matrix for every operation | Fix the safety level or the error classification |
| `orcon connector diff --verify-bump` | declared version bump equals the detected change | Change the version or the change |
| `orcon connector docs --check` | generated pages are current | Run `orcon connector docs` and commit |
| `gitleaks` on connectors | independent secret scan of recorded tests | Re-record with the scrubber |
| `scripts/check-connector-templates.sh` | templates compile and pass admission | Fix the template |
| `make connectors-live` (nightly) | live services still match the recordings | Read the "Service changed" issue and update the connector |
| `make spdx`, `make provenance` | SPDX headers; no build-assistant names or AI trailers | Add the header; reword the text |
| `make skill-check` | this skill conforms and its generated copy is identical | Fix `SKILL.md`, then run `make skill-sync` |

## Verification

1. `make connectors-check` and `make connectors-version-check` exit 0 on your branch; CI runs the same in the `connectors` job (`sh scripts/verify.sh` runs both).
2. For one connector while iterating: `sh scripts/connector-change-check.sh <connector folder>`.
3. Third-party authors: `orcon connector lint --admission`, `orcon connector test --faults` and `orcon connector docs --check` pass on the package.
4. This skill: `sh scripts/project.sh --home && mneme skills lint orcon-connector && mneme skills validate orcon-connector` pass.
5. Report the commands you ran, their results and known limitations when you hand back.

## Workflow

1. Read the accepted ADR or spec that authorizes the connector or wave (specs live in mneme); stop if it is only Proposed.
2. Choose the kind (`references/kinds.md`); for native, write `authoring.json` with the gap first.
3. Copy `assets/templates/declarative/` or `assets/templates/native/` to `internal/adapter/connectors/<name>/` and rename the example service.
4. Model resources and operations with the vocabulary (`references/naming.md`); fill the manifest, permissions and credentials (`references/manifest.md`, `references/credentials-and-egress.md`).
5. Set the safety level of every write (`references/operation-safety.md`), pagination and rate limits (`references/pagination-and-rate-limits.md`) and error mapping (`references/errors.md`).
6. Add every string to `i18n/en.json` and `i18n/es.json`; add the icon (`references/icon.md`).
7. Record cassettes against a test account with `orcon connector record`; run the fault tests (`references/testing.md`); fill `tests/live.json`.
8. Write both connection guides and the n8n equivalence pages; map n8n types (`references/n8n-mapping.md`); run `orcon connector docs`.
9. For a new version: choose the bump from `references/versioning.md`, write change descriptors and the changelog in both languages.
10. While iterating run `sh scripts/connector-change-check.sh`; before handing back run `sh scripts/verify.sh`; paste `assets/pr-connector-checklist.md` into the pull request.
11. Write the commit message in Conventional Commits, describing the change only; citing mneme identifiers (`SPEC-…`, `BL-…`) is fine.
