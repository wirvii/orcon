---
title: Audit and SIEM
description: What Orcon records about who did what, how it detects tampering, and how to send the events to your security tools.
sources:
  - ADR-0048
  - ADR-0083
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Audit and SIEM

> This page describes behavior specified by ADR 0048 and ADR 0083 that is not implemented yet. The slices that deliver the audit log remove this note.

Orcon keeps an audit log of the actions that matter: sign-ins, changes to flows, credentials, roles and policies, key operations, each use of a credential, and each view of protected execution data. The log is tamper-evident, and you can send it to your security information and event management system (SIEM).

## What an event contains

Each event records when it happened, who did it (a user, a service account, an SCIM client, an API key or the system), the action, the target, the outcome (`SUCCESS`, `FAILURE` or `DENIED`), a reason, details with no secrets and no execution data, and the request (identifier, address and user agent). An event written by a request over the API, the command line, an SDK or MCP says which channel it came from. The event format is `orcon/audit@1`.

The audit log is written in the same transaction as the action, so a committed action always has its record. Denied actions are recorded too.

## Tamper evidence

Events are chained: each one carries a hash of the previous one, and groups of events are sealed with a keyed checksum. There is one chain for each organization and one for the instance. Deleting or changing an event, even directly in the database, breaks the chain.

- Verify at any time with `orcon audit verify`, and with `--org` for one organization. Orcon also verifies every hour.
- A failure raises the event `audit.integrity_violation`, an error in the log, a metric, a banner for administrators and a SIEM event.
- Once a minute, the head of each chain is appended to a file in the data directory (`audit-anchors.log`) and sent to each configured SIEM sink. A copy outside the machine is what lets Orcon notice that someone removed the newest events. Until a chain has an external sink, administrators see a persistent warning.
- After a restore, anchors newer than the backup are moved to an archive and the gap is recorded, so verification reports it as a known discontinuity and not as tampering.

## Send events to your SIEM

| Sink | Notes |
| --- | --- |
| Syslog (RFC 5424) | Over TLS (RFC 5425), TCP or UDP |
| OpenTelemetry logs | OTLP over HTTP or gRPC |
| HTTPS webhook | JSON lines in batches of up to one second or 500 events, signed with the header `Orcon-Signature` (HMAC-SHA-256) |
| JSON-lines file | A local file |

Delivery is at least once. Orcon keeps a cursor for each sink and chain, aims for a p99 lag of 5 seconds, and raises `siem.sink_lagging` if the lag exceeds 60 seconds. An organization sink (permission `siem:manage`) receives the chain of its organization. An instance sink receives every chain. Network sinks use Orcon's outgoing client with the hosts you configure.

## Retention

The default is 365 days, and each organization may configure more, above the instance's floor. Pruning removes whole sealed blocks and records a checkpoint so that verification continues. Audit retention is independent from the retention of execution history.

## Permissions

Reading the audit log needs `audit:read` and exporting it needs `audit:export`. See [Authorization](authorization.md).
