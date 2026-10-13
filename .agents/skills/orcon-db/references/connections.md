<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Connections

Status: Accepted

This document defines the per-connection settings of SQLite and the session settings of PostgreSQL. Rule IDs (`CN-…`) are stable. `policy/` means `internal/adapter/storetest/dbrules/policy/`.

Precedence: accepted ADRs > accepted Go rulebook rules (`orcon-go` DA-1..DA-8) > this rulebook > reviewer preference.

## SQLite

- **CN-1** One process per database file, guarded by an exclusive lock file (ADR-0015 §5).
- **CN-2** Every connection, writer and readers, sets on open through the driver's per-connection pragma parameters (so reconnects inherit them): `foreign_keys = ON`, `busy_timeout = 5000`, `trusted_schema = OFF`.
- **CN-3** Writer: one `*sql.DB` with `SetMaxOpenConns(1)`; `journal_mode = WAL` (persistent; asserted at startup); `synchronous = FULL` by default (`NORMAL` is a documented, weaker opt-in); write transactions start with `BEGIN IMMEDIATE`; the writer goroutine applies queued transactions with group commit.
- **CN-4** Readers: a separate pool with `query_only = ON`, size configurable (default `GOMAXPROCS`); read transactions are short because a long reader blocks WAL checkpoints.
- **CN-5** Checkpoints: `wal_autocheckpoint` default; `journal_size_limit = 67108864`; `PRAGMA optimize` at close and every six hours.
- **CN-6** Startup fails if any pragma differs from the expected value. `TestForeignKeysOnEveryConnection` holds several reader connections and the writer at once and checks each.

## PostgreSQL

- **CN-7** Per-connection runtime parameters: `TimeZone = UTC`, `application_name = orcon-<role>`, `statement_timeout` (request plane 30 s, configurable), `idle_in_transaction_session_timeout = 10s`, `lock_timeout = 5s`; isolation READ COMMITTED.
- **CN-8** `SET ROLE orcon_application` on each pooled connection, asserted per transaction; the organization context is empty when a transaction starts (TI-6).
