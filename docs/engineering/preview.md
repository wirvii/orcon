<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Preview deployment

Status: Informational

Every merge to `main` deploys a preview of Orcon to Cloud Run so maintainers can look at the current state of the product. The preview is not a release, not a demo for the public and not a supported way to run Orcon.

## What it is

- One Cloud Run service, `orcon-preview`, that runs the `standalone` role of the binary built from the commit on `main` (`build/preview/Dockerfile`). The image embeds the web editor.
- It is reachable at `https://orcon-preview.wirvii.com`.
- It scales to zero and runs at most one instance, so the first request after a quiet period is slow.

## It is protected

The preview is not public. The service accepts traffic only from its load balancer, and the load balancer requires Google Identity-Aware Proxy (IAP) sign-in. Only the Google accounts that maintainers explicitly authorize can open it. Anonymous requests are redirected to the Google sign-in page or rejected.

## It is ephemeral

Data does not survive a restart or a new deployment. Anything the preview writes goes to the container's temporary disk, and instances are replaced on every merge and whenever Cloud Run decides. Never put real data or real credentials in it.

## How it is deployed

The `Preview` workflow (`.github/workflows/preview.yml`) runs on every push to `main` and on manual dispatch, never for pull requests. It builds the image, pushes it to Artifact Registry and deploys a new revision of `orcon-preview`. It authenticates with Workload Identity Federation: Google Cloud only accepts tokens issued for this repository on `refs/heads/main`, and the deployer identity can only write to the image repository and deploy that one service. No secret is stored in GitHub. The non-secret identifiers (project, region, provider and service account names) are repository variables prefixed with `PREVIEW_`.

## Running the image locally

```sh
docker build -f build/preview/Dockerfile -t orcon-preview .
docker run --rm -p 8080:8080 orcon-preview
```
