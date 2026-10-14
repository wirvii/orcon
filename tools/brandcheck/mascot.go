// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// defaultMascotMaxBytes is the size ceiling of a mascot PNG (SPEC-018 R-BR-M-03).
const defaultMascotMaxBytes = 200 * 1024

// pngColorRGBA is the PNG color type of truecolor with alpha.
const pngColorRGBA = 6

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

// colorChunks are the PNG chunks that change how pixel values are displayed. A
// cut-out carrying one would not look like the board even with equal pixels.
var colorChunks = map[string]bool{"gAMA": true, "iCCP": true, "sRGB": true, "cHRM": true, "cICP": true}

// pngHeader is what checkMascot needs from the PNG chunk structure.
type pngHeader struct {
	colorType  byte
	colorChunk string // first color-management chunk found, if any
}

// readPNGHeader walks the chunk list of a PNG and reports the color type and
// the first color-management chunk. A stream that is not a well-formed chunk
// list is an error.
func readPNGHeader(data []byte) (pngHeader, error) {
	var h pngHeader
	if !bytes.HasPrefix(data, pngSignature) {
		return h, errors.New("missing PNG signature")
	}
	rest := data[len(pngSignature):]
	seenHeader := false
	for len(rest) >= 12 {
		length := int(binary.BigEndian.Uint32(rest))
		kind := string(rest[4:8])
		if length < 0 || len(rest) < 12+length {
			return h, fmt.Errorf("chunk %s is truncated", kind)
		}
		body := rest[8 : 8+length]
		if kind == "IHDR" && length == 13 {
			h.colorType = body[9]
			seenHeader = true
		}
		if colorChunks[kind] && h.colorChunk == "" {
			h.colorChunk = kind
		}
		if kind == "IEND" {
			break
		}
		rest = rest[12+length:]
	}
	if !seenHeader {
		return h, errors.New("no IHDR chunk")
	}
	return h, nil
}

// checkMascot applies BC-4 to a mascot cut-out: an RGBA PNG (color type 6, 8 or
// 16 bit, without color-management chunks) where every pixel that is not fully
// transparent has exactly the RGB of the board pixel at the same position, the
// four corners are transparent, at least 20 % of the pixels are fully opaque
// and the file is small enough. A redrawn mascot fails the pixel comparison
// even when it looks the same, and hiding a redraw behind almost-opaque alpha
// fails too. Fully opaque means the maximum alpha of the file's bit depth.
func checkMascot(a Asset, data []byte, boards *boardImages) []Finding {
	limit := defaultMascotMaxBytes
	if a.MaxBytes > 0 {
		limit = min(a.MaxBytes, defaultMascotMaxBytes)
	}
	var findings []Finding
	if len(data) > limit {
		findings = append(findings, whole(a.Path, ruleMascot, "PNG is %d bytes, over the %d byte limit", len(data), limit))
	}
	head, err := readPNGHeader(data)
	if err != nil {
		return append(findings, whole(a.Path, ruleMascot, "not a decodable PNG: %v", err))
	}
	if head.colorType != pngColorRGBA {
		return append(findings, whole(a.Path, ruleMascot,
			"PNG color type %d: cut-outs must be RGBA (color type 6, 8 or 16 bit); palette, grayscale and RGB images are not accepted, even with a tRNS chunk",
			head.colorType))
	}
	if head.colorChunk != "" {
		findings = append(findings, whole(a.Path, ruleMascot,
			"PNG carries a %s chunk: color-management chunks change how the pixels display, so the cut-out would not match the board", head.colorChunk))
	}
	cut, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return append(findings, whole(a.Path, ruleMascot, "not a decodable PNG: %v", err))
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
			c := pixel(cut, cb.Min.X+x, cb.Min.Y+y)
			if c.A == 0 {
				continue
			}
			if c.A == 0xffff {
				opaque++
			}
			want := pixel(board, region.Min.X+x, region.Min.Y+y)
			if c.R != want.R || c.G != want.G || c.B != want.B {
				if differ == 0 {
					first = fmt.Sprintf("first at (%d,%d): cut-out #%04x%04x%04x alpha %04x, board #%04x%04x%04x", x, y, c.R, c.G, c.B, c.A, want.R, want.G, want.B)
				}
				differ++
			}
		}
	}
	if differ > 0 {
		findings = append(findings, whole(a.Path, ruleMascot, "%d visible pixels (alpha > 0) differ from the board crop (%s): the cut-out must be the board pixels, not a redraw", differ, first))
	}
	for _, p := range [][2]int{{0, 0}, {cb.Dx() - 1, 0}, {0, cb.Dy() - 1}, {cb.Dx() - 1, cb.Dy() - 1}} {
		if alpha := pixel(cut, cb.Min.X+p[0], cb.Min.Y+p[1]).A; alpha != 0 {
			findings = append(findings, whole(a.Path, ruleMascot, "corner (%d,%d) has alpha %d, want 0: the background was not removed", p[0], p[1], alpha))
		}
	}
	if share := float64(opaque) / float64(cb.Dx()*cb.Dy()); share < minOpaqueShare {
		findings = append(findings, whole(a.Path, ruleMascot, "only %.1f%% of the pixels are fully opaque, want at least %.0f%%", share*100, minOpaqueShare*100))
	}
	return findings
}

// pixel returns the pixel at (x, y) as 16-bit non-premultiplied RGBA. An 8-bit
// channel v widens to v*257, so an 8-bit file and a 16-bit file compare exactly.
// The non-premultiplied image types are read directly: going through the color
// model would premultiply and lose bits on partially transparent pixels.
func pixel(img image.Image, x, y int) color.NRGBA64 {
	switch im := img.(type) {
	case *image.NRGBA:
		c := im.NRGBAAt(x, y)
		return color.NRGBA64{R: uint16(c.R) * 257, G: uint16(c.G) * 257, B: uint16(c.B) * 257, A: uint16(c.A) * 257}
	case *image.NRGBA64:
		return im.NRGBA64At(x, y)
	}
	n, _ := color.NRGBA64Model.Convert(img.At(x, y)).(color.NRGBA64) // the model always returns a color.NRGBA64
	return n
}
