// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package declarativejson decodes the JSON body of an orcon/declarative@1
// connector (ADR-0030 §1) into the types of internal/connector/declarative. It
// is a format adapter: the area package itself never imports encoding/json.
// Unknown members are refused, so a misspelled key cannot silently disable a
// rule of the connector.
package declarativejson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wirvii/orcon/internal/connector/declarative"
)

// Decode reads one connector body. It checks the shape only; call
// [declarative.Validate] (or [declarative.New]) before running it.
func Decode(data []byte) (declarative.Definition, error) {
	var def declarative.Definition
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&def); err != nil {
		return declarative.Definition{}, fmt.Errorf("declarativejson: %w", err)
	}
	if dec.More() {
		return declarative.Definition{}, errors.New("declarativejson: trailing data after the connector body")
	}
	return def, nil
}
