---
title: Install the single binary
description: Install Orcon on one machine with the one-line installer, and know where it keeps its data.
sources:
  - ADR-0083
  - ADR-0081
  - ADR-0002
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Install the single binary

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers the installers removes this note.

The `orcon` executable is published for Linux, macOS and Windows, on amd64 and arm64. It works alone with its built-in SQLite database and needs no other service.

## Install with the one-line installer

On Linux and macOS the installer is a POSIX shell script that needs `curl` or `wget`.

```sh
curl -fsSL https://github.com/wirvii/orcon/releases/latest/download/install.sh | sh
```

On Windows the installer is a PowerShell script, `install.ps1`, for PowerShell 5.1 or newer. Both scripts are published as assets of every release and are listed in its `checksums.txt`.

The installer does these steps in order:

1. Detects your operating system and architecture.
2. Downloads the archive of the version you asked for (by default, the latest version that is not a pre-release), the `checksums.txt` file and its signature bundle.
3. Verifies the signature of `checksums.txt` with cosign, against the identity and issuer described in [Verifying releases](verifying-releases.md). If cosign is missing or older than the minimum version, the installer downloads a pinned cosign, checks its SHA-256 (which is written inside the installer) and uses it from a temporary directory.
4. Verifies the SHA-256 of the archive.
5. Extracts the archive and installs `orcon` in `/usr/local/bin` when it can write there, and in `~/.local/bin` otherwise. The `--dir <directory>` option chooses another directory.
6. Prints the output of `orcon --version`.

If any step fails, the installer exits with a non-zero code and installs nothing. There is no option to skip the verification.

The installer contacts three hosts, and no others: `github.com`, `objects.githubusercontent.com`, and the server of the Sigstore trust root that cosign uses.

If you prefer to download and check the files yourself, follow [Verifying releases](verifying-releases.md) and put the `orcon` executable anywhere on your path.

## Start the server

```sh
orcon serve
```

With no arguments, `orcon serve` starts the `standalone` role. It listens on `127.0.0.1:8080` and keeps its data in the default data directory of your operating system:

| System | Default data directory |
| --- | --- |
| Linux | `$XDG_DATA_HOME/orcon`, or `~/.local/share/orcon` when `XDG_DATA_HOME` is not set |
| macOS | `~/Library/Application Support/Orcon` |
| Windows | `%LOCALAPPDATA%\Orcon` |

Change the directory with the `dataDir` key, the `ORCON_DATA_DIR` environment variable, or the matching flag. The first time the server starts, it waits for the [first run](../configuration/first-run.md) to be completed.

## Roles

The `--role` flag (or the `role` key) accepts `standalone`, `api`, `scheduler` and `worker`. The default is `standalone`. SQLite works only with `standalone`; `orcon config check` reports an error for any other role with SQLite. To run the roles as separate processes you need PostgreSQL. See [Docker Compose](docker-compose.md) and [Kubernetes with Helm](kubernetes-helm.md) for complete examples.

## Run it as a service with systemd

The repository will include a hardened example unit file under `deploy/systemd`, for running the `standalone` role as a systemd service on Linux. Orcon does not run as a Windows service in version 1.

## Next steps

- Complete the [first run](../configuration/first-run.md).
- Read the [configuration overview](../configuration/overview.md).
- Plan your [backups](../operations/backup-and-restore.md) before you store anything you cannot lose.
