// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package dbrules checks the database rulebook (.agents/skills/orcon-db) against
// real databases: the schema by introspection of the catalog (SQLite and
// PostgreSQL), the connection and session settings, the plans of hot queries
// and the upgrade from every earlier seeded version.
//
// It complements tools/dbcheck, which checks migration and query files without a
// database. Both read the declaration files of the policy directory, so no list of
// names lives in code. Every function returns findings (or reports through the
// test it is given); none passes silently when it cannot do its work.
//
// SPEC-001 C2 runs these checks against the real storage adapters: LintSQLite and
// LintPostgres after the migrations, CheckSQLiteConnection and
// CheckPostgresSession against the real openers, CheckPlansSQLite and
// CheckPlansPostgres with the real hot queries, and RunUpgradeTests with the real
// migration runner.
package dbrules
