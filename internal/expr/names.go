// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import "slices"

// contextNames are the identifiers the expression runtime defines for the
// n8n-compatible context (ADR-0018 section 1). The list is checked against the
// sandbox by the adapter tests, so it cannot drift from the prelude.
var contextNames = []string{
	"$", "$json", "$binary", "$input", "$node", "$now", "$today", "$execution", "$workflow",
	"$runIndex", "$itemIndex", "$position", "$prevNode", "$vars", "$env", "$parameter",
	"$jmespath", "$ifEmpty", "$if", "$min", "$max", "$items",
	"DateTime", "Duration", "Interval",
}

// globalNames are the standard JavaScript globals available in the sandbox
// (the ES2020 subset QuickJS implements, plus crypto and Intl, which the
// prelude provides).
var globalNames = []string{
	"undefined", "NaN", "Infinity", "globalThis",
	"Object", "Function", "Array", "Number", "Boolean", "String", "Symbol", "BigInt", "Date", "RegExp",
	"Error", "EvalError", "RangeError", "ReferenceError", "SyntaxError", "TypeError", "URIError",
	"AggregateError", "InternalError",
	"Math", "JSON", "Reflect", "Proxy", "Map", "Set", "WeakMap", "WeakSet", "Iterator", "DisposableStack",
	"SuppressedError", "DOMException",
	"ArrayBuffer", "SharedArrayBuffer", "DataView", "Int8Array", "Uint8Array", "Uint8ClampedArray",
	"Int16Array", "Uint16Array", "Int32Array", "Uint32Array", "Float16Array", "Float32Array", "Float64Array",
	"BigInt64Array", "BigUint64Array",
	"parseInt", "parseFloat", "isNaN", "isFinite", "eval", "queueMicrotask",
	"encodeURI", "encodeURIComponent", "decodeURI", "decodeURIComponent", "escape", "unescape",
	"atob", "btoa", "crypto", "Intl",
}

// ContextNames returns the identifiers of the expression context, sorted.
func ContextNames() []string { return slices.Sorted(slices.Values(contextNames)) }

// GlobalNames returns the JavaScript globals available to expressions, sorted.
func GlobalNames() []string { return slices.Sorted(slices.Values(globalNames)) }

// knownName reports whether an identifier resolves in the sandbox.
func knownName(name string) bool {
	return slices.Contains(contextNames, name) || slices.Contains(globalNames, name)
}

// knownMethod reports whether a method name is defined by JavaScript, Luxon or
// the supported extension methods (see names_gen.go).
func knownMethod(name string) bool {
	if _, ok := methodNames[name]; ok {
		return true
	}
	_, aliased := callAliases[name]
	return aliased || slices.Contains(extraMethods, name)
}

// inProto reports whether key is a property name of the prototype set, which
// the fast path leaves to the interpreter when the data lacks the property.
func inProto(set map[string]struct{}, key string) bool {
	_, ok := set[key]
	return ok
}
