// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io/fs"

	"github.com/wirvii/orcon/internal/adapter/connectors/whatsapp"
	"github.com/wirvii/orcon/internal/adapter/httpapi"
)

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
