// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/test/isolation"
)

// editorOpener returns the opener of fresh backends for the store under test. Each backend is a new,
// migrated database that the case owns and that is removed when the case ends. The adapters are reached
// only through the storetest package, the declared exception for adapter wiring (AL-2.4).
func editorOpener(store isolation.Store) isolation.EditorOpener {
	return func(tb testing.TB) isolation.EditorBackend {
		tb.Helper()
		switch store {
		case isolation.StoreSQLite:
			return storetest.OpenSQLiteEditorStore(tb)
		case isolation.StorePostgres:
			return storetest.OpenPostgresEditorStore(tb)
		default:
			tb.Fatalf("isolation: no editor opener for store %q", store)
			return nil
		}
	}
}
