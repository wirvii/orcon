// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/credential/envelope"
)

func TestFileProviderRejectsLoosePermissions(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("permission bits carry no meaning on Windows")
	}
	raw := bytes.Repeat([]byte{0xAB}, envelope.KeySize)
	tests := []struct {
		mode    os.FileMode
		wantErr bool
	}{
		{0o600, false},
		{0o400, false},
		{0o700, false},
		{0o640, true},
		{0o604, true},
		{0o644, true},
		{0o660, true},
		{0o666, true},
		{0o601, true},
	}
	for _, tc := range tests {
		t.Run(tc.mode.String(), func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "master.key")
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, tc.mode); err != nil { // WriteFile is subject to the umask
				t.Fatal(err)
			}
			p, err := New(path)
			if tc.wantErr {
				if !errors.Is(err, envelope.ErrKeyFilePermissions) {
					t.Errorf("New(mode %v) = %v, want ErrKeyFilePermissions", tc.mode, err)
				}
				if err != nil && !strings.Contains(err.Error(), path) {
					t.Errorf("error %q does not name the file", err)
				}
				return
			}
			if err != nil || p == nil {
				t.Errorf("New(mode %v) = %v, want success", tc.mode, err)
			}
		})
	}
}

func TestFileProviderKeyFormats(t *testing.T) {
	t.Parallel()
	raw := bytes.Repeat([]byte{0x11, 0xFE}, envelope.KeySize/2)
	want, err := envelope.NewKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	wantFP := envelope.Fingerprint(want)
	tests := []struct {
		name    string
		content []byte
		wantErr bool
	}{
		{"raw 32 bytes", raw, false},
		{"base64 with newline", []byte(base64.StdEncoding.EncodeToString(raw) + "\n"), false},
		{"base64 without padding", []byte(base64.RawStdEncoding.EncodeToString(raw)), false},
		{"base64 url alphabet", []byte(base64.URLEncoding.EncodeToString(raw)), false},
		{"empty", nil, true},
		{"raw 31 bytes", raw[:31], true},
		{"raw 33 bytes", append(bytes.Clone(raw), 1), true},
		{"base64 of 16 bytes", []byte(base64.StdEncoding.EncodeToString(raw[:16])), true},
		{"not base64", []byte("this is not a key at all, definitely!"), true},
		{"oversized", bytes.Repeat([]byte("A"), envelope.MaxKeyMaterial+1), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "master.key")
			if err := os.WriteFile(path, tc.content, 0o600); err != nil {
				t.Fatal(err)
			}
			p, err := New(path)
			if tc.wantErr {
				if !errors.Is(err, envelope.ErrMasterKeyInvalid) {
					t.Errorf("New = %v, want ErrMasterKeyInvalid", err)
				}
				if err != nil && len(tc.content) > 0 && bytes.Contains([]byte(err.Error()), tc.content) {
					t.Errorf("error %q repeats the key material", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("New = %v, want nil", err)
			}
			if got, _ := p.Fingerprint(t.Context()); got != wantFP {
				t.Errorf("Fingerprint = %q, want %q", got, wantFP)
			}
			if p.Provider() != "file" || p.Reference() != path {
				t.Errorf("provider = %q, reference = %q; want file, %q", p.Provider(), p.Reference(), path)
			}
		})
	}
}

func TestFileProviderRejectsBadPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := New(""); !errors.Is(err, envelope.ErrMasterKeyInvalid) {
		t.Errorf("New(empty) = %v, want ErrMasterKeyInvalid", err)
	}
	if _, err := New(filepath.Join(dir, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("New(missing) = %v, want os.ErrNotExist", err)
	}
	if _, err := New(dir); !errors.Is(err, envelope.ErrMasterKeyInvalid) {
		t.Errorf("New(directory) = %v, want ErrMasterKeyInvalid", err)
	}
}

func TestFileProviderWrapsAndUnwraps(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.key")
	if err := os.WriteFile(path, bytes.Repeat([]byte{3}, envelope.KeySize), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := p.Wrap(t.Context(), []byte("k"), []byte("aad"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Unwrap(t.Context(), wrapped, []byte("aad"))
	if err != nil || string(got) != "k" {
		t.Errorf("Unwrap = %q, %v; want k, nil", got, err)
	}
}
