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

// probeMode picks how to open the file without changing the directory. A database in
// WAL mode that nobody holds has no -wal and -shm files, and a plain read-only open
// would create them (owned by whoever runs the probe, which breaks the service when
// that is another user). Nobody holds it in that case, so it is opened immutable: no
// locks and no side files. When both files exist a server holds it, and a read-only
// open reads through them without creating anything.
func probeMode(path string) string {
	_, walErr := os.Stat(path + "-wal")
	_, shmErr := os.Stat(path + "-shm")
	if walErr == nil && shmErr == nil {
		return "mode=ro"
	}
	return "immutable=1"
}

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
	db, err := sql.Open("sqlite", "file:"+escapePath(path)+"?"+probeMode(path)+"&_pragma=busy_timeout(2000)&_pragma=query_only(1)")
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
