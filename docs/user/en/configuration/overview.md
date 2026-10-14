---
title: Configuration overview
description: How Orcon reads its settings and in which order, and how to check the result.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0039
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Configuration overview

Orcon reads its settings from four places, highest first: command-line flags, ORCON_* environment variables, the YAML file passed with --config (or ORCON_CONFIG), and built-in defaults. Each key is resolved on its own.

- YAML keys are camelCase; unknown keys are errors.
- Environment names are ORCON_ plus the key path in UPPER_SNAKE (store.postgres.maxConnections → ORCON_STORE_POSTGRES_MAX_CONNECTIONS). Exceptions: ORCON_PUBLIC_URL, ORCON_MASTER_KEY, ORCON_MASTER_KEY_BACKUP_ACK.
- Secrets can be read from files: dsnFile in YAML, ORCON_STORE_POSTGRES_DSN_FILE in the environment, --store-postgres-dsn-file as a flag. Secret values are never accepted as flags.
- Run `orcon config check` to see every effective value, where it came from, and any error. Secrets are shown as [REDACTED].
- The server configuration file is never named orcon.yaml; that name belongs to flows-as-code repositories.
