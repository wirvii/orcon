// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

// BuildRegistry returns the registry with every case of the battery. It is
// empty today: no route or store method handles organization data yet. A cut
// that adds one writes its cases in a file named after the area (for example
// cases_flow.go) as a function that takes the registry, and calls it from here,
// so two cuts never edit the same lines.
func BuildRegistry() *Registry {
	return EditorRegistry()
}

// EditorRegistry returns the registry of the visual editor storage cases.
func EditorRegistry() *Registry {
	reg := NewRegistry()
	if err := addEditorCases(reg); err != nil {
		// A duplicate or invalid case is a programming error in this package, not a run-time condition.
		panic(err)
	}
	return reg
}
