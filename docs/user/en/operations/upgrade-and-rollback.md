---
title: Upgrade and rollback
description: Move an instance to a new Orcon version, with little or no downtime, and go back if you need to.
sources:
  - ADR-0083
  - ADR-0015
  - ADR-0004
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Upgrade and rollback

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers `orcon store schema` removes this note.

This page shows how to move a running instance to a new version and how to go back. The database schema only moves forward. What decides whether you can upgrade without stopping anything, and whether you can go back by running the old binary, is the compatibility floor.

## The compatibility floor

Each migration declares the oldest Orcon version that runs correctly against the database schema once the migration has been applied. The highest of these versions over all applied migrations is the effective compatibility floor. A process older than the floor must not keep working on the database.

- If every running process is at or above the new floor, the upgrade is rolling: you replace processes one at a time and nothing stops.
- If any running process would fall below the new floor, the upgrade requires a full stop: you stop everything, migrate, and start again.

The notes of every release say which case applies, on one line: `Upgrade: rolling (floor vX.Y.Z)` or `Upgrade: requires full stop (floor vX.Y.Z)`.

## Check before you upgrade

Run these commands with the new binary against your database. On SQLite, stop the instance first.

```sh
orcon store schema status
orcon store schema plan
```

`status` shows the schema version, the effective floor, the migrations already applied, and the running processes with their version and role. `plan` shows what the new binary would change and gives a verdict. For example:

```text
Schema: 0042 (floor v0.6.0)  Binary: v0.8.0 (migrations up to 0047)
Pending: 0043 additive floor v0.7.0; 0044 expand floor v0.7.0; 0045..0047 additive floor v0.7.0
Resulting floor: v0.7.0
Live processes: api v0.7.2 (2), scheduler v0.7.2 (2), worker v0.6.4 (1)
Verdict: REQUIRES_FULL_STOP — worker on host w-3 (v0.6.4) is below v0.7.0
```

The verdict is `ROLLING_SAFE` when every running process stays at or above the resulting floor, and `REQUIRES_FULL_STOP` otherwise. In the second case the output lists the processes that would fall below.

## Upgrade

1. Read the release notes.
2. Take a backup. See [Backup and restore](backup-and-restore.md). With SQLite, Orcon takes a local backup by itself before it migrates (see below); with PostgreSQL, take your own backup first.
3. Run `orcon store schema plan` with the new binary.
4. If the verdict is `ROLLING_SAFE`, replace the processes one at a time, in any order. The first new process applies the migrations when it starts.
5. If the verdict is `REQUIRES_FULL_STOP`, stop every process, run the command below, and start the new version.

   ```sh
   orcon store schema apply --full-stop
   ```

   With `--full-stop` the command requires that no process is running. Without it, the command refuses to run (exit code 4) when the verdict is `REQUIRES_FULL_STOP`.
6. Check that `/readyz` answers 200 on every process, and that the alert `OrconVersionSkew` clears once every process runs the new version.

You can skip minor versions when `orcon store schema plan` says `ROLLING_SAFE`. If it does not, upgrade one version at a time or use a full stop.

### What happens at startup

Automatic migration at startup applies only when the verdict is `ROLLING_SAFE`. Otherwise the process does not start, and its message names the processes that would fall below the floor and the command `orcon store schema apply --full-stop`. Setting `store.migrations.auto` to `false` turns the automatic migration off.

Every process compares its version with the effective floor on each heartbeat. A process that falls below it stops taking new work and new requests, finishes the work in flight as it does on shutdown, and exits with code 75. The log states the cause.

On shutdown, Orcon gives work in flight a grace period of 30 seconds. The platform's kill timeout must exceed it by at least 15 seconds; on Kubernetes, set `terminationGracePeriodSeconds` to 45 or more.

### SQLite: automatic backup before migrating

With SQLite, when migrations are pending and `backup.beforeUpgrade` is `true` (the default for SQLite), the process takes an offline local backup in `<dataDir>/backups/pre-upgrade/` before it migrates. The backup is local even when `backup.target` points to S3, so that startup never depends on the network. The three most recent are kept. With PostgreSQL the default is `false`, so take your own backup first.

### Kubernetes with Helm

The chart runs `orcon store schema apply` as a job before each install and upgrade, and the replicas start with `store.migrations.auto` set to `false`. When the verdict is `REQUIRES_FULL_STOP`, the job fails with the message above and the upgrade stops without touching the deployments. To continue, scale the deployments to zero, set `migrations.fullStop` to `true`, and upgrade.

## Roll back

How you go back depends on the floor.

- **The previous version is at or above the effective floor.** Run the previous version again. Migrations only move forward, and the previous version accepts the newer schema. `orcon store schema status` shows the floor. With Helm, `helm rollback` covers this case.
- **The previous version is below the effective floor.** Restore the backup you took before the upgrade, as described in [Backup and restore](backup-and-restore.md). Everything written after that backup is lost.

Check the floor before you upgrade, not after a failure: a rollback by restoring is a decision to lose recent data.
