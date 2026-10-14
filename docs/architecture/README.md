<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Architecture

Orcon ships a single multi-role Go executable with a statically embedded React editor.

- Local standalone: SQLite (planned).
- Distributed: PostgreSQL for durable state and task leasing (planned).
- Redis: optional optimization, not required.
- External effects: at-least-once execution with explicit idempotency and reconciliation policies.

**Status:** architectural intent only. No durable runtime implemented.
