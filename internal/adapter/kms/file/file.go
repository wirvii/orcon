// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package file is the `file` master key provider (ADR-0042 section 2): a file
// holding 32 raw bytes or their base64 text. It is an adapter package: it
// implements the envelope.MasterKey port of the credential area, and only
// cmd/orcon imports it.
package file

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/wirvii/orcon/internal/credential/envelope"
)

// Provider is the file master key provider.
type Provider struct {
	*envelope.LocalMasterKey
}

var _ envelope.MasterKey = (*Provider)(nil)

// New reads the master key at path. It refuses a file that is not a regular
// file, or that the group or other users can access (any of the mode bits
// 0077), so the process does not start with a key others can read. The check
// is made on the opened file, not on the path, so the file cannot be swapped
// between the check and the read. Permission bits carry no meaning on Windows,
// where only the file type and the size are checked.
func New(path string) (*Provider, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: file provider without a path", envelope.ErrMasterKeyInvalid)
	}
	// Stat before open: opening a FIFO blocks until a writer appears, which
	// would hang the start. Stat follows a symbolic link, so the type and the
	// permissions checked are the target's, which is what is read.
	before, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat master key file %q: %w", path, err)
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %q is not a regular file", envelope.ErrMasterKeyInvalid, path)
	}
	f, err := os.Open(path) //nolint:gosec // G304: the operator chooses the key path
	if err != nil {
		return nil, fmt.Errorf("open master key file %q: %w", path, err)
	}
	defer func() { _ = f.Close() }() // read-only file: a close error cannot lose data

	// The checks that matter are repeated on the opened file, and it must be
	// the file that was checked, so the path cannot be swapped in between.
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat master key file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() || !os.SameFile(before, info) {
		return nil, fmt.Errorf("%w: %q changed while it was being opened", envelope.ErrMasterKeyInvalid, path)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%w: %q has mode %04o, want 0600 or stricter (chmod 600)",
			envelope.ErrKeyFilePermissions, path, info.Mode().Perm())
	}
	raw, err := io.ReadAll(io.LimitReader(f, envelope.MaxKeyMaterial+1))
	if err != nil {
		return nil, fmt.Errorf("read master key file %q: %w", path, err)
	}
	defer clear(raw)
	if len(raw) > envelope.MaxKeyMaterial {
		return nil, fmt.Errorf("%w: %q is larger than %d bytes", envelope.ErrMasterKeyInvalid, path, envelope.MaxKeyMaterial)
	}
	key, err := envelope.ParseKeyMaterial(raw, true)
	if err != nil {
		return nil, fmt.Errorf("master key file %q: %w", path, err)
	}
	return &Provider{envelope.NewLocalMasterKey("file", path, key)}, nil
}
