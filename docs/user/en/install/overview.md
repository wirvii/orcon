---
title: Install overview
description: The four official ways to install Orcon, what they have in common and how to choose one.
sources:
  - ADR-0083
  - ADR-0002
  - ADR-0081
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Install overview

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers the installers, the image, the compose examples and the Helm chart removes this note.

Orcon is one executable that can run in several roles (`standalone`, `api`, `scheduler` and `worker`). There are four official ways to install it. All of them install the same program, and every release is tested on every one of them before it is published.

## Choose a path

| Path | Database | Roles | Choose it when |
| --- | --- | --- | --- |
| [Single binary](binary.md) | SQLite (built in) | `standalone` | You want one machine and no other service. This is the path of the [quickstart](../getting-started/quickstart.md). |
| [Container image](container.md) | SQLite by default; PostgreSQL by configuration | `standalone` by default | You already run containers and want one process with a data volume. |
| [Docker Compose](docker-compose.md) | SQLite, or PostgreSQL with two workers | `standalone`, or `api`, `scheduler` and two `worker` | You want a ready example of a single service, or of a small distributed setup, to adapt. |
| [Kubernetes with Helm](kubernetes-helm.md) | PostgreSQL only | `api`, `scheduler` and `worker` in separate deployments | You run Kubernetes and want to scale the roles separately. |

SQLite works only with the `standalone` role. As soon as you want more than one server, or high availability, use PostgreSQL.

## What every path has in common

- **Secure defaults.** Orcon listens on the loopback address, and metrics, exporters and the update check are off. You opt in to everything else. See [Configuration overview](../configuration/overview.md).
- **Verified downloads.** The one-line installers verify the signature of the checksum file and the SHA-256 of the archive, and have no option to skip the check. The image and the Helm chart are signed too. See [Verifying releases](verifying-releases.md).
- **Same first run.** Until setup is complete, the server accepts only the setup operations. You create the master key and the owner account either in the browser or with commands. See [First run](../configuration/first-run.md).
- **No contact with the project.** Orcon sends no usage data and no installation identifier. See [Network connections](../configuration/network-connections.md).

## What every release proves

Before a release is published, an install-and-run test installs it from scratch on each path, completes the first run without the web interface, runs a sample flow and checks that it succeeds. The same test runs again against the published assets, and it also upgrades from the previous release. On the PostgreSQL paths the test checks that executions are spread across at least two workers.

## Supported platforms

The platforms, architectures and versions that are tested are in the [support matrix](../support/matrix.md). Windows is published for evaluation and is not recommended for production.

## Not provided in version 1

Running Orcon as a Windows service, packages for Linux distributions or Homebrew, and a Kubernetes operator are not part of the first version.

## Next steps

1. Install with one of the paths above.
2. Complete the [first run](../configuration/first-run.md).
3. Read the [configuration overview](../configuration/overview.md) and the [hardening checklist](../security/hardening-checklist.md).
