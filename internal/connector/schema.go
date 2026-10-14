// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import (
	_ "embed" // the schema is part of the binary
	"slices"
)

//go:embed schema/orcon-connector-1.schema.json
var schema []byte

// Schema returns the JSON Schema (draft 2020-12) of orcon/connector@1, the same
// bytes as schema/orcon-connector-1.schema.json. The schema checks the shape of
// a manifest; [Read] is the validator the product runs and also checks the
// rules that need the whole document.
func Schema() []byte { return slices.Clone(schema) }
