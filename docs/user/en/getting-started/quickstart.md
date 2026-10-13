---
title: Quickstart
description: Install Orcon on one machine and run your first flow in about five minutes.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0039
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Quickstart

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers the installer and the first-run commands removes this note.

In about five minutes you install Orcon on one machine, set it up and run a first flow. This path uses the single binary with its built-in SQLite database, so no other service is needed.

You need a Linux or macOS computer with `curl` or `wget`, a terminal and a web browser.

## Steps

1. Install Orcon with the one-line installer. It downloads the release, checks the signature of the checksum file and the SHA-256 of the archive, and installs only if both checks pass. There is no option to skip the checks.

   <!-- install-test -->
   ```sh
   curl -fsSL https://github.com/wirvii/orcon/releases/latest/download/install.sh | sh
   ```

   The installer puts `orcon` in `/usr/local/bin` when it can write there, and in `~/.local/bin` otherwise. To choose another directory, add `--dir <directory>` after `sh -s --`.

2. Check the installation.

   <!-- install-test -->
   ```sh
   orcon --version
   ```

3. Create the master key. It protects every secret Orcon stores, so losing it makes the encrypted data unrecoverable. The command prints the fingerprint of the key and writes a recovery kit.

   <!-- install-test -->
   ```sh
   orcon key init
   ```

   Copy the recovery kit to a safe place that is not this computer, such as a password manager.

4. Tell Orcon that you saved the kit by setting the fingerprint that step 3 printed.

   ```sh
   export ORCON_MASTER_KEY_BACKUP_ACK=<fingerprint>
   ```

   `<fingerprint>` is the fingerprint printed by `orcon key init`. If the value does not match the key, the server refuses to start and exits with code 4.

5. Create the owner account. Run this command before the server starts.

   ```sh
   orcon setup owner --email <email> --name <name> --password-file <file>
   ```

   `<email>` and `<name>` identify the owner. `<file>` is a file that contains only the password, which must meet the password policy of your installation.

6. Start the server.

   <!-- install-test -->
   ```sh
   orcon serve
   ```

   With no arguments, `orcon serve` runs every role in one process (the `standalone` role) and listens only on `127.0.0.1:8080`.

7. Open `http://127.0.0.1:8080` in your browser and sign in with the email and password from step 5.

8. Create a flow with a manual trigger and one step, then run it with the test button, or press Ctrl+Enter (Cmd+Enter on macOS).

9. Open the executions list. Your run appears with the status `SUCCEEDED`.

## Prefer to set up in the browser?

Skip steps 3 to 5 and run `orcon serve`. When the terminal shows an address that starts with `Open`, open it. A setup wizard creates the master key, shows the recovery kit, asks you to type the last eight characters of the fingerprint, and then creates the owner account. The kit is offered only over HTTPS or on the local machine. If you cannot use the wizard, use the steps above.

## What next

- Back up your data: [Backup and restore](../operations/backup-and-restore.md).
- Move to a new version: [Upgrade and rollback](../operations/upgrade-and-rollback.md).
- Change a setting: [Configuration overview](../configuration/overview.md).
- Watch the instance: [Alerts](../operations/alerts.md).
