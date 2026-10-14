---
title: First run
description: Create the master key and the owner account of a new Orcon instance, in the browser or with commands.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0039
  - ADR-0038
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# First run

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers the first-run commands and screens removes this note.

A new Orcon instance is not usable until two things exist: a master key that protects every secret, and an owner account. You create both once, either with the web wizard or without a browser. Nothing else about your data is written before the key is verified.

## What the server does until setup is complete

Until setup is complete, the server accepts only the setup operations, and they are protected by a one-time setup token. An instance is in one of three states:

| State | Meaning |
| --- | --- |
| `NEEDS_KEY_CEREMONY` | No master key has been verified yet. |
| `NEEDS_OWNER` | The key is verified, and no user is the instance owner yet. |
| `COMPLETE` | Setup is finished. |

Before the state is `COMPLETE`, the server applies migrations and writes only what setup needs. It creates no organization, no user and no encrypted data until the keys are verified.

## The setup token

When the server starts in a state other than `COMPLETE`, it creates a random setup token. It is valid for 24 hours and stops being valid when setup completes.

- If the standard output is a terminal, the server prints `Open <publicUrl>/setup?token=<token>`.
- The token is never written to the logs. The logs say only how to get it.
- To get it later, run `orcon setup token`. With SQLite it reads the token from a file called `setup-token` in the data directory, which only its owner can read. With PostgreSQL, `orcon setup token --rotate` creates a new token and stores only its hash in the database.

To check where the instance is, run `orcon setup status`.

## Option 1: the web wizard

Open the address with the token, or open `/setup` and enter it. The wizard has four steps, in this order, and works with the keyboard and with screen readers. It is available in English and Spanish, and it never shows a state with colour alone.

1. **Token.** Enter the setup token. This starts a setup session that lasts 15 minutes.
2. **Master key.** With the `file` provider, the wizard generates the master key, or uses the one that already exists at `keys.master.file.path`. With a key management service, it checks that Orcon can wrap and unwrap with your key, and warns if deletion protection or key rotation are not enabled there. Then:
   - It offers the recovery kit to download: the key material or the reference to your key management service, the fingerprint, instructions to restore, and a clear warning that losing the key makes the encrypted data unrecoverable.
   - The download is offered only over HTTPS or on the loopback address. Anywhere else the button is disabled and the wizard points you to the commands below.
   - You must type the last 8 characters of the fingerprint to confirm that you saved the kit. There is no way to skip this step.
   - It offers, off by default, an optional recovery public key. It is configured in `keys.recoveryPublicKey`; see [Master key and KMS](master-key-and-kms.md).
3. **Owner account.** Enter an email address, a name and a password. The password must meet the password policy: at least 12 characters, with no composition rules.
4. **Done.** Orcon creates the organization called `default`, creates your user as instance owner and as owner of that organization, records the event `setup.completed`, invalidates the token and takes you to sign in.

After setup, every setup operation except reading the state answers with the problem `setup-already-complete`, and the `/setup` page redirects to sign in.

## Option 2: without a browser

Use this path for scripted installs, containers and Kubernetes.

1. Create the master key. With the `file` provider the command generates it; with a key management service it validates your key. It prints the fingerprint and writes the recovery kit.

   ```sh
   orcon key init
   ```

   `--out <path>` chooses where the key is written and `--kit-out <path>` where the kit is written. Copy the kit to a safe place that is not this machine.

2. Acknowledge that you saved the kit by giving the fingerprint to the server.

   ```sh
   export ORCON_MASTER_KEY_BACKUP_ACK=<fingerprint>
   ```

   `<fingerprint>` is the value printed by `orcon key init`. The server records the acknowledgement as the event `key.backup_acknowledged` with the method `HEADLESS`. If the value does not match the key, the server does not start and exits with code 4.

3. Create the owner account in one of two ways.

   - With the server stopped (SQLite), run:

     ```sh
     orcon setup owner --email <email> --name <name> --password-file <file>
     ```

     `<email>` and `<name>` identify the owner. `<file>` contains only the password.
   - Or set the keys `setup.owner.email`, `setup.owner.name` and `setup.owner.password` (a secret, so it also accepts `ORCON_SETUP_OWNER_PASSWORD_FILE`). The server uses them only while the state is `NEEDS_OWNER`, and warns if they are still present afterwards, so remove them once setup is done.

## With PostgreSQL and several replicas

The state and the hash of the token live in the database, so any `api` replica can serve the wizard, and a lock makes sure only one ceremony runs at a time. The Helm chart requires the commands path above. See [Install on Kubernetes with Helm](../install/kubernetes-helm.md).

## If something is interrupted

An interruption during setup leaves the state at `NEEDS_KEY_CEREMONY` or `NEEDS_OWNER`. A generated key without a confirmed check is not used. If the key file was already written, the next attempt reuses it and shows its fingerprint. No state leaves data encrypted with a key that Orcon does not recognise.

## Next steps

- Read [Master key and KMS](master-key-and-kms.md) and store the recovery kit safely.
- Set up [backups](../operations/backup-and-restore.md).
- Go through the [hardening checklist](../security/hardening-checklist.md).
