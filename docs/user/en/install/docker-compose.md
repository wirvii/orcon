---
title: Install with Docker Compose
description: Start from the official compose examples for a single SQLite service or for PostgreSQL with separate roles.
sources:
  - ADR-0083
  - ADR-0002
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Install with Docker Compose

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers the compose examples removes this note.

Orcon provides two compose examples. They are starting points to read and adapt, not a managed product. You need Docker Engine 24 or newer with Compose v2.

## The two examples

| Example | File in the repository | What it starts |
| --- | --- | --- |
| SQLite | `deploy/compose/sqlite/compose.yaml` | One `standalone` service, a data volume, and the configuration and secrets mounted as files. |
| PostgreSQL | `deploy/compose/postgres/compose.yaml` | PostgreSQL pinned by digest, a one-time migration service, one `api`, one `scheduler` and two `worker` services. Secrets use the `secrets:` feature of compose and are read through the `*_FILE` variants of the configuration. |

## Get the files

Each release publishes both examples as `orcon-compose_<version>.tar.gz`. The archive includes a `.env` file that sets `ORCON_VERSION` to that release.

In the repository the examples require you to set `ORCON_VERSION` yourself. There is no `latest` default, so a start never changes version by accident.

## Start

```sh
docker compose up -d
```

Run the command in the directory of the example you chose. Then complete the [first run](../configuration/first-run.md). With PostgreSQL, the first run state and the setup token hash live in the database, so it does not matter which `api` replica answers.

## Check

Ask a process whether it is ready with `orcon health --ready`, run inside its container. To list the processes that are alive, with their version and role, run `orcon store schema status`.

## Things to know

- The PostgreSQL example runs two workers, so work is spread across processes.
- The `api`, `scheduler` and `worker` services need the same blob storage. With PostgreSQL the default is to keep blobs in the database; a file system backend would need a path shared by every process, and `orcon config check` warns about it.
- To upgrade, follow [Upgrade and rollback](../operations/upgrade-and-rollback.md): change `ORCON_VERSION`, run the plan, and start the new version.
