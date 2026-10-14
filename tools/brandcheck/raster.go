// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/png" // registers the PNG decoder for image.DecodeConfig
	"slices"
)

// pngSignature starts every PNG stream, also the ones stored inside an ICO.
var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// checkPNGSize applies BC-3 to a PNG asset: the file is a PNG with exactly the
// width and height the manifest declares.
func checkPNGSize(a Asset, data []byte) []Finding {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "png" {
		return []Finding{whole(a.Path, ruleSizes, "not a decodable PNG (format %q): %v", format, err)}
	}
	if cfg.Width != a.Width || cfg.Height != a.Height {
		return []Finding{whole(a.Path, ruleSizes, "PNG is %dx%d, the manifest declares %dx%d", cfg.Width, cfg.Height, a.Width, a.Height)}
	}
	return nil
}

// icoEntry is one directory entry of an ICO file.
type icoEntry struct {
	width, height int
	size, offset  int
}

// checkICO applies BC-3 to an ICO asset: the directory holds exactly the
// square entries the manifest declares, no more and no fewer.
func checkICO(a Asset, data []byte) []Finding {
	entries, err := parseICO(data)
	if err != nil {
		return []Finding{whole(a.Path, ruleSizes, "invalid ICO: %v", err)}
	}
	var findings []Finding
	var got []int
	for i, e := range entries {
		if e.width != e.height {
			findings = append(findings, whole(a.Path, ruleSizes, "ICO entry %d is %dx%d, entries must be square", i, e.width, e.height))
		}
		got = append(got, e.width)
		payload := data[e.offset : e.offset+e.size]
		if !bytes.HasPrefix(payload, pngSignature) {
			continue
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(payload))
		if err != nil || cfg.Width != e.width || cfg.Height != e.height {
			findings = append(findings, whole(a.Path, ruleSizes, "ICO entry %d declares %dx%d but its PNG image does not match", i, e.width, e.height))
		}
	}
	want := slices.Clone(a.Entries)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		findings = append(findings, whole(a.Path, ruleSizes, "ICO entries are %v, the manifest declares exactly %v", got, want))
	}
	return findings
}

// parseICO reads the directory of an ICO file and checks that every entry
// points inside the file.
func parseICO(data []byte) ([]icoEntry, error) {
	const headerLen, entryLen = 6, 16
	if len(data) < headerLen {
		return nil, fmt.Errorf("file has %d bytes, shorter than the 6 byte header", len(data))
	}
	if reserved, kind := binary.LittleEndian.Uint16(data[0:]), binary.LittleEndian.Uint16(data[2:]); reserved != 0 || kind != 1 {
		return nil, fmt.Errorf("header says reserved=%d type=%d, want 0 and 1 (icon)", reserved, kind)
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	if len(data) < headerLen+count*entryLen {
		return nil, fmt.Errorf("directory of %d entries does not fit in %d bytes", count, len(data))
	}
	entries := make([]icoEntry, 0, count)
	for i := range count {
		p := data[headerLen+i*entryLen:]
		e := icoEntry{
			width:  int(p[0]),
			height: int(p[1]),
			size:   int(binary.LittleEndian.Uint32(p[8:])),
			offset: int(binary.LittleEndian.Uint32(p[12:])),
		}
		if e.width == 0 {
			e.width = 256
		}
		if e.height == 0 {
			e.height = 256
		}
		if e.size <= 0 || e.offset < headerLen+count*entryLen || e.offset+e.size > len(data) {
			return nil, fmt.Errorf("entry %d points outside the file (offset %d, size %d, file %d bytes)", i, e.offset, e.size, len(data))
		}
		entries = append(entries, e)
	}
	return entries, nil
}
