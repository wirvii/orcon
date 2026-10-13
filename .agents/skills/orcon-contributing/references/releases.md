<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Releases

Status: Proposed — pending owner approval

This document defines how Orcon versions are chosen, and how a release is decided, tagged, built, signed and verified, and what happens when a release fails. It is part of the `orcon-contributing` rulebook; the decision behind it is ADR 0081. Rule IDs (`RL-…`) are stable. The check that enforces a rule, when one exists, is named in brackets. Precedence: accepted ADRs (ADR 0081) > accepted `orcon-go` rules > this rulebook > reviewer preference.

## Rules

- **RL-1** Semantic versioning with tags `vMAJOR.MINOR.PATCH`. Before 1.0 (`bump-minor-pre-major`): breaking → minor, `feat` → minor, `fix`/`perf` → patch.
- **RL-2** release-please (Apache-2.0) keeps one release pull request `chore(tooling): release X.Y.Z` with `CHANGELOG.md`. Notes are grouped by change type (Features, Bug Fixes, Performance, Reverts, Documentation) and each entry starts with its scope in bold (owner decision, 2026-10-12); no custom regrouping.
- **RL-3** Publishing is the owner's decision, made by merging the release pull request after reviewing it with `assets/templates/release-pr-review.md`.
- **RL-4** The `tag` job creates an annotated tag signed with gitsign (keyless; identity `https://github.com/wirvii/orcon/.github/workflows/release-please.yml@refs/heads/main`, issuer `https://token.actions.githubusercontent.com`) and pushes it with the release GitHub App, the only actor the `release-tags` ruleset allows. [`verify-tag`]
- **RL-5** Nobody creates, moves or deletes a `v*` tag by hand, and published assets are never edited or replaced. A bad release is fixed forward with a new patch release, noted in its release notes; a security problem also gets an advisory.
- **RL-6** Artifacts: `orcon` for linux, darwin and windows on amd64 and arm64 (`CGO_ENABLED=0`, `-trimpath`, version and commit injected); `tar.gz` archives (`zip` on windows) with `LICENSE` and `README.md` (and `NOTICE` once it exists); `checksums.txt` (SHA-256, including the SBOM); the CycloneDX SBOM from `make sbom`; image `ghcr.io/wirvii/orcon:X.Y.Z` for linux/amd64 and linux/arm64 on a digest-pinned distroless static nonroot base; `latest` only for non-prerelease versions.
- **RL-7** Signatures and provenance: cosign keyless bundle for `checksums.txt`; cosign signature of the image digest; cosign CycloneDX attestation on the image; SLSA v1 Build Level 3 provenance from slsa-github-generator for the archives (via the checksums) and for the image.
- **RL-8** The `verify` job runs `scripts/release/verify.sh` on the published release; users follow `docs/releases/verifying.md`.
- **RL-9** No release branches or backports before 1.0; a later ADR defines maintenance branches.
- **RL-10** `make release-prepare VERSION=vX.Y.Z` runs on the release pull request branch after every refresh; skills that generate per-release files hook into it (the API changelog of `orcon-api`).
- **RL-11** Release notes are the version's `CHANGELOG.md` section plus `build/release/notes-footer.md` (`scripts/release/notes.sh`).
- **RL-12** Tool versions and checksums are pinned in `tools/versions.mk`; actions are pinned by SHA except the two slsa-github-generator reusable workflows, pinned by exact version tag (ADR 0081 §7).
- **RL-13** The only release secret is the GitHub App credential in the `release` environment (limited to `main` and `v*`). There are no signing keys.
- **RL-14** A failed release workflow: re-run the failed jobs. If a release cannot be completed, mark it as a prerelease with the note `incomplete — do not use` and publish the next patch.
