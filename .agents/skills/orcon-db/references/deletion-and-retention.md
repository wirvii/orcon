<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Deletion and retention

Status: Accepted

This document defines how rows are deleted, audited, recovered (flows only) and purged by retention. Rule IDs (`DL-…`) are stable. `policy/` means `internal/adapter/storetest/dbrules/policy/`.

Precedence: accepted ADRs > accepted Go rulebook rules (`orcon-go` DA-1..DA-8) > this rulebook > reviewer preference.

## Rules

- **DL-1** Deletion is physical (hard delete) by default. Every table except those in `policy/soft_delete_tables.txt` uses hard delete only.
- **DL-2** A user-initiated deletion of an organization's business entity runs in a store method that, in the same transaction, inserts into `audit_pending` an event `<entity>.deleted` with actor, target type, id and name, and no payload, secret or execution data (ADR-0048, R-AUD-08). Every DELETE query is annotated: `-- orcon:delete audit:<action>`, `-- orcon:delete retention:<class>` or `-- orcon:delete bookkeeping:<reason>` (leases, delivered outbox rows, sealed pending audit rows).
- **DL-3** Cascades are declared per foreign key (IN-1). A cascade that removes another area's data is described in the spec that owns the parent.
- **DL-4** Soft delete exists only where an accepted spec requires recovery: the table is listed in `policy/soft_delete_tables.txt` with the spec reference and has `deleted_at` and `deleted_by_principal_id` (IN-12); no unlisted table has `deleted_at` (schema check). Every query on a listed table filters `deleted_at IS NULL` unless annotated `-- orcon:include-deleted <reason>`. Uniqueness uses a partial unique index over non-deleted rows. After the recovery window, retention deletes the row physically with an audit event `<entity>.purged`.
- **DL-5** The only soft-delete table is `flow`, with a fixed 30-day recovery window:
  - delete: set `deleted_at` and `deleted_by_principal_id`, audit `flow.deleted`, same transaction; running executions continue on their version; the owning specs deactivate triggers and refuse new executions;
  - restore within 30 days: clear both columns, audit `flow.restored`; a name conflict with an active flow in the project is a conflict error and nothing changes;
  - purge after 30 days: `orcon_engine.prune_batch('flow_purge', n)` deletes, in bounded batches, the flow's executions (and their data by cascade), then its versions, snapshots and call edges, then the flow row, and records `flow.purged` (actor: the `SYSTEM` principal) in the transaction that deletes the flow row; a flow with an unfinished execution is retried on the next run;
  - deleting the project or the organization hard-deletes its flows, including those inside the recovery window.
- **DL-6** Accumulating data (executions and their node data, execution events, notifications, AI conversations, audit with its own policy) is purged by `orcon_engine.prune_batch(class, n)` in bounded batches, oldest first, under the retention policies of ADR-0017 (as amended) and R-AUD-07; "always" retention is never pruned by the size cap. Each purge run records one summary audit event per organization (`retention.purged`, counts per class).
- **DL-7** Encrypted data is made unreadable by destroying its key (per-execution key, organization key) where the owning spec says so, before or instead of deleting rows.
- **DL-8** Personal data: hard delete; audit events keep ids or HMACs, never personal payloads; this supports erasure requests (GDPR; Colombian personal data law, Ley 1581 de 2012, habeas data).
- **DL-9** Deleting an organization deletes all its rows by cascade from `org` and destroys its keys, as a background job in bounded batches.
- **DL-10** Deleting a principal never deletes history: actor columns become NULL (IN-12) and the audit log keeps the identity.
