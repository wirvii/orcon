// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
)

// TestEditorSuite runs the visual editor storage battery (SPEC-014 ED2) on a fresh SQLite database per case.
func TestEditorSuite(t *testing.T) {
	storetest.RunEditorSuite(t, func(t *testing.T) storetest.EditorStore {
		return open(t, dbPath(t))
	})
}
