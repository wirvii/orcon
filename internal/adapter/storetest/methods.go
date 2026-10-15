// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"reflect"
	"sort"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
)

// infrastructureMethods are the exported methods of the stores that are not
// ports: they open, close and expose connections for the database rule checks.
//
// NotifyRunnable and ListenRunnable are the optional engine Notifier (LISTEN/NOTIFY
// wake-ups, SPEC-001 C4b): PostgreSQL implements it, SQLite has one process and no
// notifier. The notices carry no organization data (orcon:scope engine).
var infrastructureMethods = map[string]bool{
	"Close": true, "Pool": true, "Reader": true, "Writer": true,
	"NotifyRunnable": true, "ListenRunnable": true,
}

// PortMethodNames returns, for each adapter ("sqlite", "postgres"), the sorted names
// of the exported methods of its Store that implement a port of an area. The
// isolation battery compares them with the methods it has a case for, so a port
// method that an adapter gains cannot go without one (orcon-db TI-11).
func PortMethodNames() map[string][]string {
	names := func(v any) []string {
		t := reflect.TypeOf(v)
		var out []string
		for i := range t.NumMethod() {
			if name := t.Method(i).Name; !infrastructureMethods[name] {
				out = append(out, name)
			}
		}
		sort.Strings(out)
		return out
	}
	return map[string][]string{
		"sqlite":   names((*sqlite.Store)(nil)),
		"postgres": names((*postgres.Store)(nil)),
	}
}
