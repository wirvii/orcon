// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

// TestOfficialPackagesLoadTheirAssets: every official package built into the binary has an icon that
// passes the sanitizer and a source line, because the start fails otherwise.
func TestOfficialPackagesLoadTheirAssets(t *testing.T) {
	assets, err := officialConnectorAssets()
	if err != nil {
		t.Fatalf("official connector assets: %v", err)
	}
	if len(assets) != len(officialPackages) {
		t.Fatalf("%d assets for %d official packages", len(assets), len(officialPackages))
	}
	for _, p := range officialPackages {
		if assets[p.id].Icon == nil {
			t.Errorf("the official connector %s has no icon", p.id)
		}
	}
}
