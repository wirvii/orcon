// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"io/fs"
	"testing/fstest"
)

const spdxHeader = "-- SPDX-FileCopyrightText: The Orcon Authors\n-- SPDX-License-Identifier: Apache-2.0\n"

func header(floor string) string {
	return spdxHeader + "-- orcon:kind additive\n-- orcon:compatibility-floor " + floor + "\n"
}

func fstestFS(files map[string]string) fs.FS {
	m := fstest.MapFS{}
	for name, content := range files {
		m[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return m
}
