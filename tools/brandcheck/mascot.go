// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// defaultMascotMaxBytes is the size ceiling of a mascot PNG (SPEC-018 R-BR-M-03).
const defaultMascotMaxBytes = 200 * 1024

// minOpaqueShare is the least share of fully opaque pixels in a cut-out.
const minOpaqueShare = 0.20

// boardImages decodes each reference board once.
type boardImages struct {
	root   string
	boards map[string]Board
	cache  map[string]boardResult
}

type boardResult struct {
	img image.Image
	err error
}

func newBoardImages(root string, boards map[string]Board) *boardImages {
	return &boardImages{root: root, boards: boards, cache: map[string]boardResult{}}
}

// get returns the decoded board name, or the reason it cannot be used.
func (b *boardImages) get(name string) (image.Image, error) {
	if r, ok := b.cache[name]; ok {
		return r.img, r.err
	}
	var r boardResult
	board, ok := b.boards[name]
	if !ok {
		r.err = fmt.Errorf("board %q is not declared", name)
	} else if data, err := readRel(b.root, board.Path); err != nil {
		r.err = fmt.Errorf("read board %s: %w", board.Path, err)
	} else if r.img, err = png.Decode(bytes.NewReader(data)); err != nil {
		r.err = fmt.Errorf("decode board %s: %w", board.Path, err)
	}
	b.cache[name] = r
	return r.img, r.err
}

// checkMascot applies BC-4 to a mascot cut-out: an RGBA PNG whose fully opaque
// pixels equal the board pixels at the same position, whose four corners are
// transparent, and that is mostly opaque and small enough. A redrawn mascot
// fails the pixel comparison even when it looks the same.
func checkMascot(a Asset, data []byte, boards *boardImages) []Finding {
	limit := defaultMascotMaxBytes
	if a.MaxBytes > 0 {
		limit = min(a.MaxBytes, defaultMascotMaxBytes)
	}
	var findings []Finding
	if len(data) > limit {
		findings = append(findings, whole(a.Path, ruleMascot, "PNG is %d bytes, over the %d byte limit", len(data), limit))
	}
	cut, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return append(findings, whole(a.Path, ruleMascot, "not a decodable PNG: %v", err))
	}
	switch cut.(type) {
	case *image.NRGBA, *image.NRGBA64, *image.RGBA, *image.RGBA64:
	default:
		return append(findings, whole(a.Path, ruleMascot, "PNG color type %T has no alpha channel, want RGBA", cut))
	}
	board, err := boards.get(a.Source.Board)
	if err != nil {
		return append(findings, whole(a.Path, ruleMascot, "%v", err))
	}
	r := a.Source.Region
	region := image.Rect(r[0], r[1], r[0]+r[2], r[1]+r[3])
	if !region.In(board.Bounds()) {
		return append(findings, whole(a.Path, ruleMascot, "source region %v is outside the %v board", region, board.Bounds().Size()))
	}
	cb := cut.Bounds()
	if cb.Dx() != r[2] || cb.Dy() != r[3] {
		return append(findings, whole(a.Path, ruleMascot, "PNG is %dx%d, the source region is %dx%d: cut-outs are not scaled", cb.Dx(), cb.Dy(), r[2], r[3]))
	}

	var opaque, differ int
	var first string
	for y := range cb.Dy() {
		for x := range cb.Dx() {
			c := nrgba(cut.At(cb.Min.X+x, cb.Min.Y+y))
			if c.A != 0xff {
				continue
			}
			opaque++
			want := nrgba(board.At(region.Min.X+x, region.Min.Y+y))
			if c.R != want.R || c.G != want.G || c.B != want.B {
				if differ == 0 {
					first = fmt.Sprintf("first at (%d,%d): cut-out #%02x%02x%02x, board #%02x%02x%02x", x, y, c.R, c.G, c.B, want.R, want.G, want.B)
				}
				differ++
			}
		}
	}
	if differ > 0 {
		findings = append(findings, whole(a.Path, ruleMascot, "%d opaque pixels differ from the board crop (%s): the cut-out must be the board pixels, not a redraw", differ, first))
	}
	for _, p := range [][2]int{{0, 0}, {cb.Dx() - 1, 0}, {0, cb.Dy() - 1}, {cb.Dx() - 1, cb.Dy() - 1}} {
		if alpha := nrgba(cut.At(cb.Min.X+p[0], cb.Min.Y+p[1])).A; alpha != 0 {
			findings = append(findings, whole(a.Path, ruleMascot, "corner (%d,%d) has alpha %d, want 0: the background was not removed", p[0], p[1], alpha))
		}
	}
	if share := float64(opaque) / float64(cb.Dx()*cb.Dy()); share < minOpaqueShare {
		findings = append(findings, whole(a.Path, ruleMascot, "only %.1f%% of the pixels are opaque, want at least %.0f%%", share*100, minOpaqueShare*100))
	}
	return findings
}

// nrgba converts any color to 8-bit non-premultiplied RGBA.
func nrgba(c color.Color) color.NRGBA {
	n, _ := color.NRGBAModel.Convert(c).(color.NRGBA) // the model always returns a color.NRGBA
	return n
}
