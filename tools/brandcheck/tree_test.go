// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func TestGoodTreeHasNoFindings(t *testing.T) {
	t.Parallel()
	requireExactly(t, goodTree(t).check())
}

func TestCopiesMustBeByteIdentical(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(tr *tree)
		want   []want
	}{
		{name: "identical copies", mutate: func(*tree) {}},
		{
			name: "svg copy differs by one byte",
			mutate: func(tr *tree) {
				tr.put(symbolCopy, append(tr.read(symbolCopy), ' '))
			},
			want: []want{{ruleCopies, symbolCopy, "not byte-identical to " + symbolPath}},
		},
		{
			name: "mascot copy is a different image",
			mutate: func(tr *tree) {
				tr.putFixture(mascotCopy, "raster/mascot-soft-edge.png")
			},
			want: []want{{ruleCopies, mascotCopy, "not byte-identical to " + mascotPath}},
		},
		{
			name:   "declared copy does not exist",
			mutate: func(tr *tree) { tr.remove(faviconCopy) },
			want:   []want{{ruleCopies, faviconCopy, "does not exist"}},
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

func TestRasterSizesMatchManifest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(tr *tree)
		want   []want
	}{
		{name: "PNG has the declared size", mutate: func(*tree) {}},
		{
			name: "PNG is smaller than declared",
			mutate: func(tr *tree) {
				tr.putFixture(faviconPNG, "raster/solid-16.png")
			},
			want: []want{{ruleSizes, faviconPNG, "PNG is 16x16, the manifest declares 32x32"}},
		},
		{
			name: "manifest declares a different size",
			mutate: func(tr *tree) {
				tr.asset(faviconPNG).Width = 48
				tr.saveManifest()
			},
			want: []want{{ruleSizes, faviconPNG, "PNG is 32x32, the manifest declares 48x32"}},
		},
		{
			name:   "file is not a PNG",
			mutate: func(tr *tree) { tr.put(faviconPNG, []byte("GIF89a not a png")) },
			want:   []want{{ruleSizes, faviconPNG, "not a decodable PNG"}},
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

func TestICOHasExactEntries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		fixture string
		want    []want
	}{
		{name: "16, 32 and 48", fixture: "raster/icon-16-32-48.ico"},
		{
			name: "48 is missing", fixture: "raster/icon-16-32.ico",
			want: []want{{ruleSizes, faviconICO, "ICO entries are [16 32], the manifest declares exactly [16 32 48]"}},
		},
		{
			name: "an extra 64 entry", fixture: "raster/icon-16-32-48-64.ico",
			want: []want{{ruleSizes, faviconICO, "ICO entries are [16 32 48 64], the manifest declares exactly [16 32 48]"}},
		},
		{
			name: "truncated file", fixture: "raster/icon-truncated.ico",
			want: []want{{ruleSizes, faviconICO, "invalid ICO"}},
		},
		{
			name: "not an icon at all", fixture: "raster/solid-16.png",
			want: []want{{ruleSizes, faviconICO, "invalid ICO"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := goodTree(t)
			// The wired copy follows the canonical file so only BC-3 is exercised.
			tr.putFixture(faviconICO, tc.fixture)
			tr.putFixture(faviconCopy, tc.fixture)
			requireExactly(t, tr.check(), tc.want...)
		})
	}
}

func TestMascotOpaquePixelsComeFromBoard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		fixture string
		mutate  func(tr *tree)
		want    []want
	}{
		{name: "board pixels over a transparent background", fixture: "raster/mascot-good.png"},
		{name: "soft edge pixels may change color", fixture: "raster/mascot-soft-edge.png"},
		{name: "16-bit RGBA widened exactly (v*257)", fixture: "raster/mascot-16bit.png"},
		{
			name: "redraw hidden behind alpha 254", fixture: "raster/mascot-semi-redraw.png",
			want: []want{{ruleMascot, mascotPath, "158 visible pixels (alpha > 0) differ from the board crop"}},
		},
		{
			name: "16-bit channels shifted instead of widened", fixture: "raster/mascot-16bit-shifted.png",
			want: []want{{ruleMascot, mascotPath, "visible pixels (alpha > 0) differ from the board crop"}},
		},
		{
			name: "16-bit alpha 0xff00 is not opaque", fixture: "raster/mascot-16bit-alpha.png",
			want: []want{{ruleMascot, mascotPath, "0.0% of the pixels are fully opaque"}},
		},
		{
			name: "palette image with tRNS", fixture: "raster/mascot-palette-trns.png",
			want: []want{{ruleMascot, mascotPath, "PNG color type 3: cut-outs must be RGBA"}},
		},
		{
			name: "one opaque pixel was redrawn", fixture: "raster/mascot-bad-pixel.png",
			want: []want{{ruleMascot, mascotPath, "1 visible pixels (alpha > 0) differ from the board crop"}},
		},
		{
			name: "wrong size is not a crop of the region", fixture: "raster/mascot-wrong-size.png",
			want: []want{{ruleMascot, mascotPath, "PNG is 10x10, the source region is 24x20"}},
		},
		{
			name: "no alpha channel", fixture: "raster/mascot-no-alpha.png",
			want: []want{{ruleMascot, mascotPath, "PNG color type 0"}},
		},
		{
			name: "region outside the board", fixture: "raster/mascot-good.png",
			mutate: func(tr *tree) {
				tr.asset(mascotPath).Source.Region = []int{50, 40, regionW, regionH}
				tr.saveManifest()
			},
			want: []want{{ruleMascot, mascotPath, "outside the"}},
		},
		{
			name: "region shifted by one pixel no longer matches", fixture: "raster/mascot-good.png",
			mutate: func(tr *tree) {
				tr.asset(mascotPath).Source.Region = []int{regionX + 1, regionY, regionW, regionH}
				tr.saveManifest()
			},
			want: []want{{ruleMascot, mascotPath, "visible pixels (alpha > 0) differ from the board crop"}},
		},
		{
			name: "not a PNG", fixture: "raster/icon-16-32.ico",
			want: []want{{ruleMascot, mascotPath, "not a decodable PNG"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := goodTree(t)
			tr.putFixture(mascotPath, tc.fixture)
			tr.putFixture(mascotCopy, tc.fixture)
			if tc.mutate != nil {
				tc.mutate(tr)
			}
			requireExactly(t, tr.check(), tc.want...)
		})
	}
}

func TestMascotCornersTransparent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		fixture string
		mutate  func(tr *tree)
		want    []want
	}{
		{name: "all four corners transparent", fixture: "raster/mascot-good.png"},
		{
			name: "an opaque board pixel in a corner", fixture: "raster/mascot-opaque-corner.png",
			want: []want{{ruleMascot, mascotPath, "corner (0,0) has alpha 65535"}},
		},
		{
			name: "too few opaque pixels", fixture: "raster/mascot-sparse.png",
			want: []want{{ruleMascot, mascotPath, "of the pixels are fully opaque, want at least 20%"}},
		},
		{
			name: "file over the size limit", fixture: "raster/mascot-good.png",
			mutate: func(tr *tree) {
				tr.asset(mascotPath).MaxBytes = 100
				tr.saveManifest()
			},
			want: []want{{ruleMascot, mascotPath, "over the 100 byte limit"}},
		},
		{
			name: "manifest cannot raise the limit above 200 KiB", fixture: "raster/mascot-good.png",
			mutate: func(tr *tree) {
				tr.put(mascotPath, append(tr.read(mascotPath), make([]byte, 205000)...))
				tr.put(mascotCopy, tr.read(mascotPath))
				tr.asset(mascotPath).MaxBytes = 1 << 20
				tr.saveManifest()
			},
			want: []want{
				{ruleMascot, mascotPath, "over the 204800 byte limit"},
				{ruleMascot, mascotPath, "205000 bytes after the IEND chunk"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := goodTree(t)
			tr.putFixture(mascotPath, tc.fixture)
			tr.putFixture(mascotCopy, tc.fixture)
			if tc.mutate != nil {
				tc.mutate(tr)
			}
			requireExactly(t, tr.check(), tc.want...)
		})
	}
}

func TestBoardsUnchanged(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(tr *tree)
		want   []want
	}{
		{name: "boards match manifest and README", mutate: func(*tree) {}},
		{
			name: "board file edited after the hashes were recorded",
			mutate: func(tr *tree) {
				tr.put(primaryBoard, append(tr.read(primaryBoard), 0))
			},
			want: []want{
				{ruleBoards, primaryBoard, "primary board changed"},
				{ruleBoards, readmePath, "records sha256"},
			},
		},
		{
			name: "README records a stale hash",
			mutate: func(tr *tree) {
				readme := strings.Replace(string(tr.read(readmePath)), digest(tr.read(secondBoard)), strings.Repeat("0", 64), 1)
				tr.put(readmePath, []byte(readme))
			},
			want: []want{{ruleBoards, readmePath, "records sha256 " + strings.Repeat("0", 64)}},
		},
		{
			name: "README does not mention a board",
			mutate: func(tr *tree) {
				tr.put(readmePath, []byte("# Brand\n"))
			},
			want: []want{
				{ruleBoards, readmePath, "does not record a sha256 for orcon-board-primary.png"},
				{ruleBoards, readmePath, "does not record a sha256 for orcon-board-secondary.png"},
			},
		},
		{
			name:   "README is missing",
			mutate: func(tr *tree) { tr.remove(readmePath) },
			want:   []want{{ruleBoards, readmePath, "cannot read the README"}},
		},
		{
			name:   "board file is missing",
			mutate: func(tr *tree) { tr.remove(secondBoard) },
			want:   []want{{ruleBoards, secondBoard, "cannot read the secondary board"}},
		},
		{
			name: "manifest hash is stale",
			mutate: func(tr *tree) {
				board := tr.m.Boards["primary"]
				board.SHA256 = strings.Repeat("a", 64)
				tr.m.Boards["primary"] = board
				tr.saveManifest()
			},
			want: []want{{ruleBoards, primaryBoard, "the manifest records " + strings.Repeat("a", 64)}},
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

func TestEveryAssetIsListed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(tr *tree)
		want   []want
	}{
		{name: "everything listed", mutate: func(*tree) {}},
		{
			name:   "SVG dropped into brand/logo without a manifest entry",
			mutate: func(tr *tree) { tr.putFixture("brand/logo/extra.svg", "svg/clean.svg") },
			want:   []want{{ruleManifest, "brand/logo/extra.svg", "not listed"}},
		},
		{
			name:   "PNG dropped into a nested mascot directory",
			mutate: func(tr *tree) { tr.putFixture("brand/mascot/old/pose.png", "raster/mascot-good.png") },
			want:   []want{{ruleManifest, "brand/mascot/old/pose.png", "not listed"}},
		},
		{
			name:   "hidden files are assets too",
			mutate: func(tr *tree) { tr.put("brand/icon/.DS_Store", []byte{0}) },
			want:   []want{{ruleManifest, "brand/icon/.DS_Store", "not listed"}},
		},
		{
			name:   "files outside the asset directories are not assets",
			mutate: func(tr *tree) { tr.put("brand/exports/banner.png", []byte{1}) },
		},
		{
			name:   "listed asset is missing from disk",
			mutate: func(tr *tree) { tr.remove(faviconPNG) },
			want:   []want{{ruleManifest, faviconPNG, "does not exist"}},
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

func TestWiredAssetsMustBeApproved(t *testing.T) {
	t.Parallel()
	unapproved := func(path, status string) func(tr *tree) {
		return func(tr *tree) {
			tr.asset(path).Approval = Approval{Status: status}
			tr.saveManifest()
		}
	}
	tests := []struct {
		name   string
		mutate func(tr *tree)
		want   []want
	}{
		{name: "wired and approved", mutate: func(*tree) {}},
		{name: "pending asset without copies is not wired", mutate: unapproved(faviconPNG, statusPending)},
		{
			name:   "wired asset still pending",
			mutate: unapproved(symbolPath, statusPending),
			want:   []want{{ruleManifest, symbolPath, "not approved by the owner"}},
		},
		{
			name:   "wired asset the owner rejected",
			mutate: unapproved(mascotPath, statusRejected),
			want:   []want{{ruleManifest, mascotPath, `approval.status="rejected"`}},
		},
		{
			name: "approved without who and when is not an approval",
			mutate: func(tr *tree) {
				tr.asset(faviconICO).Approval = Approval{Status: statusApproved}
				tr.saveManifest()
			},
			want: []want{
				{ruleManifest, manifestPath, "approved without approval.by"},
				{ruleManifest, manifestPath, "approved without a YYYY-MM-DD approval.date"},
			},
		},
		{
			name: "unapproved asset copied into an app directory without being declared",
			mutate: func(tr *tree) {
				tr.putFixture("apps/web/public/favicon-32.png", "raster/solid-32.png")
			},
			want: []want{{ruleManifest, "apps/web/public/favicon-32.png", "copy of " + faviconPNG + ", which the owner has not approved"}},
		},
		{
			name: "unrelated app file is not flagged",
			mutate: func(tr *tree) {
				tr.put("apps/web/public/robots.txt", []byte("User-agent: *\n"))
			},
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

func TestMissingManifestChecksOnlySVGsAndBoards(t *testing.T) {
	t.Parallel()
	boardsAndReadme := func(t *testing.T) *tree {
		t.Helper()
		tr := goodTree(t)
		tr.remove(manifestPath)
		return tr
	}
	tests := []struct {
		name   string
		mutate func(tr *tree)
		want   []want
	}{
		{name: "clean tree passes"},
		{
			name: "assets that would fail the manifest rules are not looked at",
			mutate: func(tr *tree) {
				tr.putFixture("brand/png/unlisted.png", "raster/solid-16.png")
				tr.put(symbolCopy, []byte("differs"))
			},
		},
		{
			name:   "bad SVG is still rejected",
			mutate: func(tr *tree) { tr.putFixture("brand/logo/with-image.svg", "svg/image-lower.svg") },
			want:   []want{{ruleVector, "brand/logo/with-image.svg", "<image>"}},
		},
		{
			name:   "stale README board hash is still rejected",
			mutate: func(tr *tree) { tr.put(primaryBoard, []byte("changed")) },
			want:   []want{{ruleBoards, readmePath, "records sha256"}},
		},
		{
			name: "README without board hashes is rejected",
			mutate: func(tr *tree) {
				tr.put(readmePath, []byte("# Brand\n"))
			},
			want: []want{
				{ruleBoards, readmePath, "does not record a sha256 for orcon-board-primary.png"},
				{ruleBoards, readmePath, "does not record a sha256 for orcon-board-secondary.png"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := boardsAndReadme(t)
			if tc.mutate != nil {
				tc.mutate(tr)
			}
			got, notes, err := checkAll(tr.root)
			if err != nil {
				t.Fatalf("checkAll: %v", err)
			}
			requireExactly(t, got, tc.want...)
			if len(notes) != 1 || !strings.Contains(notes[0], "no brand/manifest.json") {
				t.Errorf("notes = %q, want one note saying there is no brand/manifest.json", notes)
			}
		})
	}
}

func TestManifestMustBeWellFormed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(tr *tree)
		raw    string // when set, replaces the manifest file
		sub    string
	}{
		{name: "not JSON", raw: "{", sub: "invalid manifest JSON"},
		{name: "trailing content", raw: `{"schema_version":1} {}`, sub: "trailing content"},
		{name: "unknown field", raw: `{"schema_version":1,"asset":[]}`, sub: "unknown field"},
		{name: "wrong schema version", mutate: func(tr *tree) { tr.m.SchemaVersion = 2 }, sub: "schema_version is 2"},
		{name: "missing board", mutate: func(tr *tree) { delete(tr.m.Boards, "secondary") }, sub: "boards.secondary is missing"},
		{name: "bad board hash", mutate: func(tr *tree) { tr.m.Boards["primary"] = Board{Path: primaryBoard, SHA256: "xyz"} }, sub: "boards.primary.sha256"},
		{name: "path escapes brand", mutate: func(tr *tree) { tr.m.Assets[0].Path = "../etc/passwd" }, sub: "not a clean relative path below brand/"},
		{name: "duplicate asset", mutate: func(tr *tree) { tr.m.Assets = append(tr.m.Assets, tr.m.Assets[0]) }, sub: "listed twice"},
		{name: "unknown kind", mutate: func(tr *tree) { tr.m.Assets[0].Kind = "gif" }, sub: `kind "gif"`},
		{name: "png without size", mutate: func(tr *tree) { tr.m.Assets[1].Width = 0 }, sub: "positive width and height"},
		{name: "ico without entries", mutate: func(tr *tree) { tr.m.Assets[2].Entries = nil }, sub: "non-empty entries"},
		{name: "mascot without source", mutate: func(tr *tree) { tr.m.Assets[3].Source = nil }, sub: "needs a source board region"},
		{name: "source board not declared", mutate: func(tr *tree) { tr.m.Assets[3].Source.Board = "tertiary" }, sub: `source board "tertiary"`},
		{name: "region of three numbers", mutate: func(tr *tree) { tr.m.Assets[3].Source.Region = []int{1, 2, 3} }, sub: "region must be [x, y, width, height]"},
		{name: "fidelity mask unknown", mutate: func(tr *tree) { tr.m.Assets[0].Fidelity[0].Mask = "sepia" }, sub: `mask "sepia"`},
		{name: "fidelity floor above one", mutate: func(tr *tree) { tr.m.Assets[0].Fidelity[0].IoUMin = 1.5 }, sub: "iou_min 1.5"},
		{name: "copy equals the asset", mutate: func(tr *tree) { tr.m.Assets[0].Copies = []string{symbolPath} }, sub: "distinct from the asset"},
		{name: "unknown approval status", mutate: func(tr *tree) { tr.m.Assets[0].Approval.Status = "maybe" }, sub: `approval.status "maybe"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := goodTree(t)
			if tc.mutate != nil {
				tc.mutate(tr)
				tr.saveManifest()
			} else {
				tr.put(manifestPath, []byte(tc.raw))
			}
			got := tr.check()
			// An invalid manifest stops the manifest rules: the findings are all about it.
			if len(got) == 0 {
				t.Fatalf("checkAll found nothing, want a manifest finding containing %q", tc.sub)
			}
			for _, f := range got {
				if f.Rule != ruleManifest || f.File != manifestPath {
					t.Errorf("finding %v: want only %s findings on %s", f, ruleManifest, manifestPath)
				}
			}
			if !strings.Contains(render(got), tc.sub) {
				t.Errorf("findings:\n%swant one containing %q", render(got), tc.sub)
			}
		})
	}
}

func TestMascotRejectsColorManagementChunks(t *testing.T) {
	t.Parallel()
	for _, chunk := range []string{"gAMA", "iCCP", "sRGB", "cHRM", "cICP"} {
		t.Run(chunk, func(t *testing.T) {
			t.Parallel()
			tr := goodTree(t)
			managed := injectChunk(t, readFixture(t, "raster/mascot-good.png"), chunk, []byte{0, 0, 0x4e, 0x20})
			tr.put(mascotPath, managed)
			tr.put(mascotCopy, managed)
			requireExactly(t, tr.check(), want{ruleMascot, mascotPath, "carries a " + chunk + " chunk"})
		})
	}
}

func TestREADMEDigestMustBelongToTheBoardLine(t *testing.T) {
	t.Parallel()
	zeros := strings.Repeat("0", 64)
	ones := strings.Repeat("1", 64)
	tests := []struct {
		name   string
		readme func(tr *tree, valid string) string
		want   []want
	}{
		{name: "valid README", readme: func(_ *tree, valid string) string { return valid }},
		{
			name:   "longer file name with another digest before",
			readme: func(_ *tree, valid string) string { return "orcon-board-primary.png.bak " + zeros + "\n" + valid },
		},
		{
			name:   "prefixed file name with another digest before",
			readme: func(_ *tree, valid string) string { return "old-orcon-board-primary.png " + zeros + "\n" + valid },
		},
		{
			name: "digest in upper case",
			readme: func(tr *tree, valid string) string {
				return strings.Replace(valid, digest(tr.read(primaryBoard)), strings.ToUpper(digest(tr.read(primaryBoard))), 1)
			},
		},
		{
			name: "same board recorded twice with different digests",
			readme: func(_ *tree, valid string) string {
				return valid + "| Primary | `orcon-board-primary.png` | `" + zeros + "` |\n"
			},
			want: []want{{ruleBoards, readmePath, "more than one sha256 on the lines that name orcon-board-primary.png"}},
		},
		{
			name: "two digests on the board line",
			readme: func(_ *tree, valid string) string {
				return valid + "orcon-board-secondary.png " + zeros + " " + ones + "\n"
			},
			want: []want{{ruleBoards, readmePath, "more than one sha256 on the lines that name orcon-board-secondary.png"}},
		},
		{
			name: "a 65 digit run is not a digest",
			readme: func(tr *tree, valid string) string {
				return strings.Replace(valid, digest(tr.read(secondBoard)), "a"+digest(tr.read(secondBoard)), 1)
			},
			want: []want{{ruleBoards, readmePath, "does not record a sha256 for orcon-board-secondary.png"}},
		},
		{
			name: "only a longer file name carries the digest",
			readme: func(tr *tree, _ string) string {
				return "orcon-board-primary.png.bak " + digest(tr.read(primaryBoard)) + "\norcon-board-secondary.png " + digest(tr.read(secondBoard)) + "\n"
			},
			want: []want{{ruleBoards, readmePath, "does not record a sha256 for orcon-board-primary.png"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := goodTree(t)
			tr.put(readmePath, []byte(tc.readme(tr, string(tr.read(readmePath)))))
			requireExactly(t, tr.check(), tc.want...)
		})
	}
}

func TestMascotRejectsTextChunksAndTrailingBytes(t *testing.T) {
	t.Parallel()
	good := readFixture(t, "raster/mascot-good.png")
	tests := []struct {
		name string
		png  []byte
		sub  string
	}{
		{"tEXt", injectChunk(t, good, "tEXt", []byte("Comment\x00hidden")), "tEXt text chunk"},
		{"zTXt", injectChunk(t, good, "zTXt", []byte("Comment\x00\x00x")), "zTXt text chunk"},
		{"iTXt", injectChunk(t, good, "iTXt", []byte("Comment\x00\x00\x00\x00\x00hidden")), "iTXt text chunk"},
		{"payload after IEND", append(append([]byte(nil), good...), []byte("PK-hidden-archive")...), "17 bytes after the IEND chunk"},
		{"one byte after IEND", append(append([]byte(nil), good...), 0), "1 bytes after the IEND chunk"},
		{"truncated before IEND", good[:len(good)-12], "no IEND chunk"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := goodTree(t)
			tr.put(mascotPath, tc.png)
			tr.put(mascotCopy, tc.png)
			requireExactly(t, tr.check(), want{ruleMascot, mascotPath, tc.sub})
		})
	}
}
