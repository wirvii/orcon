// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readFixture returns the bytes of testdata/<rel>.
func readFixture(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	return data
}

// want describes one expected finding: rule and file must match exactly and
// the detail must contain sub.
type want struct{ rule, file, sub string }

// requireExactly fails unless got holds one finding for each want and nothing
// else. Strictness is the point: an unexpected extra finding is a bug in the
// rule or in the fixture.
func requireExactly(t *testing.T, got []Finding, wants ...want) {
	t.Helper()
	remaining := append([]Finding(nil), got...)
	for _, w := range wants {
		found := -1
		for i, f := range remaining {
			if f.Rule == w.rule && f.File == w.file && strings.Contains(f.Detail, w.sub) {
				found = i
				break
			}
		}
		if found < 0 {
			t.Errorf("missing finding %s %s containing %q; got:\n%s", w.rule, w.file, w.sub, render(got))
			continue
		}
		remaining = append(remaining[:found], remaining[found+1:]...)
	}
	if len(remaining) > 0 {
		t.Errorf("unexpected findings:\n%s", render(remaining))
	}
}

func render(fs []Finding) string {
	if len(fs) == 0 {
		return "  (none)"
	}
	var b strings.Builder
	for _, f := range fs {
		fmt.Fprintf(&b, "  %s\n", f)
	}
	return b.String()
}

// svgCase is a table row of the BC-1 tests: a fixture and, for a bad one, text
// that every reading of the finding list must contain.
type svgCase struct {
	name    string
	fixture string // file under testdata/svg, without extension
	reject  string // substring expected in a BC-1 finding; empty means the SVG is accepted
}

// runSVGCases checks each case against checkSVG: a rejected SVG yields only
// BC-1 findings and at least one mentions reject; an accepted SVG yields none.
func runSVGCases(t *testing.T, cases []svgCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file := tc.fixture + ".svg"
			got := checkSVG(file, readFixture(t, "svg/"+file))
			if tc.reject == "" {
				if len(got) != 0 {
					t.Fatalf("checkSVG(%s) found:\n%swant the SVG accepted", file, render(got))
				}
				return
			}
			if len(got) == 0 {
				t.Fatalf("checkSVG(%s) found nothing, want a finding containing %q", file, tc.reject)
			}
			matched := false
			for _, f := range got {
				if f.Rule != ruleVector || f.File != file || f.Line < 1 {
					t.Errorf("finding %+v: want rule %s, file %s, line >= 1", f, ruleVector, file)
				}
				matched = matched || strings.Contains(f.Detail, tc.reject)
			}
			if !matched {
				t.Errorf("checkSVG(%s) found:\n%swant one containing %q", file, render(got), tc.reject)
			}
		})
	}
}

// tree is a scratch repository root with a brand manifest.
type tree struct {
	t    *testing.T
	root string
	m    Manifest
}

func (tr *tree) put(rel string, data []byte) {
	tr.t.Helper()
	path := filepath.Join(tr.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tr.t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		tr.t.Fatal(err)
	}
}

func (tr *tree) putFixture(rel, fixture string) {
	tr.t.Helper()
	tr.put(rel, readFixture(tr.t, fixture))
}

func (tr *tree) remove(rel string) {
	tr.t.Helper()
	if err := os.Remove(filepath.Join(tr.root, filepath.FromSlash(rel))); err != nil {
		tr.t.Fatal(err)
	}
}

func (tr *tree) read(rel string) []byte {
	tr.t.Helper()
	data, err := os.ReadFile(filepath.Join(tr.root, filepath.FromSlash(rel)))
	if err != nil {
		tr.t.Fatal(err)
	}
	return data
}

// saveManifest writes tr.m to brand/manifest.json.
func (tr *tree) saveManifest() {
	tr.t.Helper()
	data, err := json.MarshalIndent(tr.m, "", "  ")
	if err != nil {
		tr.t.Fatal(err)
	}
	tr.put(manifestPath, data)
}

// writeReadme records the digests of the two boards as the real README does.
func (tr *tree) writeReadme() {
	tr.t.Helper()
	var b strings.Builder
	b.WriteString("# Brand\n\n| Board | File | SHA-256 |\n| --- | --- | --- |\n")
	for _, name := range []string{"primary", "secondary"} {
		board := tr.m.Boards[name]
		fmt.Fprintf(&b, "| %s | [`%s`](%s) | `%s` |\n", name, board.Path, board.Path, digest(tr.read(board.Path)))
	}
	tr.put(readmePath, []byte(b.String()))
}

// asset returns a pointer to the manifest entry for path.
func (tr *tree) asset(path string) *Asset {
	tr.t.Helper()
	for i := range tr.m.Assets {
		if tr.m.Assets[i].Path == path {
			return &tr.m.Assets[i]
		}
	}
	tr.t.Fatalf("asset %s is not in the manifest", path)
	return nil
}

// check runs every rule and fails the test on an input error.
func (tr *tree) check() []Finding {
	tr.t.Helper()
	got, _, err := checkAll(tr.root)
	if err != nil {
		tr.t.Fatalf("checkAll: %v", err)
	}
	return got
}

var approvedBy = Approval{Status: statusApproved, By: "owner", Date: "2026-10-14"}

// Paths of the assets in goodTree.
const (
	symbolPath   = "brand/logo/orcon-symbol.svg"
	symbolCopy   = "packages/js/ui/src/assets/brand/orcon-symbol.svg"
	faviconPNG   = "brand/png/favicon-32.png"
	faviconICO   = "brand/icon/favicon.ico"
	faviconCopy  = "apps/web/public/favicon.ico"
	mascotPath   = "brand/mascot/mascot-happy.png"
	mascotCopy   = "packages/js/ui/src/assets/mascot/mascot-happy.png"
	primaryBoard = "brand/reference/orcon-board-primary.png"
	secondBoard  = "brand/reference/orcon-board-secondary.png"
)

// goodTree builds a repository root where every rule passes: one SVG, one PNG,
// one ICO and one mascot cut-out, the wired ones approved and copied.
func goodTree(t *testing.T) *tree {
	t.Helper()
	tr := &tree{t: t, root: t.TempDir()}
	tr.putFixture(primaryBoard, "raster/board.png")
	tr.putFixture(secondBoard, "raster/solid-64.png")
	tr.m = Manifest{
		SchemaVersion: 1,
		Boards: map[string]Board{
			"primary":   {Path: primaryBoard, SHA256: digest(tr.read(primaryBoard))},
			"secondary": {Path: secondBoard, SHA256: digest(tr.read(secondBoard))},
		},
		Assets: []Asset{
			{
				Path: symbolPath, Kind: kindSVG, Copies: []string{symbolCopy}, Approval: approvedBy,
				Source:   &Source{Board: "primary", Region: []int{1, 1, 10, 10}},
				Fidelity: []Fidelity{{Name: "symbol", Region: []int{1, 1, 10, 10}, Mask: "brand-hue", IoUMin: 0.95}},
			},
			{Path: faviconPNG, Kind: kindPNG, Width: 32, Height: 32, From: symbolPath, Approval: Approval{Status: statusPending}},
			{Path: faviconICO, Kind: kindICO, Entries: []int{16, 32, 48}, Copies: []string{faviconCopy}, Approval: approvedBy},
			{
				Path: mascotPath, Kind: kindMascot, MaxBytes: 204800, Copies: []string{mascotCopy}, Approval: approvedBy,
				Source: &Source{Board: "primary", Region: []int{regionX, regionY, regionW, regionH}},
			},
		},
	}
	tr.putFixture(symbolPath, "svg/clean.svg")
	tr.putFixture(symbolCopy, "svg/clean.svg")
	tr.putFixture(faviconPNG, "raster/solid-32.png")
	tr.putFixture(faviconICO, "raster/icon-16-32-48.ico")
	tr.putFixture(faviconCopy, "raster/icon-16-32-48.ico")
	tr.putFixture(mascotPath, "raster/mascot-good.png")
	tr.putFixture(mascotCopy, "raster/mascot-good.png")
	tr.saveManifest()
	tr.writeReadme()
	return tr
}

// Region of the synthetic board that the mascot fixtures are cut from; it is
// the one tools/brandcheck/testdata/gen.go uses.
const regionX, regionY, regionW, regionH = 8, 6, 24, 20
