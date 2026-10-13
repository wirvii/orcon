---
title: Alerts
description: What each Orcon alert means and what to do when it fires.
sources:
  - ADR-0083
  - ADR-0012
  - ADR-0013
  - ADR-0017
  - ADR-0021
  - ADR-0025
  - ADR-0042
  - ADR-0048
  - ADR-0073
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Alerts

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers the monitoring kit removes this note.

The monitoring kit in `deploy/monitoring/prometheus/orcon-alerts.yaml` defines 29 alert rules. This page has one section for each rule, named exactly like the rule. The `runbook_url` annotation of every rule points to its section, with the rule name in lowercase as the anchor.

Every section gives the severity, the condition in plain words, and what to do. The expressions assume that your Prometheus job is called `orcon`, which is the documented default. Metric names are listed in `docs/reference/metrics.md`.

Two commands help with many alerts. `orcon health --ready` asks a local process whether it is ready. `orcon store schema status` lists the processes that are alive, with their version and role.

## OrconInstanceDown

Severity: critical. Fires when Prometheus cannot reach a process for 5 minutes.

What to do:

1. Check that the process or container is running, with your service manager or with `kubectl get pods`.
2. On the machine, run `orcon health`. If it answers but Prometheus does not, the problem is the network path or the address of the operations listener (`ops.address`).
3. Read the log of the last start. A process that exits with code 75 fell below the compatibility floor of the database and must be upgraded; see [Upgrade and rollback](upgrade-and-rollback.md).

## OrconNotReady

Severity: warning. Fires when a process reports that it is not ready for 5 minutes.

`/readyz` answers 503 with a list of checks. Run `orcon health --ready`, or request `/readyz`, and read the check that fails: the store cannot be reached, the schema is older than the compatibility floor, the keys are not verified, the process is shutting down, or (for a worker or scheduler) the process has no recent heartbeat. Fix the failing check. A process that is draining on purpose is not ready by design.

## OrconHTTPErrorRateHigh

Severity: warning. Fires when more than 5 % of the HTTP requests answered in the last 5 minutes had a 5xx status, for 10 minutes.

What to do: look at the logs of the `api` processes for errors around the same time (each response carries a request id), check that the store answers (the storage dashboard shows transaction latency and errors), and check whether the errors began with a deployment or a configuration change.

## OrconExecutionFailureRatioHigh

Severity: warning. Fires when more than 20 % of the executions that finished in the last 15 minutes ended `FAILED`, for 15 minutes.

What to do: list the failed executions and group them by flow to see whether one flow or one external service explains the failures. A broad failure usually points at a shared dependency: a service that is down, an expired credential, or a change in the instance. The error rules and retries of each flow decide what counts as a failure.

## OrconClaimLatencyHigh

Severity: warning. Fires when the 99th percentile of the time a runnable step waits to be claimed is above 5 seconds, for 10 minutes.

Work is ready, but workers are slow to pick it up. Check that workers are running and not paused (see `OrconWorkerAdmissionPaused`), add workers if they are all busy, and check the latency of store transactions on the storage dashboard.

## OrconRunnableBacklogGrowing

Severity: warning. Fires when more than 1,000 steps are ready to run and the number keeps growing, for 15 minutes.

Workers are not keeping up with the arrival of work. Add workers, check `OrconClaimLatencyHigh` and `OrconWorkerAdmissionPaused`, and look for a slow external service that holds steps for a long time. On Kubernetes you can scale workers on this metric, for example with KEDA.

## OrconLeaseExpirations

Severity: warning. Fires when more than 0.1 leases per second expire over 10 minutes, for 10 minutes.

A worker takes a lease on the work it runs and renews it. Leases expire when a worker stops renewing, because it was killed, ran out of memory, lost its connection to the store or was stalled. Check for restarts and out-of-memory kills of workers, the latency of the store, and the load on the worker hosts. Expired leases are recovered automatically; the alert is about why they expire.

## OrconPoisonTokens

Severity: warning. Fires as soon as a step was suspended as poison in the last hour.

A step that is abandoned three times (the default) is suspended with the reason `POISON`, and an attention request with the cause `POISON_TOKEN` is opened. A crash caused by the data of one execution is the usual reason. Open the attention request, look at the execution and its input, fix the flow or the data, and then continue or cancel the execution.

## OrconUncertainOperationsOpen

Severity: warning. Fires when at least one external operation has an unknown outcome for 5 minutes.

An operation became `UNCERTAIN` when Orcon cannot tell whether the external system received the change. Orcon never sends such an operation again without a decision. List the items awaiting action with `orcon attention list`, check in the external system whether the change happened, and resolve the item with that evidence. Resolving records who decided, the evidence and whether a risk was accepted.

## OrconAttentionOverdue

Severity: warning. Fires when the oldest open request for human attention is older than one hour, for 5 minutes.

A person has to act and nobody did. Run `orcon attention list` and act on the oldest items. If people were not notified, check the notification channels and `OrconNotificationDeliveriesDead`.

## OrconRetentionPressure

Severity: warning. Fires when stored execution data uses more than 95 % of its size cap, for 30 minutes.

Execution data is pruned by retention policy, but flows with the policy `ALWAYS` keep their data and can fill the cap. Review which flows use `ALWAYS`, shorten the retention of the others, or enlarge the storage and raise the cap with it. See the retention and storage guidance of your version.

## OrconDiskProtectionMode

Severity: critical. Fires when the instance entered disk protection mode and stayed in it for 1 minute.

The data volume is nearly full. In this mode flows whose policy is not `ALWAYS` store no new execution detail beyond what a running execution needs, `ALWAYS` flows keep storing, and all flows keep running. Free space or enlarge the volume. The mode ends when free space is above the threshold by 2 percentage points or 1 GiB.

## OrconDataVolumeLow

Severity: warning. Fires when less than 10 % of the data volume is free, for 15 minutes.

Free space before the disk protection mode starts. Look for what grew: execution data (see `OrconRetentionPressure`), local backups, the temporary directory, the write-ahead log of SQLite (see `OrconSQLiteWALLarge`). Enlarge the volume or reduce what is stored.

## OrconBackupMissing

Severity: critical. Fires when a backup schedule is configured and the last successful backup is more than 26 hours old, for 10 minutes.

Run `orcon backup list` to see the latest runs and their errors. If runs failed, see `OrconBackupFailed`. If there are no runs, check that a `scheduler` or `standalone` process is running and that `backup.schedule` is set. Take a backup now with `orcon backup create`. See [Backup and restore](backup-and-restore.md).

## OrconBackupFailed

Severity: warning. Fires as soon as a backup failed in the last 24 hours.

Run `orcon backup list` and read the error of the failed run. Common causes are an unreachable target or wrong target credentials, too little free space for the temporary copy (on SQLite the free space must be at least 1.1 times the size of the database), and, on PostgreSQL, a row count in the snapshot that does not match the rows copied.

## OrconBackupNotVerified

Severity: warning. Fires when the last verification is older than 8 days, for 1 hour.

Run `orcon backup verify <backupId>` on the latest backup and set `backup.verify.schedule` so that it happens by itself. A backup that was never verified may not restore.

## OrconAuditVerificationFailed

Severity: critical. Fires as soon as the audit log verification failed in the last hour.

The audit log is tamper-evident, and the verification found a break in the chain: a missing event, a changed event, a bad block seal, or a chain head behind its anchor. Run `orcon audit verify` (add `--org` to check an organization) to see what failed. Keep the database and the logs as they are, and treat the alert as a security incident until you know the cause. A restore that moved anchors is recorded as a declared discontinuity and is not reported as a failure.

## OrconSIEMLagHigh

Severity: warning. Fires when an audit export sink is more than 60 seconds behind, for 10 minutes.

Events are delivered at least once and each sink keeps its position, so a lagging sink loses nothing, but the SIEM sees events late. Check that the sink is reachable and accepts the events (network, credentials, capacity at the destination).

## OrconKeyUnwrapFailures

Severity: critical. Fires as soon as a key could not be unwrapped in the last 15 minutes.

Orcon opens its working keys with the master key. With a KMS provider, the keys are unwrapped again every hour, so disabling the KMS key, removing the permission of the server's identity or losing the connection to the KMS shows up here. Check that the KMS is reachable, that the key is enabled and that the identity may use it. Without a KMS, check that the master key file or variable is the one the instance was set up with.

## OrconOutboxBacklog

Severity: warning. Fires when the oldest pending outbox entry is older than 300 seconds, for 10 minutes.

The outbox hands committed changes to their consumers, such as public events and Git commits. A backlog means these consumers are stalled. Check that `scheduler` and `worker` processes are running, the latency of the store, and whether one consumer keeps failing.

## OrconNotificationDeliveriesDead

Severity: warning. Fires as soon as a notification delivery used all its attempts and failed in the last hour.

Check the destination of the channel: the webhook address, the credentials, the mail server. Fix it. For webhook channels, send the delivery again with the redeliver operation of the API (`POST /deliveries/{deliveryId}/redeliver`). While deliveries fail, people may not learn about waiting executions, so also look at `OrconAttentionOverdue`.

## OrconIngressRejecting

Severity: warning. Fires when more than one incoming event per second was rejected over 5 minutes, for 10 minutes.

The most common reason is a cap on pending events, per flow or for the instance, which answers 429 with a `Retry-After`. The trigger events `ingress.rejected` carry the reason. Bring the backlog down by adding workers (see `OrconRunnableBacklogGrowing`), or raise the cap if the load is expected.

## OrconScheduleLag

Severity: warning. Fires when schedules fire more than 60 seconds late, for 10 minutes.

A slot that is late within the grace period still fires. After the grace period it is recorded as missed, and the flow's policy decides whether it is skipped or caught up. Check that `scheduler` processes are running and that the store answers quickly.

## OrconWorkerAdmissionPaused

Severity: warning. Fires when a worker stopped claiming new work, for 10 minutes.

A worker stops claiming when its memory use reaches 85 % of its budget. Check the memory limit of the worker and what uses the memory, such as large payloads or many concurrent steps. Add memory or workers.

## OrconPostgresPoolSaturated

Severity: warning. Fires when a process has no idle PostgreSQL connection, for 10 minutes.

Compare `store.postgres.maxConnections` with the load. The connections of all processes, plus one for notifications and one for locks in each process, plus those used for migrations and backups, must fit in the `max_connections` of the server, minus the reserved ones. Raise the setting if the server allows it, or add capacity. If you use a connection pooler, it must be in session mode.

## OrconVersionSkew

Severity: warning. Fires when processes of more than one Orcon version run for 2 hours.

An upgrade did not finish. Run `orcon store schema status` to see each process with its version and role, and replace the ones that are behind. See [Upgrade and rollback](upgrade-and-rollback.md).

## OrconInstanceOnHold

Severity: critical. Fires as soon as the instance is on hold.

The instance was restored from a backup. It claims no work and fires no triggers until an instance administrator releases the hold. Check that the restore is what you intended, then release the hold from the banner in the web interface or with `orcon instance hold release`. See [Backup and restore](backup-and-restore.md).

## OrconSQLiteWALLarge

Severity: warning. Fires when the write-ahead log of SQLite is larger than 1 GB, for 30 minutes.

The log cannot be compacted while a read transaction older than its end is open. A backup or a verification in progress does this by design while it copies the database. If no backup is running, look for another long reader.

## OrconUpdateAvailable

Severity: info. Fires as soon as a newer stable version exists.

This alert exists only when the version check is enabled (`updateCheck.enabled`). Read the release notes, then plan the upgrade; see [Upgrade and rollback](upgrade-and-rollback.md).
