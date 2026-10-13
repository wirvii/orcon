<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Checks

Status: Accepted

This file lists every automated check that enforces this rulebook: the tool, its licence, the command, where it runs, what it verifies and how to fix a failure. Rule IDs (`CK-…`) are stable. Precedence is in `kinds.md`.

- **CK-1** A rule that a machine can check is checked by one of the commands below; a reviewer never re-checks by hand what a command already verifies.
- **CK-2** Every check here runs the same way locally, in Orcon CI and in the automated stage of catalog review (CA-1). Third-party authors run the `orcon connector` commands; maintainers run the `make` targets.
- **CK-3** The tooling is delivered by the code slices of SPEC-011, which wait for the connector model of SPEC-003. Until a command exists, the rule it would check is applied in review.

## Lint

| Tool | Licence | Command | Runs | Verifies | How to fix |
|---|---|---|---|---|---|
| `orcon connector lint` (`internal/connector/authoring`) | Apache-2.0 | `orcon connector lint [--admission] <dir>`; `--all` for every official connector | locally, `make connectors-check`, CI job `connectors`, catalog admission | naming, manifest, credentials, safety, icon, i18n, guides, examples, kind justification, cassette scrubbing | fix the file named in `path:rule:message`; the rule id points to the rule in this rulebook |

Without `--admission` admission findings are warnings; with it they are errors. Manifest schema validation runs first and rejects any enumerated value that is not UPPER_SNAKE (NM-13), naming the expected value.

Planted cases in `internal/connector/authoring/testdata/bad/<case>/` (each must report exactly its rule), plus `testdata/good/` with no findings:

| # | Rule | Planted case |
|---|---|---|
| 1 | NM-4, NM-5 verb outside the vocabulary without `verbExceptions[]` | `verb-not-in-vocabulary` |
| 2 | NM-3 plural resource without exception | `resource-plural` |
| 3 | NM-8 `getMany` without `returnAll` and `limit` | `getmany-no-returnall` |
| 4 | NM-7 id not named `<resource>Id` or not `RESOURCE_LOCATOR` | `id-param-name` |
| 5 | NM-12 missing `i18n/es.json` or key without Spanish text | `missing-es-entry` |
| 6 | NM-12 `i18n/en.json` differs from the manifest | `en-catalog-mismatch` |
| 7 | IC-3 icon with `<script>`, `on*` or `<foreignObject>` | `icon-script` |
| 8 | IC-3 icon with an external reference | `icon-external-ref` |
| 9 | IC-3 icon larger than 8192 bytes | `icon-too-large` |
| 10 | IC-2 icon not square | `icon-not-square` |
| 11 | DC-4 missing `docs/connection-guide.en.md` or a required heading | `missing-guide-en` |
| 12 | DC-4 missing `docs/connection-guide.es.md` or a required heading | `missing-guide-es` |
| 13 | DC-3 operation without an `example: true` cassette | `operation-no-example` |
| 14 | SF-4 `UNSAFE` without `manualCheck` | `unsafe-no-manualcheck` |
| 15 | KD-3 native without `kind.gap` | `native-no-justification` |
| 16 | KD-3 native citing an unknown gap | `native-unknown-gap` |
| 17 | KD-5 fourth official native for one gap without `kind.extensionBacklog` | `native-gap-threshold` |
| 18 | CR-3 `credentialRawAccess` without `rawAccessReason` | `raw-access-unjustified` |
| 19 | MF-6 invalid domain wildcard | `domain-wildcard` |
| 20 | MF-7 `http` or `allowPrivateNetworks` without `fromCredential` and `networkReason` | `http-scheme` |
| 21 | ER-2 error code outside the list | `error-code-unknown` |
| 22 | VR-6 version without changelog in English and Spanish | `missing-changelog` |
| 23 | TE-4 cassette with an unscrubbed e-mail address | `cassette-email` |
| 24 | TE-4 cassette with a bearer token | `cassette-bearer` |
| 25 | TE-4 cassette with a name that is not a pseudonym | `cassette-name` |
| 26 | CR-6 `OAUTH2` credential without `test` | `oauth-no-test` |

Planted case names are folder identifiers and do not change. `TestLintFixtures` requires the expected rule per case; `TestVocabularyMatchesRulebook` compares the verbs of `naming.md` (NM-4) and the codes of `errors.md` (ER-2) with `Verbs` and `ErrorCodes` in `internal/connector/authoring/vocabulary.go`.

The declared exception for `internal/adapter/connectortest` (TE-10) has its own planted violation: a non-test file in `internal/adapter/connectors/zzfixture` that imports it must fail `make lint-fixtures`.

## Recorded tests

| Tool | Licence | Command | Runs | Verifies | How to fix |
|---|---|---|---|---|---|
| Cassette recorder and replayer (`internal/adapter/connectortest/cassette`) | Apache-2.0 | `orcon connector record <dir> --operation <op> --params <file> --case <name> [--example]`, or `ORCON_RECORD=1 go test`; replay in `go test` | recording: locally against a test account; replay: locally, `make connectors-check`, CI | replay without network; requests match on method, path, sorted query and canonical JSON body; host-applied values are checked separately | re-record the case (TE-3); an unmatched request prints a diff |
| Scrubber (`connectortest/scrub`) | Apache-2.0 | runs inside `record`; `orcon connector scrub <file>` re-scrubs | before every write of a cassette | credential values, sensitive headers and keys, tokens, e-mail addresses, personal names, phone numbers, cards and IBANs replaced by placeholders | add personal-data paths to `tests/scrub.json`; never edit the placeholders |
| Detector (`connectortest/detect`) | Apache-2.0 | runs inside `record`, replay and `lint --admission` | locally, CI, catalog admission | no secret or personal data is left (TE-4) | re-record or re-scrub; a justified exception goes to `tests/scrub-allow.json` with a reason |

## Fault tests

| Tool | Licence | Command | Runs | Verifies | How to fix |
|---|---|---|---|---|---|
| Fault injector (`connectortest.Faults`) | Apache-2.0 | `TestFaults` in `go test`; `orcon connector test --faults <dir>` | locally, `make connectors-check`, CI, catalog admission | the TE-6 matrix per operation: sends, same key on resends, verification, final state, exactly one `execution.awaiting_human` and zero resends for UNCERTAIN | fix the safety level (SF-1) or the error classes (ER-3, ER-4) |

## Conformance

| Tool | Licence | Command | Runs | Verifies | How to fix |
|---|---|---|---|---|---|
| Conformance suite (`conformancetest`, SPEC-003) extended with lint, replay and detection | Apache-2.0 | `TestConformance` in `go test` | locally, `make connectors-check`, CI, catalog admission | the common contract of ADR-0031 §5, FI-01 to FI-03 against the mock provider, `lint --admission`, replay of every cassette, detector | follow the failing assertion; it names the rule |

## Versioning

| Tool | Licence | Command | Runs | Verifies | How to fix |
|---|---|---|---|---|---|
| Change classifier (`internal/connector/changeclass`) | Apache-2.0 | `orcon connector diff --classify`; `orcon connector diff --verify-bump --base <ref> --changed <dir>`; `make connectors-version-check BASE=origin/main` | locally, CI job `connectors`, catalog admission | the declared bump matches the detected one in both directions (VR-3, VR-4); change descriptor and changelog in English and Spanish exist (VR-5, VR-6) | change the version to the printed `PATCH`, `MINOR` or `MAJOR`, add the descriptor and the changelog entries |

## Documentation

| Tool | Licence | Command | Runs | Verifies | How to fix |
|---|---|---|---|---|---|
| Documentation generator (`internal/connector/docgen`) | Apache-2.0 | `orcon connector docs [--all] <dir>`; `orcon connector docs --check` | locally, `make connectors-check`, CI, catalog admission | committed pages have no drift; admission would pass (guide and example present, DC-6); AI-provider pages only under `docs/reference/connectors/ai/` (DC-7) | run `orcon connector docs` and commit the result; never edit generated pages |

## Secret scan

| Tool | Licence | Command | Runs | Verifies | How to fix |
|---|---|---|---|---|---|
| gitleaks command-line tool, version pinned in `tools/versions.mk` (not the GitHub action) | MIT | `.bin/gitleaks dir --config .gitleaks-connectors.toml --redact --exit-code 1 internal/adapter/connectors` | `make connectors-check`, CI | an independent second scan of the committed cassettes | re-record or re-scrub the cassette; never weaken the configuration to pass |

## Templates

| Tool | Licence | Command | Runs | Verifies | How to fix |
|---|---|---|---|---|---|
| Template test | Apache-2.0 | `sh scripts/check-connector-templates.sh` | `make connectors-check`, CI | each template in `assets/templates/` becomes a temporary connector that passes schema validation with its UPPER_SNAKE values, `go test` and `lint --admission` | fix the template, not the check |

## Live tests

| Tool | Licence | Command | Runs | Verifies | How to fix |
|---|---|---|---|---|---|
| Live runner (`connectortest.Live`) and the nightly workflow `connectors-live.yml` | Apache-2.0 (runner), MIT (GitHub CLI) | `make connectors-live CONNECTOR=<name>` (`ORCON_LIVE=1`) | nightly on `main` and on manual dispatch only; never for pull requests; never required | the real service still answers like the example cassette (removed key or changed type = shape drift) | read the "Service changed: <connectorId>" issue, re-record, release a new version |

Secret policy: test accounts owned by Wirvii, never personal; one per provider; least privilege; secrets named `ORCON_LIVE_<CONNECTOR>_<FIELD>` in the `connectors-live` environment; rotated every 90 days; OAuth refreshes kept in memory only; every secret masked in logs; writes only in test mode with `cleanup`; the inventory lives in `docs/engineering/connectors.md` without values.

## Inherited checks

These checks belong to slices of SPEC-003 and to every connector wave; they are recorded in those specifications:

- **H-CN3-1** `conformancetest.Run` includes the lint, replay and detection sections.
- **H-CN3-2** The declarative interpreter accepts `class: UNCERTAIN` only on operations whose purity is `EFFECT` and applies ER-4.
- **H-CN4-1** The native template compiles against the real SDK and enters `check-connector-templates.sh`.
- **H-CN4-2** PostgreSQL and SMTP cite `kind.gap` DG-1 and pass `lint --admission` and `TestFaults`.
- **H-CN7-1** The manifest diff feeds `changeclass`; change descriptors support `addNode`, `addOperation` and `addCredentialType`.
- **H-CN12-1** The automated stage of the catalog runs `lint --admission`, `docs --check`, the detector and `diff --verify-bump`, rejecting with the same rule ids.
- **H-WAVE-1** Each connector wave passes `connectors-check` and `connectors-version-check`.
- **H-WAVE-2** Each connector has `tests/live.json` with a test account when one exists, otherwise `reasonUnavailable`.
- **H-WAVE-3** Each connector has English and Spanish pages and a connection guide.

## mneme

Every connector wave is its own specification tracked in mneme. After editing this skill, run `make skill-sync` and validate it with `mneme skills lint orcon-connector` and `mneme skills validate orcon-connector`. Never run `mneme skills pin` on it.
