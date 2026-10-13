<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR 0081: Contribution workflow and signed releases

Status: Proposed

## Context

Orcon is an Apache-2.0 project meant for community adoption. Until now it had no written rules for branches, commit messages, pull requests, review or releases, and nothing prevented a hand-made, unsigned release. Users need to verify that a binary or image was built by this repository's release workflow from a reviewed commit. Some GitHub behaviors constrain the design: an author can never approve their own pull request; events created with the default `GITHUB_TOKEN` do not start other workflows; release-please creates lightweight, unsigned tags through the REST API; and the SLSA trusted builders must be referenced by a version tag.

## Decision

1. Trunk-based development. Short-lived branches named `<type>/<slug>` reach `main` only through pull requests that pass every required check. `main` is protected by a ruleset with an empty bypass list: no direct pushes, no force pushes, no deletion, linear history, squash merge only. The pull request title becomes the single commit on `main`; the squash message body is empty. Required approvals are zero while the owner is the only maintainer and become one, with code owners, as soon as a second maintainer with write access exists.
2. Conventional Commits with exactly one scope from a closed list. Product scopes: engine, trigger, connector, api, cli, mcp, web, ui, db, ai, security, notify. Internal scopes: docs, ci, deps, tooling. A new scope requires an amendment of this ADR. commitlint (MIT) enforces the format on pull request titles and on every commit of a pull request in a required CI job, and on outgoing commits in a local pre-push hook.
3. Pull requests follow a template (what and why, the mneme spec id when the work is tracked there, how it was tested, screenshots for UI changes, size justification, breaking changes, the checklists of the skills that apply). A soft limit of 400 changed lines excludes paths marked in `.gitattributes`; going over it needs a written justification, and an advisory CI job warns.
4. Releases use semantic versioning. release-please (Apache-2.0) runs in pull-request-only mode and keeps one release pull request with the changelog, grouped by change type with each entry's scope in bold. Merging it is the decision to publish. A workflow job then creates an annotated tag signed with Sigstore gitsign (Apache-2.0) using the workflow's OpenID Connect identity, and pushes it with the token of a dedicated GitHub App, which is the only actor allowed to create `v*` tags.
5. The tag starts the release workflow. It verifies the tag signature, then GoReleaser (MIT) builds `orcon` for linux, darwin and windows on amd64 and arm64, archives with checksums, the CycloneDX SBOM of ADR 0076's toolchain, and a multi-architecture image on `ghcr.io/wirvii/orcon` based on a digest-pinned distroless static image.
6. Every artifact is verifiable without trusting the project's infrastructure: Sigstore cosign (Apache-2.0) keyless signatures on the checksum file and the image, a CycloneDX attestation on the image, and SLSA v1 Build Level 3 provenance for the archives and the image from slsa-github-generator (Apache-2.0), checked with slsa-verifier (Apache-2.0). A final job verifies the published release with the same commands users run (`docs/releases/verifying.md`).
7. Actions are pinned by commit SHA, except the two slsa-github-generator reusable workflows, which are pinned to an exact version tag because the verifier requires it.
8. No long-lived signing key exists. The only release secret is the GitHub App credential, stored in a `release` environment limited to `main` and `v*` tags.
9. Vulnerabilities are reported through GitHub private vulnerability reporting (`SECURITY.md`). The Code of Conduct is the Contributor Covenant 2.1, with info@wirvii.com as the contact.
10. The detailed rules live in the `orcon-contributing` skill (`.agents/skills/orcon-contributing/references/`); `CONTRIBUTING.md` summarizes them.

## Consequences

- Every change on `main` is a reviewed, linted, single commit, and release notes are generated from those titles.
- Publishing is a merge, not a manual procedure; a published tag or asset is never moved or replaced, and mistakes are fixed forward with a new patch release.
- Contributors must write scoped commit messages and may need to reword commits before a pull request can merge.
- The release pipeline depends on GitHub Actions, the GitHub App, Sigstore's public infrastructure and the GitHub container registry.
- The container image aggregates distroless and Debian packages under their own licenses; they are not linked into Orcon.
- While the owner is the only maintainer, no second human approval is enforced; the required checks, resolved conversations and the QA record carry the review. When a second maintainer with write access joins, the owner adds `.github/CODEOWNERS` and raises required approvals to one in the same change.
- A reader looking for every change in one area finds it spread across the type sections of the release notes, each line labeled with its scope.

## Alternatives considered

- semantic-release (MIT): publishes on every merge, which removes the owner's decision of when to publish.
- changesets (MIT): designed around npm packages.
- GPG-signed tags: need a long-lived private key in repository secrets.
- GitHub artifact attestations (`actions/attest-build-provenance`, MIT): simpler, but Build Level 2 by default and verification centered on GitHub's API; kept as a fallback.
- ko (Apache-2.0) for images: does not produce the archives and signed checksums.
- A project-specific commit format checker: commitlint already provides the parser and rules.
- Release notes regrouped by scope with a custom script: more code to maintain and a changelog format that differs from the standard one.
- One required approval from the start: with a single maintainer, who cannot approve their own pull requests, it would block every change.
