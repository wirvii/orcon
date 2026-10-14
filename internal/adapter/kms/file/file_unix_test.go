// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package file

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/wirvii/orcon/internal/credential/envelope"
)

// writeKey writes a master key file with the given mode and returns its path.
func writeKey(t *testing.T, dir string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, "target.key")
	if err := os.WriteFile(path, bytes.Repeat([]byte{0x5C}, envelope.KeySize), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestFileProviderRejectsFIFO would hang if the provider opened the path
// before looking at its type: opening a FIFO blocks until a writer appears.
// If it regresses, the test binary stops at the go test timeout.
func TestFileProviderRejectsFIFO(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("mkfifo is not available here: %v", err)
	}
	if _, err := New(path); !errors.Is(err, envelope.ErrMasterKeyInvalid) {
		t.Errorf("New(fifo) = %v, want ErrMasterKeyInvalid", err)
	}
}

func TestFileProviderFollowsSymlinks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		targetMode os.FileMode
		wantErr    error
	}{
		{"symlink to a 0600 file", 0o600, nil},
		{"symlink to a group-readable file", 0o640, envelope.ErrKeyFilePermissions},
		{"symlink to a world-readable file", 0o644, envelope.ErrKeyFilePermissions},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			target := writeKey(t, dir, tc.targetMode)
			link := filepath.Join(dir, "link.key")
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			p, err := New(link)
			if tc.wantErr == nil {
				if err != nil || p == nil {
					t.Fatalf("New(symlink) = %v, want success", err)
				}
				if p.Reference() != link {
					t.Errorf("Reference = %q, want the configured path %q", p.Reference(), link)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("New(symlink) = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestFileProviderRejectsDanglingSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	link := filepath.Join(dir, "dangling.key")
	if err := os.Symlink(filepath.Join(dir, "gone"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := New(link); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("New(dangling symlink) = %v, want os.ErrNotExist", err)
	}
}
