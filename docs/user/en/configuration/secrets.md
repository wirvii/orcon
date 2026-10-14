---
title: Secrets in the configuration
description: How to give Orcon passwords, keys and tokens without putting them on a command line or in a configuration file.
sources:
  - ADR-0083
  - ADR-0042
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Secrets in the configuration

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers the configuration schema removes this note.

Some configuration keys hold secrets: a database connection string, an access key, a password. Orcon treats these keys differently from the others so that a secret does not end up in a process list, a shell history or a file you share.

This page is about the secrets that Orcon needs to start. The credentials that your flows use to call other services are stored by Orcon itself, and are described in [Credentials and secret stores](../security/credentials-and-secret-stores.md).

## The rules

- **Never as a flag.** A secret key does not accept its value on the command line, because the value would be visible to other users of the machine. A secret flag exists only in the file form.
- **Every secret can come from a file.** For a secret key, Orcon accepts three file forms, one for each source of configuration:

  | Source | Form | Example |
  | --- | --- | --- |
  | YAML file | the key name followed by `File` | `dsnFile: /run/secrets/orcon/dsn` |
  | Environment | the variable name followed by `_FILE` | `ORCON_STORE_POSTGRES_DSN_FILE=/run/secrets/orcon/dsn` |
  | Flag | the flag name followed by `-file` | `--store-postgres-dsn-file /run/secrets/orcon/dsn` |

- **One form at a time.** Giving both a value and a file for the same key is an error.
- **One trailing newline is removed** from the content of the file, so a file written with `echo` works.
- **File permissions.** On Unix, Orcon warns when a secret file can be read by other users. Keep these files readable only by the account that runs Orcon.
- **Redacted everywhere.** `orcon config check` shows a secret as `[REDACTED]`. Inside the process, secret values are held in types that redact themselves when printed.

## Which keys are secret

| Key | Environment variable |
| --- | --- |
| `store.postgres.dsn` | `ORCON_STORE_POSTGRES_DSN` |
| `keys.master.key` (only with the `env` provider) | `ORCON_MASTER_KEY` |
| `keys.master.vaultTransit.token` | `ORCON_KEYS_MASTER_VAULT_TRANSIT_TOKEN` |
| `blobs.s3.accessKeyId` and `blobs.s3.secretAccessKey` | `ORCON_BLOBS_S3_ACCESS_KEY_ID` and `ORCON_BLOBS_S3_SECRET_ACCESS_KEY` |
| `backup.s3.accessKeyId` and `backup.s3.secretAccessKey` | `ORCON_BACKUP_S3_ACCESS_KEY_ID` and `ORCON_BACKUP_S3_SECRET_ACCESS_KEY` |
| `backup.verify.postgres.dsn` | `ORCON_BACKUP_VERIFY_POSTGRES_DSN` |
| `observability.otlp.headers` | `ORCON_OBSERVABILITY_OTLP_HEADERS` |
| `setup.owner.password` | `ORCON_SETUP_OWNER_PASSWORD` |

Each of them also has the `_FILE` variable, for example `ORCON_MASTER_KEY_FILE`. The generated configuration reference lists every key with its type, default and whether it is secret.

## Examples

A database connection string kept in a file:

```sh
export ORCON_STORE_POSTGRES_DSN_FILE=/run/secrets/orcon/dsn
orcon serve --role=api
```

The same secret in the YAML file, as a path:

```yaml
store:
  driver: postgres
  postgres:
    dsnFile: /run/secrets/orcon/dsn
```

## In containers and clusters

- **Docker Compose.** The PostgreSQL example declares the secrets with the `secrets:` feature of compose, which mounts them as files, and reads them with the `*_FILE` variables. See [Install with Docker Compose](../install/docker-compose.md).
- **Kubernetes.** The Helm chart accepts no secret values. It refers to Kubernetes Secrets by name, mounts them in `/run/secrets/orcon/` and passes them with `*_FILE`. See [Install on Kubernetes with Helm](../install/kubernetes-helm.md).

## Master key

The master key can come from an environment variable (`ORCON_MASTER_KEY`), but that is the weakest option and Orcon warns about it. Prefer a file with restrictive permissions or a key management service. See [Master key and KMS](master-key-and-kms.md).
