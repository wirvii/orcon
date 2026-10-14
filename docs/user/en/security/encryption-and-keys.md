---
title: Encryption and keys
description: What Orcon encrypts, with which keys, and what you must do to keep the data recoverable.
sources:
  - ADR-0042
  - ADR-0049
  - ADR-0083
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Encryption and keys

> This page describes behavior specified by ADR 0042, ADR 0049 and ADR 0083 that is not implemented yet. The slices that deliver the key hierarchy remove this note.

Orcon encrypts the sensitive data that it stores. This page says what is encrypted and by which key. For the choices you make about the master key, and the procedures, see [Master key and KMS](../configuration/master-key-and-kms.md).

## What is encrypted

- **Credentials.** Secret fields and OAuth tokens, per credential version.
- **Secrets in configuration of an organization.** For example, the client secret of an identity source and the configuration of an external secret store, under the organization key.
- **Instance secrets.** For example, the seeds of two-factor codes, under the instance key.
- **Working state of sensitive executions.** Encrypted with a key for each execution, which is destroyed when the execution ends, so that Orcon can recover from a crash without running the same external actions again, and the data cannot be read afterwards. The execution history is always masked.
- **Backups.** The data artifact is encrypted. See [Backup and restore](../operations/backup-and-restore.md).

Orcon does not encrypt the whole database. If you need that, use the encryption of your storage or your database service as well.

## The layers

1. The **master key** comes from a provider you choose and is never in the database.
2. The **organization keys** and the **instance key** are stored wrapped by the master key.
3. The **data keys** protect individual credentials and execution states, and are wrapped by an organization key or the instance key.

The algorithm is AES-256-GCM. Every encrypted value carries authenticated information about its organization, object, version and field, so a value cannot be moved to another place and still decrypt.

## Why layers matter to you

- Rotating the master key rewrites tens of key records, not your data.
- Each organization has its own key. Deleting an organization, after its grace period, destroys its keys, and its credentials and working state become unreadable even in old backups.
- With a key management service, disabling the key there stops Orcon from unwrapping within one hour.

## Keys are checked at startup

Before any write, Orcon checks that every active key can decrypt a known test value. A process never writes data under a key it has not verified. If the check fails, Orcon exits and names the provider, the key reference and the expected fingerprint.

## Protected execution data

Seeing execution data is a separate permission from seeing that an execution ran. See [Authorization](authorization.md).

## Your duties

- Save the recovery kit during the [first run](../configuration/first-run.md), outside this machine.
- Keep backups of the data and of the keys in different places.
- Protect the master key as strictly as the data it protects: a file readable only by the Orcon account, or a key management service with deletion protection and key rotation.
