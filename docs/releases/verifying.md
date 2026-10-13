<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Verifying an Orcon release

Status: Accepted

Every Orcon release can be checked without trusting the project's servers or its maintainers. The release workflow signs each artifact with a short-lived certificate tied to the workflow's identity, and publishes build provenance that says which repository, commit and workflow produced it. This page shows how to check a release yourself. The decision behind it is ADR 0081.

In the commands below, replace `0.1.0` with the version you downloaded.

## What is signed

| Artifact | How it is protected | How you check it |
|---|---|---|
| Release tag `vX.Y.Z` | Annotated tag signed with gitsign by the release workflow | `gitsign verify-tag` |
| `checksums.txt` | Keyless cosign signature bundle `checksums.txt.sigstore.json`; lists the SHA-256 of every archive and of the SBOM | `cosign verify-blob`, then `sha256sum -c` |
| Archives `orcon_X.Y.Z_<os>_<arch>.tar.gz` (`.zip` on Windows) | Covered by `checksums.txt`, plus SLSA v1 Build Level 3 provenance `orcon_X.Y.Z.intoto.jsonl` | `slsa-verifier verify-artifact` |
| SBOM (CycloneDX, Go modules) | Listed in `checksums.txt`; attached to the image as a signed attestation | `sha256sum -c`, `cosign verify-attestation` |
| Image `ghcr.io/wirvii/orcon:X.Y.Z` (linux/amd64, linux/arm64) | Keyless cosign signature on the image digest, CycloneDX attestation, SLSA v1 Build Level 3 provenance | `cosign verify`, `slsa-verifier verify-image` |

No long-lived signing key exists: there is no public key to download. Each signature is checked against the identity of the GitHub Actions workflow that made it and the GitHub OpenID Connect issuer, recorded in Sigstore's public transparency log.

## Tools

All three tools are Apache-2.0 open source projects:

- cosign — <https://docs.sigstore.dev/cosign/system_config/installation/>
- slsa-verifier — <https://github.com/slsa-framework/slsa-verifier#installation>
- gitsign — <https://docs.sigstore.dev/cosign/signing/gitsign/> (only needed to check the tag)

## Verify a downloaded archive

Download the archive for your platform, `checksums.txt`, `checksums.txt.sigstore.json` and `orcon_X.Y.Z.intoto.jsonl` from the release page into one directory, then run:

```sh
VERSION=0.1.0
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/wirvii/orcon/.github/workflows/release.yml@refs/tags/v${VERSION}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --ignore-missing -c checksums.txt
slsa-verifier verify-artifact "orcon_${VERSION}_linux_amd64.tar.gz" \
  --provenance-path "orcon_${VERSION}.intoto.jsonl" \
  --source-uri github.com/wirvii/orcon --source-tag "v${VERSION}"
```

1. `cosign verify-blob` proves that `checksums.txt` was signed by Orcon's release workflow for that tag.
2. `sha256sum` proves that your archive matches the signed checksum. On macOS use `shasum -a 256 --ignore-missing -c checksums.txt`.
3. `slsa-verifier verify-artifact` proves that the archive was built by the trusted builder from `wirvii/orcon` at that tag. Replace `linux_amd64` with your platform.

Each command must end without an error. If any of them fails, do not use the file.

## Verify the container image

```sh
VERSION=0.1.0
IDENTITY="https://github.com/wirvii/orcon/.github/workflows/release.yml@refs/tags/v${VERSION}"
ISSUER=https://token.actions.githubusercontent.com

cosign verify "ghcr.io/wirvii/orcon:${VERSION}" \
  --certificate-identity "${IDENTITY}" \
  --certificate-oidc-issuer "${ISSUER}"
cosign verify-attestation --type cyclonedx "ghcr.io/wirvii/orcon:${VERSION}" \
  --certificate-identity "${IDENTITY}" \
  --certificate-oidc-issuer "${ISSUER}"
```

`cosign verify` prints the image digest (`sha256:…`). Use that digest, not the tag, to check the provenance and to pull the image:

```sh
slsa-verifier verify-image "ghcr.io/wirvii/orcon@sha256:<digest>" \
  --source-uri github.com/wirvii/orcon --source-tag "v${VERSION}"
```

## Verify the tag

```sh
VERSION=0.1.0
git fetch --tags
gitsign verify-tag "v${VERSION}" \
  --certificate-identity https://github.com/wirvii/orcon/.github/workflows/release-please.yml@refs/heads/main \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

GitHub does not show Sigstore signatures as "Verified" in its web interface. That label is not part of this check: the command above is.

## Read the SBOM

The SBOM is a CycloneDX JSON file listing the Go modules compiled into `orcon`, with their versions and licenses. After checking it against `checksums.txt`, open it with any CycloneDX tool, or extract the same document from the image attestation:

```sh
cosign verify-attestation --type cyclonedx "ghcr.io/wirvii/orcon:${VERSION}" \
  --certificate-identity "${IDENTITY}" \
  --certificate-oidc-issuer "${ISSUER}" \
  | jq -r '.payload' | base64 -d | jq '.predicate'
```

## Report a problem

If a check fails for an official release, or you find a signature that should not exist, do not open a public issue. Report it privately as described in [SECURITY.md](../../SECURITY.md).
