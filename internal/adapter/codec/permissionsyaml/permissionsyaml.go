// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package permissionsyaml is the YAML codec of the permission catalog file
// internal/authz/permissions.yaml (ADR-0041 section 2).
//
// It is an adapter package. It checks the shape of the file, which is the
// strict format the contract lint also reads: version 1 and the scopes
// organization, project and instance, each a list of items with a name. The
// rules of a valid catalog live in [authz.NewCatalog].
package permissionsyaml

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"

	"github.com/wirvii/orcon/internal/authz"
)

// fileVersion is the only version of the file format.
const fileVersion = 1

// ErrInvalid means the file does not follow the strict format.
var ErrInvalid = errors.New("invalid permissions file")

type file struct {
	Version     int                         `yaml:"version"`
	Permissions map[string][]permissionItem `yaml:"permissions"`
}

type permissionItem struct {
	Name string `yaml:"name"`
	Note string `yaml:"note"`
}

// Parse reads the catalog from the bytes of permissions.yaml. Unknown keys, an
// unknown scope, a wrong version or an item without a valid name are errors;
// nothing is ignored.
func Parse(data []byte) (*authz.Catalog, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var f file
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: the file is empty", ErrInvalid)
		}
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if f.Version != fileVersion {
		return nil, fmt.Errorf("%w: version must be %d, found %d", ErrInvalid, fileVersion, f.Version)
	}
	byScope := map[authz.Scope][]authz.Permission{}
	for scope, items := range f.Permissions {
		s := authz.Scope(scope)
		for i, item := range items {
			if item.Name == "" {
				return nil, fmt.Errorf("%w: permissions.%s[%d] needs a name", ErrInvalid, scope, i)
			}
			byScope[s] = append(byScope[s], authz.Permission(item.Name))
		}
	}
	cat, err := authz.NewCatalog(byScope)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return cat, nil
}
