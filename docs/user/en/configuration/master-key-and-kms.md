---
title: Master key and KMS
description: Choose where the Orcon master key lives, keep the recovery kit safe, and understand what happens if the key is lost.
sources:
  - ADR-0042
  - ADR-0083
  - ADR-0043
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Master key and KMS

> This page describes behavior specified by ADR 0042 and ADR 0083 that is not implemented yet. The slices that deliver the key hierarchy and the first-run commands remove this note.

The master key is the one secret that protects all the others. Orcon never stores it in its database. If you lose it, the data that it protects cannot be read by anyone, including the Orcon project. This page explains your choices and your duties.

## How the keys fit together

Orcon encrypts secrets in layers, so the master key rarely touches your data:

- The **master key** comes from a provider that you choose.
- Each organization has its own **organization key**, and the instance has an **instance key**. They are stored encrypted by the master key.
- Each credential version, and each execution's sensitive working state, has its own **data key**, encrypted by the organization key or the instance key.

All encryption uses AES-256-GCM. Because of the layers, replacing the master key rewrites only a few key records, never your credentials or your data. Deleting an organization destroys its keys, which makes its encrypted data unreadable even in old backups, unless that backup is restored together with a master key that can unwrap a backup copy of the organization key.

## Choose a provider

Set the provider with `keys.master.provider`. The default is `file`.

| Provider | Where the master key lives | Settings |
| --- | --- | --- |
| `file` | A file of 32 bytes (raw or base64). Orcon refuses to start if other users can read it. | `keys.master.file.path`, default `<dataDir>/keys/master.key` |
| `env` | The environment variable `ORCON_MASTER_KEY` (base64). The weakest option; Orcon warns at startup. | `keys.master.key`, also `ORCON_MASTER_KEY_FILE` |
| `awskms` | AWS Key Management Service | `keys.master.awskms.keyArn` |
| `gcpkms` | Google Cloud KMS | `keys.master.gcpkms.keyName` |
| `azurekv` | Azure Key Vault (or Managed HSM) | `keys.master.azurekv.keyId` |
| `vault-transit` | HashiCorp Vault or OpenBao, transit engine | `keys.master.vaultTransit.address`, `.mount`, `.keyName` and the secret `.token` |

With a key management service (KMS), the master key never leaves the service. Orcon asks the service to wrap and unwrap, keeps the unwrapped organization and instance keys in memory (locked in memory where the system allows it), and unwraps them again every hour. Disabling the key in the service therefore takes effect within one hour. The cloud identity of the server can authenticate to the service; that identity belongs to the instance, not to any organization.

## The fingerprint and the startup check

The fingerprint identifies a master key: for a key you hold, it is the SHA-256 of the key, shortened; for a KMS, it is the key reference. At startup, before any migration or write, Orcon uses the master key to open a small test value for each key and checks that it decrypts. If it does not, Orcon stops and tells you which provider, which key reference and which fingerprint it expected. Orcon never writes data under a key it has not verified.

## The recovery kit

The recovery kit contains the key material (or the reference to your KMS key), the fingerprint, instructions to restore, and a warning that loss is irreversible. You get it during the [first run](first-run.md), and you must acknowledge that you saved it. There is no way to skip the acknowledgement.

Store the kit where this machine's failure cannot reach it: a password manager, a safe, a second site. Do not keep it only next to the data or the backups.

## Optional recovery key

You can add a second way to recover. Set `keys.recoveryPublicKey` to an age public key (`age1…`, X25519). Orcon then also wraps every organization key to it. Keep the matching private key offline. If you lose the master key, `orcon key recover` uses the private key to recover. This is off by default.

## Rotation

- **Rotate the master key** with `orcon key rotate-master --to <provider configuration>`. Orcon rewraps every organization and instance key under the new key in small batches, with both keys configured during the change. Your data is not touched, the command can resume after a crash, and canaries confirm the result. Remove the old key only after it finishes.
- **Rotate an organization key** with `orcon key rotate-org <org>`. A new version is used for all new writes, and a background job rewraps the data keys in batches. By default this also happens every year.

Rotations hold no long transactions, slow down under load, and are audited.

## With a KMS: your side of the work

When you validate a KMS key, Orcon warns if the key has no deletion protection or no key rotation. Fix both in the service. Deleting the key there destroys access to everything it protects.

## Backups and the master key

A backup of the data is useless without the master key, and the master key is never written next to the data. For a `file` or `env` key, a backup can also store the key material encrypted to the age recipients you list. See [Backup and restore](../operations/backup-and-restore.md).

## If you lose the master key

Without the master key, and without a recovery key or a backup of the key material, encrypted credentials and the working state of sensitive executions cannot be recovered by anyone.
