-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The ingress lookup of a public request (SPEC-002 R-ADDR-08). The lookup is the ResolveTriggerRoute query, which SQLite
-- runs under its single writer over the tables that already exist; the schema does not change. The PostgreSQL
-- counterpart is the resolve_trigger_route function of the same number.
