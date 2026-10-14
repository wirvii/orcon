---
title: Run the container image
description: Run Orcon from its official image, with a data volume and a health check.
sources:
  - ADR-0083
  - ADR-0081
  - ADR-0002
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Run the container image

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers the image removes this note.

The official image is `ghcr.io/wirvii/orcon:<version>`. It is built for `linux/amd64` and `linux/arm64`, runs as a non-root user, and is signed with cosign. `<version>` is a release number such as `0.3.0`.

## What the image contains

| Setting | Value |
| --- | --- |
| Base | A static distroless image, `nonroot`. It has no shell and no `curl`. |
| User | `65532` |
| Data directory | `/var/lib/orcon`, owned by user `65532` and declared as a volume. `ORCON_DATA_DIR` is set to it. |
| Ports | `8080` for the application (`ORCON_HTTP_ADDRESS=:8080`) and `9464` for the operations listener (`ORCON_OPS_ADDRESS=:9464`) |
| Entry point | `/usr/local/bin/orcon`, with the default command `serve --role=standalone` |
| Health check | `orcon health --ready` |

The image works with a read-only root file system: Orcon writes only to `/var/lib/orcon` and `/tmp`. The image changes only the listen addresses from the secure defaults; Prometheus metrics stay off until you turn them on.

## Run it

```sh
docker run --name orcon -p 8080:8080 -v orcon-data:/var/lib/orcon ghcr.io/wirvii/orcon:<version>
```

Replace `<version>` with the release you want. Always pin a version; do not rely on a moving tag. The volume holds the SQLite database, the master key file and the blobs, so create it before you put anything important in Orcon and include it in your [backups](../operations/backup-and-restore.md).

To use PostgreSQL or any other setting, pass `ORCON_*` environment variables or mount a YAML file and give its path in `ORCON_CONFIG`. Secrets go in files read through the `*_FILE` variants. See [Secrets](../configuration/secrets.md).

## First run

When the server starts for the first time it waits for the [first run](../configuration/first-run.md). The setup token is shown on the terminal only if the output is a terminal. In a container you normally read it with a command:

```sh
docker exec orcon orcon setup token
```

Then open `/setup` on your Orcon address and enter the token. If you reach Orcon through another address than `http://127.0.0.1:8080`, set `ORCON_PUBLIC_URL` to it, and read [TLS and reverse proxy](../configuration/tls-and-reverse-proxy.md).

## Check that it is healthy

```sh
docker exec orcon orcon health --ready
```

The command exits with 0 when the process is ready and with 1 otherwise. It exists because the image has no `curl`.

## Verify the image

Check the signature and provenance of the image before you run it. See [Verifying releases](verifying-releases.md).
