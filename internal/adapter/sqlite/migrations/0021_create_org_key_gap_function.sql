-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- SQLite has no engine functions and this migration changes no schema. The organizations without an active
-- organization key are read by the query ListOrgsWithoutActiveKey (queries/keys.sql); PostgreSQL reads them through
-- orcon_engine.list_orgs_without_active_key, created by the PostgreSQL file of this number.
SELECT 1 WHERE 0;
