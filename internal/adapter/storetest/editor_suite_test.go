// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
)

// TestEditorSuite runs the visual editor storage battery (SPEC-014 ED2) on each selected store, on a fresh
// database per case. The adapter packages do not import this package (AL-2.4), so the battery lives here.
func TestEditorSuite(t *testing.T) {
	if wantSQLite() {
		t.Run("sqlite", func(t *testing.T) {
			storetest.RunEditorSuite(t, func(t *testing.T) storetest.EditorStore {
				return sqliteFactory()(t).(storetest.EditorStore)
			})
		})
	}
	if wantPostgres() {
		t.Run("postgres", func(t *testing.T) {
			storetest.RunEditorSuite(t, func(t *testing.T) storetest.EditorStore {
				return postgresFactory()(t).(storetest.EditorStore)
			})
		})
	}
}
