-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The PostgreSQL function orcon_engine.clear_principal_references is replaced by migration 0017 of the
-- PostgreSQL adapter so that it also clears the six actor columns of the editor tables. SQLite has no engine
-- function and no schema change here: the single writer clears the same columns in the adapter, with the
-- batched statements of identity.sql (principalClearers in the adapter lists every one of them).
