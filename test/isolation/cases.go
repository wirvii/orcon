// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

// BuildRegistry returns the registry with every case of the battery. A cut that
// adds a route or a store method writes its cases in a file named after the area
// (cases_store.go holds the store methods of identity, flow and execution) as a
// function that takes the registry, and calls it from here, so two cuts never
// edit the same lines.
func BuildRegistry() *Registry {
	r := NewRegistry()
	if err := addStoreCases(r); err != nil {
		panic("isolation: " + err.Error()) // a duplicate case is a mistake in this package, found by any test run
	}
	if err := addEditorCases(r); err != nil {
		panic("isolation: " + err.Error()) // the editor cases of SPEC-014 ED2, same rule as the store cases
	}
	if err := addCredentialCases(r); err != nil {
		panic("isolation: " + err.Error())
	}
	return r
}
