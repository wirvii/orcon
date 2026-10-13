// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package sqlite

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// processLock is the exclusive lock file that keeps a second process away from
// the database file (ADR-0015 section 5, orcon-db CN-1). Windows releases the
// lock when the process ends.
type processLock struct {
	file *os.File
}

// acquireLock takes the exclusive lock of path without waiting. A held lock
// returns an error wrapping [ErrLocked].
func acquireLock(path string) (*processLock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600) //nolint:gosec // the path is the configured database lock file
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	var overlapped windows.Overlapped
	err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, fmt.Errorf("%w: %s", ErrLocked, path)
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return &processLock{file: f}, nil
}

// release unlocks and closes the lock file.
func (l *processLock) release() error {
	if l == nil || l.file == nil {
		return nil
	}
	var overlapped windows.Overlapped
	err := windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &overlapped)
	if cerr := l.file.Close(); err == nil {
		err = cerr
	}
	l.file = nil
	return err
}
