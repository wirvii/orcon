---
title: Backup and restore
description: Back up an Orcon instance while it runs, check the backup, and restore it safely.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0048
  - ADR-0073
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Backup and restore

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers `orcon backup` removes this note.

Orcon backs itself up while it runs, writes the backup encrypted, and keeps the key material in a different place from the data. A backup of the data without the keys is useless, and the keys without the data are useless too.

## What a backup contains

A backup is written to a target as three kinds of file.

| File | Where | What it holds |
| --- | --- | --- |
| `manifest.json` and `manifest.json.mac` | `backup.target` | The backup description in plain text, authenticated with a MAC. It lists the version, the parts with their SHA-256, the row counts and any warnings. It is written last: a backup without a manifest does not exist as far as listing, retention and restore are concerned. |
| `database.age` and `blobs-NNNN.age` | `backup.target` | The data: the database and the blobs stored on the file system, compressed with zstd and encrypted with age. File blobs are split in parts of 1 GiB. |
| `<backupId>.keys.age` and `<backupId>.keys.json` | `backup.keys.target` | The key artifact: the fingerprint, the provider and the reference of the master key, and, for file or environment keys, the key material encrypted to the age recipients you list in `backup.keys.recipients`. |

The data is encrypted so that the master key can open it, and also the recovery key when you configured `keys.recoveryPublicKey`. The key artifact can be opened only by the age identities you hold. The master key itself is never written next to the data. `backup.keys.target` must be a different directory, bucket or prefix from `backup.target`, and `orcon config check` reports an error if it is not.

If you use a file or environment master key and list no recipients, the backup completes with the warning `KEY_MATERIAL_NOT_EXPORTED`. In that case only the recovery kit you saved at first run lets you restore. The warning shows in the manifest, the API, the metrics and `orcon config check`.

## Choose a target and a schedule

Set these keys in your configuration file (see the [configuration overview](../configuration/overview.md)).

| Key | Meaning |
| --- | --- |
| `backup.target` | Where the data goes: `file:///<path>` or `s3://<bucket>/<prefix>`. |
| `backup.keys.target` | Where the key artifact goes. Another location than `backup.target`. |
| `backup.keys.recipients` | A list of age public keys (`age1…`) that can open the key artifact. |
| `backup.schedule` | A cron expression in UTC. Empty means no scheduled backups. |
| `backup.retention.daily`, `weekly`, `monthly` | How many days, ISO weeks and months to keep. Defaults 7, 4 and 0. |
| `backup.spoolDirectory` | Local directory for the temporary copy. Default `<dataDir>/tmp`. |

S3 targets use the S3 API and accept static keys read from files, or the identity of the server's cloud account for instance-level targets. Uploads use 64 MiB parts. S3-compatible servers other than AWS S3 should work but are not tested.

### Retention

Retention keeps the last successful backup of each of the last `daily` UTC days, of the last `weekly` ISO weeks and of the last `monthly` months. The target is the source of truth, also after a restore. Orcon never deletes the last good backup, nor one that a verification or a restore is using. Each deletion is audited as `backup.pruned` and removes the data and the keys of that backup together. An object that the provider has locked produces a warning, not an error.

## Create a backup

While the instance runs, the `scheduler` role (or `standalone`) runs the backup, one at a time per instance. Start one on demand with:

```sh
orcon backup create
```

The command calls the API (`POST /api/v1/backups`) and needs the permission `instance:backups`. List and inspect backups with:

```sh
orcon backup list
```

With the instance stopped, run the same command on the files directly:

```sh
orcon backup create --offline --config <file>
```

`<file>` is the configuration file of the instance.

### What is consistent, exactly

- **SQLite.** The database part is the state of one read transaction, copied with the SQLite online backup API in a single step. It does not block writers. The copy is written to `<dataDir>/tmp`, checked with `PRAGMA quick_check`, and encrypted from there. While the copy runs the write-ahead log cannot be compacted past that read, and the temporary space needed is the size of the database. Orcon checks that free space is at least 1.1 times the database size and fails before it starts if it is not.
- **PostgreSQL.** The database part is the state of one `REPEATABLE READ, READ ONLY` snapshot. Orcon copies each organization's rows inside that snapshot with the organization context set for the transaction, so row-level security stays enforced and no role that bypasses it is needed. An engine function counts the rows in the same snapshot, and the backup fails if the counts do not match what was copied. The snapshot is written to the local spool directory, the transaction ends, and only then is it uploaded, so no transaction stays open during an external call. While it runs, the snapshot holds back PostgreSQL's cleanup of old row versions.
- **Blobs.** Every blob referenced by a committed row in the snapshot is in the backup, with its SHA-256 checked as it is read. Blobs written after the snapshot are not. While a backup runs, physical deletion of blob files is postponed. With the PostgreSQL blob backend, blobs are part of the database snapshot. With the S3 blob backend, the backup does not copy the objects; it stores their list (key, size, SHA-256), and `verify` checks that they exist. Enable versioning on the bucket and keep it at least as long as your backups.

For large PostgreSQL databases, use the native tools: a base backup with WAL archiving, or `pg_dump` with a role created by your database administrator that bypasses row-level security. `orcon db setup-roles --backup-role` prints the SQL for that role. These tools do not cover the file blob directory (if you use it) or the master key material, so keep the Orcon key backup as well. To restore after a native restore, see "After restoring the database yourself" below.

## Check a backup

A backup you have not checked is a hope. Verify:

```sh
orcon backup verify <backupId>
```

Verification checks the MAC of the manifest, the checksum of each part and the decryption. For SQLite it then restores into a temporary file and runs `PRAGMA integrity_check` and `PRAGMA foreign_key_check`, compares the schema checksums with the ones of that version, compares the row counts with the manifest, checks the key canaries and the SHA-256 of every blob. That is the `FULL` level. For PostgreSQL, set `backup.verify.postgres.dsn` to a test database that Orcon empties before and after, and verification does the same there. Without it, verification runs at the `ARCHIVE` level and says so.

To check a backup from a different machine, without the instance:

```sh
orcon backup verify --from <uri> --offline
```

`<uri>` is the backup location. Set `backup.verify.schedule` to verify the latest backup on a schedule. Results are recorded, audited as `backup.verified` and exposed as metrics; the alerts `OrconBackupMissing`, `OrconBackupFailed` and `OrconBackupNotVerified` use them.

## Restore

Restoring is a local command. It never runs through the API. Run it on the machine where you want the instance.

```sh
orcon backup restore --from <uri> --keys <uri> --identity <file>
```

`--from` is the backup location, `--keys` is the key artifact location and `--identity` is an age identity file that can open it. `--keys` and `--identity` are optional.

Before it changes anything, the command shows the backup id, its date, the source instance, the schema version and the destination, and asks you to type the backup id. In a script, pass `--confirm <backupId>`.

Requirements:

- **SQLite.** The data directory is locked, and the destination database does not exist. With `--replace`, the existing database is moved to `<path>.pre-restore-<date>`. It is never deleted.
- **PostgreSQL.** The destination database is empty and no process is running.
- The binary must contain migrations up to the schema version of the backup. If it does not, the command exits with code 4 and names the minimum version.

The restore applies the schema, the rows, the blobs, checks the row counts and the key canaries, and sets tokens that were leased back to runnable with a new epoch. On PostgreSQL this happens in one transaction; on SQLite it uses an atomic rename.

### The instance starts on hold

After a restore, the instance is on hold. It claims no work and fires no triggers until an instance administrator releases the hold. Effects that were sent after the backup was taken are not known to the restored data, and Orcon does not claim to send an external effect exactly once across a restore; the hold gives a person the chance to decide. When the instance starts, it raises the operational alert `INSTANCE_RESTORED` for instance administrators, and the web interface shows a persistent banner with the "Release hold" action.

Release the hold when you are ready. It needs the permission `instance:maintenance`, asks for confirmation and is audited.

```sh
orcon instance hold release
```

After the release, external operations that were being sent when the backup was taken follow the normal recovery of the operation journal: they are reconciled, or they become uncertain operations that a person resolves. Nothing is sent again without going through that path. For automated recovery drills, `orcon backup restore --release-hold` releases the hold as part of the restore and is audited in the same way.

### Audit chain after a restore

Audit anchors are kept outside the database so that truncation is detected. A restore moves the anchors that are newer than the backup to `<dataDir>/anchors/pre-restore-<date>/` and writes the audit event `backup.restored`. The event records the last anchor included in the backup, how many chain events the backup lacks according to the archived anchors, and the operating-system user and host that restored. `orcon audit verify` recognizes this declared discontinuity and does not report it as tampering.

### After restoring the database yourself

If you restored the database with PostgreSQL's native tools, run the Orcon restore steps that come after the data copy:

```sh
orcon backup restore --database-restored-externally
```

This applies only the later steps: the instance hold, the tokens, the audit event and the anchors.

## Back up before you upgrade

SQLite takes an automatic local backup before it migrates. With PostgreSQL, take a backup before every upgrade. See [Upgrade and rollback](upgrade-and-rollback.md).
