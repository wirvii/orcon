<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

## Verify this release

Every archive, the image and the tag of this release can be verified without trusting the project's infrastructure. The full recipe is in [docs/releases/verifying.md](https://github.com/wirvii/orcon/blob/v{{VERSION}}/docs/releases/verifying.md).

```sh
VERSION={{VERSION}}
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/wirvii/orcon/.github/workflows/release.yml@refs/tags/v${VERSION}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --ignore-missing -c checksums.txt
slsa-verifier verify-artifact "orcon_${VERSION}_linux_amd64.tar.gz" \
  --provenance-path "orcon_${VERSION}.intoto.jsonl" \
  --source-uri github.com/wirvii/orcon --source-tag "v${VERSION}"
```
