-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- SQLite has no engine functions and this migration changes no schema. The organizations of a user are read by the
-- query ResolveUserOrganizations (queries/identity.sql); PostgreSQL reads them through
-- orcon_engine.resolve_user_organizations, created by the PostgreSQL file of this number.
SELECT 1 WHERE 0;
