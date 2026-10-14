// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// symlink makes link (a slash path below the tree) point at target (another
// slash path below the tree) and skips the test where links are unavailable.
func (tr *tree) symlink(link, target string) {
	tr.t.Helper()
	linkPath := filepath.Join(tr.root, filepath.FromSlash(link))
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		tr.t.Fatal(err)
	}
	_ = os.Remove(linkPath) // replacing a file with a link; a missing file is fine
	if err := os.Symlink(filepath.Join(tr.root, filepath.FromSlash(target)), linkPath); err != nil {
		tr.t.Skipf("cannot create symbolic links here: %v", err)
	}
}

// TestVectorCheckCoversEveryBrandFile guards the ways a bad vector could hide
// from the BC-1 walk: a hidden name, a compressed file, a symbolic link, a
// vector with another extension and a renamed wired copy.
func TestVectorCheckCoversEveryBrandFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(tr *tree)
		want   []want
	}{
		{
			name:   "hidden SVG with an image",
			mutate: func(tr *tree) { tr.putFixture("brand/logo/.hidden.svg", "svg/image-lower.svg") },
			want: []want{
				{ruleVector, "brand/logo/.hidden.svg", "<image>"},
				{ruleManifest, "brand/logo/.hidden.svg", "not listed"},
			},
		},
		{
			name:   "SVG in a directory outside the asset directories",
			mutate: func(tr *tree) { tr.putFixture("brand/exports/art.svg", "svg/script.svg") },
			want: []want{
				{ruleVector, "brand/exports/art.svg", "<script>"},
				{ruleVector, "brand/exports/art.svg", `text "alert(1)"`},
			},
		},
		{
			name:   "compressed SVG",
			mutate: func(tr *tree) { tr.put("brand/logo/orcon.svgz", []byte{0x1f, 0x8b, 8}) },
			want: []want{
				{ruleVector, "brand/logo/orcon.svgz", "compressed SVG"},
				{ruleManifest, "brand/logo/orcon.svgz", "not listed"},
			},
		},
		{
			name: "upper case extension",
			mutate: func(tr *tree) {
				tr.putFixture("brand/logo/ORCON.SVG", "svg/foreignobject.svg")
			},
			want: []want{
				{ruleVector, "brand/logo/ORCON.SVG", "<foreignObject>"},
				{ruleVector, "brand/logo/ORCON.SVG", "element <div> is not in the SVG namespace"},
				{ruleVector, "brand/logo/ORCON.SVG", "namespace"},
				{ruleVector, "brand/logo/ORCON.SVG", `text "Orcon"`},
				{ruleManifest, "brand/logo/ORCON.SVG", "not listed"},
			},
		},
		{
			name: "asset of kind svg with a png name",
			mutate: func(tr *tree) {
				tr.m.Assets = append(tr.m.Assets, Asset{Path: "brand/png/disguised.png", Kind: kindSVG, Approval: Approval{Status: statusPending}})
				tr.saveManifest()
				tr.putFixture("brand/png/disguised.png", "svg/image-lower.svg")
			},
			want: []want{{ruleVector, "brand/png/disguised.png", "<image>"}},
		},
		{
			name: "copy of a vector under a non-svg name",
			mutate: func(tr *tree) {
				const renamed = "packages/js/ui/src/assets/brand/orcon-symbol.bin"
				tr.asset(symbolPath).Copies = []string{renamed}
				tr.saveManifest()
				tr.putFixture(renamed, "svg/clean.svg")
				tr.putFixture(symbolPath, "svg/clean.svg")
				tr.put(renamed, tr.read(symbolPath))
			},
		},
		{
			name: "copy of a vector under a non-svg name carrying an image",
			mutate: func(tr *tree) {
				const renamed = "packages/js/ui/src/assets/brand/orcon-symbol.bin"
				tr.asset(symbolPath).Copies = []string{renamed}
				tr.saveManifest()
				tr.putFixture(renamed, "svg/image-lower.svg")
			},
			want: []want{
				{ruleVector, "packages/js/ui/src/assets/brand/orcon-symbol.bin", "<image>"},
				{ruleCopies, "packages/js/ui/src/assets/brand/orcon-symbol.bin", "not byte-identical"},
			},
		},
		{
			name: "symbolic link to a vector inside brand",
			mutate: func(tr *tree) {
				tr.symlink("brand/logo/alias.svg", symbolPath)
			},
			want: []want{{ruleVector, "brand/logo/alias.svg", "symbolic link"}},
		},
		{
			name: "canonical asset replaced by a symbolic link",
			mutate: func(tr *tree) {
				tr.putFixture("elsewhere/solid-32.png", "raster/solid-32.png")
				tr.symlink(faviconPNG, "elsewhere/solid-32.png")
			},
			want: []want{
				{ruleVector, faviconPNG, "symbolic link"},
				{ruleManifest, faviconPNG, "symbolic link"},
			},
		},
		{
			name: "declared copy replaced by a symbolic link",
			mutate: func(tr *tree) {
				tr.symlink(faviconCopy, faviconICO)
			},
			want: []want{{ruleCopies, faviconCopy, "symbolic link"}},
		},
		{
			name: "symbolic link in an app directory to an unapproved asset",
			mutate: func(tr *tree) {
				tr.symlink("apps/web/public/fav.png", faviconPNG)
			},
			want: []want{{ruleManifest, "apps/web/public/fav.png", "copy of " + faviconPNG + ", which the owner has not approved"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := goodTree(t)
			tc.mutate(tr)
			requireExactly(t, tr.check(), tc.want...)
		})
	}
}
