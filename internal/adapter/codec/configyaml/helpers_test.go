// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package configyaml_test

import "io/fs"

// memFS is a file system of named strings, enough for the loader.
type memFS map[string]string

func (m memFS) ReadFile(name string, _ int64) ([]byte, error) {
	d, ok := m[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return []byte(d), nil
}

func (m memFS) Stat(name string) (fs.FileInfo, error) {
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}
