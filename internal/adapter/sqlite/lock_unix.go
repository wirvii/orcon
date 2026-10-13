// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package sqlite

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"syscall"
)

// processLock is the exclusive lock file that keeps a second process away from
// the database file (ADR-0015 section 5, orcon-db CN-1). The lock is held by the
// open file description, so the operating system releases it when the process
// ends, even after a crash.
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
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		holder := readHolder(f)
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("%w: %s%s", ErrLocked, path, holder)
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	// Informational only: the lock, not the content, is what excludes other processes.
	if err := f.Truncate(0); err == nil {
		_, _ = f.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0)
	}
	return &processLock{file: f}, nil
}

func readHolder(f *os.File) string {
	buf := make([]byte, 32)
	n, _ := f.ReadAt(buf, 0)
	if n == 0 {
		return ""
	}
	for i := range buf[:n] {
		if buf[i] == '\n' {
			n = i
			break
		}
	}
	return " (held by process " + string(buf[:n]) + ")"
}

// release unlocks and closes the lock file.
func (l *processLock) release() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	if cerr := l.file.Close(); err == nil {
		err = cerr
	}
	l.file = nil
	return err
}
