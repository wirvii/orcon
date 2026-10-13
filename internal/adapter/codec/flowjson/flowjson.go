// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flowjson

import (
	_ "embed" // the schema is part of the binary
	"fmt"
	"slices"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// SchemaID is the $id of the published JSON Schema.
const SchemaID = "urn:orcon:schema:flow:1"

//go:embed schema/orcon-flow-1.schema.json
var schema []byte

// Schema returns the JSON Schema (draft 2020-12) of orcon/flow@1, the same
// bytes as schema/orcon-flow-1.schema.json.
func Schema() []byte { return slices.Clone(schema) }

// Decode reads a flow document from JSON text. The text must be one valid JSON
// value without duplicate object keys. A document of an older format is
// upgraded in memory; the input is never changed. A document of an unknown or
// newer format fails with a [*flow.UnknownFormatError], and one with the wrong
// shape with a [*flow.ValidationError]. The returned document has the right
// shape but may still break the rules of [flow.Validate].
func Decode(data []byte) (flow.Document, error) {
	v, err := jsonvalue.Parse(data)
	if err != nil {
		return flow.Document{}, fmt.Errorf("decode flow document: %w", err)
	}
	doc, err := flow.Read(v)
	if err != nil {
		return flow.Document{}, fmt.Errorf("decode flow document: %w", err)
	}
	return doc, nil
}

// Encode writes a flow document as compact JSON in the member order of the
// format. Encoding the result of [Decode] gives back the same JSON data.
func Encode(doc *flow.Document) ([]byte, error) {
	data, err := jsonvalue.Marshal(doc.Value())
	if err != nil {
		return nil, fmt.Errorf("encode flow document: %w", err)
	}
	return data, nil
}

// Canonical returns the whole document, editor data included, in the JSON
// Canonicalization Scheme (RFC 8785). Two documents have the same canonical
// text exactly when they hold the same data, whatever the member order or
// number spelling. The content hash uses the same form without the editor.
func Canonical(doc *flow.Document) ([]byte, error) {
	data, err := jsonvalue.Canonical(doc.Value())
	if err != nil {
		return nil, fmt.Errorf("canonicalize flow document: %w", err)
	}
	return data, nil
}
