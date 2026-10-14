---
title: Uninstall Orcon
description: What to keep and what to remove when you take an Orcon installation out of service.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0048
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Uninstall Orcon

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers the installers removes this note. No uninstall command is specified, so removal is manual.

Removing Orcon means removing the program, and then deciding what to do with the data. Decide the second part first, because some of the data cannot be recreated.

## Before you remove anything

1. Stop every Orcon process.
2. Take a final [backup](../operations/backup-and-restore.md) if you may need the data again. Keep the backup and its key artifact in different places.
3. Keep the master key and the recovery kit for as long as you keep any backup or database copy. Without the master key, encrypted data cannot be read. See [Master key and KMS](../configuration/master-key-and-kms.md).
4. If you must keep the audit log, export it first. The audit chain anchors are in the data directory.

## What the data directory holds

The data directory is `dataDir` (see [Install the single binary](binary.md) for its default, and `/var/lib/orcon` in the image). With the default settings it holds:

| Path under the data directory | What it holds |
| --- | --- |
| `orcon.db` | The SQLite database, when you use SQLite. |
| `blobs/` | Payloads stored on the file system. |
| `keys/master.key` | The master key, with the `file` provider. |
| `setup-token` | The setup token, until the first run is complete. |
| `backups/pre-upgrade/` | Automatic backups taken before an upgrade with SQLite. |
| `audit-anchors.log` and `anchors/` | The anchors of the tamper-evident audit log, and archived anchors after a restore. |
| `tmp/` | Temporary files, including backup spooling. |

Deleting the data directory deletes the database, the blobs and, with the `file` provider, the master key. Do it only when you have decided that the data is no longer needed, or when you have a backup and its keys elsewhere.

## Remove the program

- **Single binary.** Delete the `orcon` file from the directory the installer used: `/usr/local/bin` or `~/.local/bin`, or the directory you gave with `--dir`. If you made a systemd unit, stop, disable and remove it.
- **Container.** Remove the container. Remove the data volume only if you no longer need the data.
- **Docker Compose.** Bring the stack down. Remove the volumes only if you no longer need the data.
- **Helm.** Run `helm uninstall` for the release. The Secrets you created for the chart, and your PostgreSQL database, belong to you and stay until you remove them.

## PostgreSQL and external services

Orcon does not remove anything it does not own. If you used PostgreSQL, S3 storage, a backup target or an external key management service, remove those resources yourself when you are sure they are no longer needed. Deleting a key from a key management service makes the data encrypted under it unreadable.
