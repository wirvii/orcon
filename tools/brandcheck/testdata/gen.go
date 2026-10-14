// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

//go:build ignore

// Command gen writes the raster fixtures of the brandcheck tests into the
// directory given as its only argument (normally testdata/raster):
//
//	go run ./brandcheck/testdata/gen.go brandcheck/testdata/raster
//
// The board is a small synthetic image, not a real reference board. The mascot
// cut-outs are disc-shaped crops of a fixed region of it; each bad variant
// breaks exactly one rule.
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
)

// Region of the synthetic board that the mascot fixtures cut out.
const (
	regionX, regionY, regionW, regionH = 8, 6, 24, 20
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: go run gen.go <output dir>")
	}
	dir := os.Args[1]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	board := makeBoard(64, 48)
	write(dir, "board.png", encode(board))

	good := cutOut(board, 10)
	write(dir, "mascot-good.png", encode(good))

	bad := cloneNRGBA(good)
	// One fully opaque pixel inside the disc gets a color the board does not have.
	px := bad.NRGBAAt(regionW/2, regionH/2)
	px.R ^= 0x40
	bad.SetNRGBA(regionW/2, regionH/2, px)
	write(dir, "mascot-bad-pixel.png", encode(bad))

	corner := cloneNRGBA(good)
	corner.SetNRGBA(0, 0, toNRGBA(board.At(regionX, regionY)))
	write(dir, "mascot-opaque-corner.png", encode(corner))

	write(dir, "mascot-sparse.png", encode(cutOut(board, 4)))

	// Partial-alpha edge pixels may carry a de-contaminated color.
	write(dir, "mascot-soft-edge.png", encode(softEdge(good)))

	gray := image.NewGray(image.Rect(0, 0, regionW, regionH))
	write(dir, "mascot-no-alpha.png", encode(gray))

	write(dir, "mascot-wrong-size.png", encode(image.NewNRGBA(image.Rect(0, 0, 10, 10))))

	write(dir, "solid-16.png", encode(solid(16)))
	write(dir, "solid-32.png", encode(solid(32)))
	write(dir, "solid-48.png", encode(solid(48)))
	write(dir, "solid-64.png", encode(solid(64)))

	write(dir, "icon-16-32-48.ico", ico(16, 32, 48))
	write(dir, "icon-16-32.ico", ico(16, 32))
	write(dir, "icon-16-32-48-64.ico", ico(16, 32, 48, 64))
	write(dir, "icon-truncated.ico", ico(16, 32, 48)[:20])
}

func makeBoard(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 4), G: uint8(y * 5), B: uint8((x * y) % 251), A: 255})
		}
	}
	return img
}

// cutOut crops the region and keeps only the pixels inside a centered disc of
// the given radius: board pixels, fully opaque, over a transparent background.
func cutOut(board *image.NRGBA, radius float64) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, regionW, regionH))
	cx, cy := float64(regionW)/2, float64(regionH)/2
	for y := range regionH {
		for x := range regionW {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			if dx*dx+dy*dy <= radius*radius {
				out.SetNRGBA(x, y, board.NRGBAAt(regionX+x, regionY+y))
			}
		}
	}
	return out
}

// softEdge marks the opaque pixels that touch a transparent one as half
// transparent with a different color, as a background-removal step would.
func softEdge(src *image.NRGBA) *image.NRGBA {
	out := cloneNRGBA(src)
	b := src.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if src.NRGBAAt(x, y).A != 255 {
				continue
			}
			for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if image.Pt(x+d[0], y+d[1]).In(b) && src.NRGBAAt(x+d[0], y+d[1]).A == 0 {
					out.SetNRGBA(x, y, color.NRGBA{R: 1, G: 2, B: 3, A: 128})
					break
				}
			}
		}
	}
	return out
}

func solid(n int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for i := 0; i < len(img.Pix); i += 4 {
		copy(img.Pix[i:], []uint8{0x22, 0xC5, 0x5E, 0xFF})
	}
	return img
}

// ico packs PNG images of the given square sizes into an ICO file.
func ico(sizes ...int) []byte {
	var images [][]byte
	for _, s := range sizes {
		images = append(images, encode(solid(s)))
	}
	var out bytes.Buffer
	header := []uint16{0, 1, uint16(len(sizes))}
	if err := binary.Write(&out, binary.LittleEndian, header); err != nil {
		log.Fatal(err)
	}
	offset := 6 + 16*len(sizes)
	for i, s := range sizes {
		entry := []byte{byte(s), byte(s), 0, 0}
		entry = binary.LittleEndian.AppendUint16(entry, 1)  // planes
		entry = binary.LittleEndian.AppendUint16(entry, 32) // bits per pixel
		entry = binary.LittleEndian.AppendUint32(entry, uint32(len(images[i])))
		entry = binary.LittleEndian.AppendUint32(entry, uint32(offset))
		out.Write(entry)
		offset += len(images[i])
	}
	for _, img := range images {
		out.Write(img)
	}
	return out.Bytes()
}

func cloneNRGBA(src *image.NRGBA) *image.NRGBA {
	out := image.NewNRGBA(src.Bounds())
	copy(out.Pix, src.Pix)
	return out
}

func toNRGBA(c color.Color) color.NRGBA {
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

func encode(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

func write(dir, name string, data []byte) {
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		log.Fatal(err)
	}
}
