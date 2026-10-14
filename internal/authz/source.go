// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import _ "embed" // permissions.yaml is embedded as bytes; the codec adapter parses it.

//go:embed permissions.yaml
var catalogSource []byte

// CatalogSource returns the bytes of permissions.yaml, the source of the
// permission catalog (ADR-0041 section 2). Areas do not parse YAML (AL-2.1):
// the composition root hands these bytes to the codec adapter and builds the
// catalog with [NewCatalog].
func CatalogSource() []byte {
	out := make([]byte, len(catalogSource))
	copy(out, catalogSource)
	return out
}
