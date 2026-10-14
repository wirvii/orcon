---
title: Install on Kubernetes with Helm
description: Deploy the api, scheduler and worker roles on Kubernetes with the official Helm chart and PostgreSQL.
sources:
  - ADR-0083
  - ADR-0002
  - ADR-0042
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Install on Kubernetes with Helm

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers the Helm chart removes this note.

The Helm chart deploys the `api`, `scheduler` and `worker` roles as three separate deployments from the same image. It works with PostgreSQL only. You need a Kubernetes version and a Helm version listed in the [support matrix](../support/matrix.md): Helm 3.14 or newer, or Helm 4.

## Where the chart is

The chart is published with every release as an OCI artifact at `oci://ghcr.io/wirvii/charts/orcon`. Its version is the same as the Orcon version, and it is signed with cosign without keys, like the image. See [Verifying releases](verifying-releases.md).

## Secrets are never values

The chart does not accept secret values in `values.yaml`, because values stay in the Helm release history. You create Kubernetes Secrets yourself and the chart refers to them by name (`existingSecret`). They are mounted as files in `/run/secrets/orcon/` and passed to Orcon through the `*_FILE` variants of the configuration. The settings that are not secret are rendered into a ConfigMap mounted at `/etc/orcon/config.yaml`.

## The first run is headless

A chart install needs the headless first run, so it cannot depend on a person opening a browser. `helm install` fails when the values schema finds that these are missing, and the message points to `orcon key init`:

- `masterKey.existingSecret`, a Secret that holds the key under the name `master.key` (for the `file` provider).
- `masterKey.backupAcknowledgement`, the fingerprint printed by `orcon key init`.
- `setup.owner.existingSecret`, a Secret with the keys `email`, `name` and `password` of the owner account.

Run `orcon key init` on a trusted machine first, keep the recovery kit somewhere safe, and create the Secrets from the key it wrote. See [First run](../configuration/first-run.md) and [Master key and KMS](../configuration/master-key-and-kms.md).

## Install

Write a values file with at least the required values and install:

```yaml
publicUrl: https://orcon.example.com
postgres:
  existingSecret: orcon-postgres
masterKey:
  existingSecret: orcon-master-key
  backupAcknowledgement: <fingerprint>
setup:
  owner:
    existingSecret: orcon-owner
```

```sh
helm install orcon oci://ghcr.io/wirvii/charts/orcon --version <version> --values values.yaml
```

`<version>` is the Orcon release. `<fingerprint>` is the fingerprint printed by `orcon key init`. The names after `existingSecret` are the names of your own Secrets. The `postgres.existingSecret` Secret holds the connection string under the key `dsn` by default (`postgres.dsnKey`).

## What the chart creates

- Three deployments, `api`, `scheduler` and `worker`, started with `serve --role=<role>`. The defaults are 2 replicas each.
- A Service for `api` on port 8080, and a headless Service for the operations listener of each role on port 9464.
- An optional Ingress or HTTPRoute (off by default), an optional NetworkPolicy (off by default), and an optional HorizontalPodAutoscaler for `worker` on CPU (off by default).
- A migration job that runs `orcon store schema apply` before each install and upgrade. The replicas start with automatic migration at startup turned off.
- Probes on the operations port: a startup probe and a liveness probe on `/healthz`, and a readiness probe on `/readyz`.
- Pod disruption budgets: `api` keeps at least 1 pod available only when it has 2 or more replicas, `scheduler` allows 1 unavailable, and `worker` allows 25 percent. A budget never blocks draining a node that runs a single replica.
- A termination grace period of 45 seconds, which is the shutdown timeout plus 15 seconds.

The chart sets the operations address on every replica and turns Prometheus metrics on. Creating a `ServiceMonitor` and a `PrometheusRule` is available and off by default. They hold the same content as the monitoring kit.

## Security settings

Pods meet the `restricted` Pod Security profile of Kubernetes: they run as non-root user and group `65532`, use the `RuntimeDefault` seccomp profile, drop all capabilities, forbid privilege escalation and use a read-only root file system, with an empty directory at `/tmp`. The service account token is not mounted unless you set `serviceAccount.automountToken: true`, which you do only to use a cloud identity for a key management service or for S3.

## Scale the workers

Set `worker.replicas`, or turn on `worker.autoscaling`. Scaling by the number of runnable tokens, for example with KEDA, is possible; the metric is `orcon_tokens_runnable`, and the chart does not include KEDA.

## Upgrade and roll back

Upgrade with `helm upgrade`. The migration job checks the compatibility floor first. When the verdict is `REQUIRES_FULL_STOP`, the job fails with the reason and the upgrade stops without touching the deployments. In that case scale the deployments to 0, set `migrations.fullStop: true` and upgrade again.

`helm rollback` works when the previous version is at or above the effective compatibility floor. Otherwise restore the backup you took before the upgrade. See [Upgrade and rollback](../operations/upgrade-and-rollback.md).

## Test the installation

`helm test` runs `orcon health --ready` against the `api` Service.
