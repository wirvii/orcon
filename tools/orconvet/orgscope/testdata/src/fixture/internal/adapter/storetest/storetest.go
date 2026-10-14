// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package storetest is a declared exception: contract suites run SQL.
package storetest

import "database/sql"

// Explain builds statement text on purpose: the suites run EXPLAIN on queries.
func Explain(db *sql.DB, statement string) error {
	_, err := db.Exec("EXPLAIN " + statement)
	return err
}

// Reset is allowed to call the driver.
func Reset(db *sql.DB) error {
	_, err := db.Exec("DELETE FROM flow")
	return err
}
