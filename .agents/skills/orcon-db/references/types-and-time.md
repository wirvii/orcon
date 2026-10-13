<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Types and time

Status: Proposed — pending owner approval

This document defines the type mapping between PostgreSQL and SQLite, the forbidden types and how instants are stored. Rule IDs (`TY-…`) are stable. `policy/` means `internal/adapter/storetest/dbrules/policy/`.

Precedence: accepted ADRs > accepted Go rulebook rules (`orcon-go` DA-1..DA-8) > this rulebook > reviewer preference.

## Rules

- **TY-1** Type mapping (ADR-0015 §2 as amended by ADR-0077):

  | Logical | PostgreSQL | SQLite (STRICT) |
  |---|---|---|
  | ID | `uuid` | `BLOB` + `CHECK (length(x) = 16)` |
  | Instant | `timestamptz` | `INTEGER` unix microseconds UTC |
  | JSON document | `bytea` (default) / `jsonb` (only with a JSON-path exception) | `BLOB` |
  | Bytes | `bytea` | `BLOB` |
  | Bool | `boolean` | `INTEGER` + `CHECK (x IN (0, 1))` |
  | Integer, counter, sequence number, duration, money | `bigint` | `INTEGER` |
  | Text | `text` + `CHECK (char_length(x) <= n)` when bounded | `TEXT` + `CHECK (length(x) <= n)` |

- **TY-2** Forbidden: PostgreSQL `ENUM` types, domains, composite types, arrays, `varchar(n)`, `char(n)`, `timestamp` without time zone, `date`, `time`, `timetz`, `interval`, `money`, `real`, `double precision`, `numeric` (without an exception), `serial`/`bigserial`, `citext`, non-default collations; SQLite `ANY`, `REAL` (without an exception). Lists become child tables or JSON documents.
- **TY-3** Every SQLite table is declared `STRICT`. `WITHOUT ROWID` needs benchmark evidence and an exception.
- **TY-4** Every instant is UTC. Adapters convert with `t.UTC()` and truncate to microseconds (`t.Truncate(time.Microsecond)`) before writing, so values round-trip equal on both engines (DA-3.2). SQLite stores `t.UnixMicro()`. PostgreSQL connections set `TimeZone=UTC` (CN-7).
- **TY-5** The application supplies every instant from the injected `clock.Clock`; no `DEFAULT now()`, `CURRENT_TIMESTAMP` or trigger sets times. Exception: lease, expiry and recovery-window comparisons inside PostgreSQL engine functions use `clock_timestamp()` so that one clock decides across processes (ADR-0013 §4); SQLite runs in one process and uses the injected clock.
- **TY-6** Local time exists only at the edge. Schedules store an IANA time zone name as `text` next to UTC instants.
- **TY-7** Text is UTF-8. Case-insensitive uniqueness uses a normalized column (`email_normalized`), never `citext` or collations.
