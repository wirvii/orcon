// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Probe checks that the database file can be opened and read, for orcon config
// check --probe. It opens the file read-only (mode=ro), so it creates nothing
// and does not take the process lock of [Open]; it then runs SELECT 1.
//
// When the file does not exist yet, Probe returns exists false and no error:
// the server creates it on first start.
func Probe(ctx context.Context, path string) (exists bool, err error) {
	if path == "" {
		return false, errors.New("probe sqlite: path is required")
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("probe sqlite: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+escapePath(path)+"?mode=ro&_pragma=busy_timeout(2000)&_pragma=query_only(1)")
	if err != nil {
		return true, fmt.Errorf("probe sqlite: open: %w", err)
	}
	defer func() { _ = db.Close() }() // read-only handle: nothing is lost if Close fails
	var one int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		return true, fmt.Errorf("probe sqlite: SELECT 1: %w", err)
	}
	return true, nil
}
