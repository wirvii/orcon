// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/wirvii/orcon/internal/adapter/connectors/whatsapp"
	"github.com/wirvii/orcon/internal/adapter/httpapi"
	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
)

// packageManifestFile is the manifest of a connector package.
const packageManifestFile = "orcon-package.json"

// officialPackage is a connector package built into the binary: its id and the files of the
// package (manifest, catalogs and icon), embedded in the binary.
type officialPackage struct {
	id    string
	files fs.FS
}

// officialPackages are the official connector packages of this build. A package joins the list with
// the change that embeds it; its icon and catalogs then reach the node type catalog.
var officialPackages = []officialPackage{
	{id: "orcon.whatsapp", files: whatsapp.Files},
}

// officialConnectorAssets loads the icons and message catalogs of the official packages. An icon
// that is not safe to show inline fails the start, so it never reaches the API.
func officialConnectorAssets() (map[string]httpapi.ConnectorAssets, error) {
	out := make(map[string]httpapi.ConnectorAssets, len(officialPackages))
	for _, p := range officialPackages {
		assets, err := httpapi.LoadConnectorAssets(p.files)
		if err != nil {
			return nil, fmt.Errorf("load the assets of the connector %s: %w", p.id, err)
		}
		out[p.id] = assets
	}
	return out, nil
}

// registerOfficialPackages records every official package in the store of installed connectors, so the
// node type catalog and flow validation see it: the catalog lists what is installed, and an official
// connector has no archive to install, so the start records its manifest. Recording is idempotent; a
// package whose content changed under the same version fails the start, because a published version
// never changes (ADR-0034 section 5).
func registerOfficialPackages(ctx context.Context, packages connector.PackageStore, clk clock.Clock, ids id.Generator) error {
	registry := connector.NewRegistry(packages, nil, connector.OpenPolicy{}, clk, ids)
	for _, p := range officialPackages {
		manifest, err := fs.ReadFile(p.files, packageManifestFile)
		if err != nil {
			return fmt.Errorf("read the manifest of the connector %s: %w", p.id, err)
		}
		rec, err := registry.RegisterBuiltin(ctx, manifest)
		if errors.Is(err, connector.ErrConflict) {
			// Another replica that started together recorded it first: read what it wrote.
			rec, err = registry.RegisterBuiltin(ctx, manifest)
		}
		if err != nil {
			return fmt.Errorf("register the connector %s: %w", p.id, err)
		}
		if rec.ConnectorID != p.id {
			return fmt.Errorf("the package listed as %s declares the id %s", p.id, rec.ConnectorID)
		}
	}
	return nil
}
