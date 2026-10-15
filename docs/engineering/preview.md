<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Preview deployment

Status: Informational

Every merge to `main` deploys a preview of Orcon to Cloud Run so maintainers can look at the current state of the product. The preview is not a release, not a demo for the public and not a supported way to run Orcon.

## What it is

- One Cloud Run service, `orcon-preview`, that runs the `standalone` role of the binary built from the commit on `main` (`build/preview/Dockerfile`), started as `orcon serve --role=standalone --http-address=0.0.0.0:8080`. The image embeds the web editor.
- It is reachable at `https://orcon-preview.wirvii.com`.
- It scales to zero and runs at most one instance, so the first request after a quiet period is slow.

## It is protected

The preview is not public. The service accepts traffic only from its load balancer, and the load balancer requires Google Identity-Aware Proxy (IAP) sign-in. Only the Google accounts that maintainers explicitly authorize can open it. Anonymous requests are redirected to the Google sign-in page or rejected.

## Data

The preview keeps its data in the database `orcon_preview` on the shared Cloud SQL instance `wirvii-shared-pg` (PostgreSQL 16, private address only), under its own login `orcon_preview`. That instance also hosts databases of other products: the preview only ever connects to its own database, and its pool is capped at five connections (`ORCON_STORE_POSTGRES_MAX_CONNECTIONS=5`). Data survives restarts and new deployments, but the preview is still not a place for real data or real credentials.

The service reaches the instance through Direct VPC egress (network `shared-vpc`, subnet `shared-usc1`, private ranges only). Three Secret Manager secrets feed it, readable by the runtime identity and listed by the deployer identity: `orcon-preview-db-dsn`, `orcon-preview-master-key` (the `env` master key provider) and `orcon-preview-master-key-ack` (the master key fingerprint that completes the first start). The migrations create the cluster roles `orcon_owner`, `orcon_application` and `orcon_engine`. Cloud SQL does not give the login a superuser, so a one-time grant inside the preview database is needed: the login becomes a member (with `SET`) of the three roles, and `orcon_engine` receives `CREATE` on the `orcon_engine` schema so the engine functions can be handed to it.

## How it is deployed

The `Preview` workflow (`.github/workflows/preview.yml`) runs on every push to `main` and on manual dispatch, never for pull requests. It builds the image, pushes it to Artifact Registry and deploys a new revision of `orcon-preview`. It authenticates with Workload Identity Federation: Google Cloud only accepts tokens issued for this repository on `refs/heads/main`, and the deployer identity can only write to the image repository and deploy that one service. No secret is stored in GitHub. The non-secret identifiers (project, region, provider and service account names) are repository variables prefixed with `PREVIEW_`.

## Running the image locally

```sh
docker build -f build/preview/Dockerfile -t orcon-preview .
docker run --rm -p 8080:8080 orcon-preview
```
