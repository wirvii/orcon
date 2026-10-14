// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package configyaml is the YAML codec of the server configuration file
// (R-CONF-03). It turns the bytes of the file into the flat list of values that
// [config.Schema.Load] resolves, keeping the line of every key so orcon config
// check can say where a value came from, and it writes YAML for orcon config
// check -o yaml.
//
// It is an adapter package. It does not know which keys exist beyond the
// predicate it is given: unknown keys are reported by the loader, not here. The
// decoder reads untrusted input, so it refuses aliases, anchors, merge keys,
// duplicate keys and more than one document.
package configyaml
