// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authoring

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

const cleanMark = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#25D366" d="M1 1h22v22H1z"/></svg>`

func TestSanitizeRejectsUnsafeMarkup(t *testing.T) {
	cases := map[string]string{
		"script element":       `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"event attribute":      `<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><path d="M0 0"/></svg>`,
		"external href":        `<svg xmlns="http://www.w3.org/2000/svg"><use href="https://evil.example/x.svg#a"/></svg>`,
		"javascript href":      `<svg xmlns="http://www.w3.org/2000/svg"><a href="javascript:alert(1)"><path d="M0 0"/></a></svg>`,
		"external url() paint": `<svg xmlns="http://www.w3.org/2000/svg"><path fill="url(https://evil.example/p.svg#g)" d="M0 0"/></svg>`,
		"foreign object":       `<svg xmlns="http://www.w3.org/2000/svg"><foreignObject><div/></foreignObject></svg>`,
		"raster image":         `<svg xmlns="http://www.w3.org/2000/svg"><image href="#a" width="1" height="1"/></svg>`,
		"doctype with entity":  `<?xml version="1.0"?><!DOCTYPE svg [<!ENTITY x "y">]><svg xmlns="http://www.w3.org/2000/svg"/>`,
		"not an svg root":      `<html xmlns="http://www.w3.org/1999/xhtml"><body/></html>`,
		"style element":        `<svg xmlns="http://www.w3.org/2000/svg"><style>@import url(https://evil.example/x.css);</style></svg>`,
		"animation element":    `<svg xmlns="http://www.w3.org/2000/svg"><set attributeName="href" to="javascript:alert(1)"/></svg>`,
		"oversized":            `<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat(`<path d="M0 0"/>`, 1000) + `</svg>`,
		"not well formed":      `<svg xmlns="http://www.w3.org/2000/svg"><path></svg>`,
	}
	for name, markup := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := SanitizeSVG([]byte(markup)); !errors.Is(err, ErrUnsafeIcon) {
				t.Fatalf("want ErrUnsafeIcon, got %v", err)
			}
		})
	}
}

func TestSanitizeAcceptsCleanMarks(t *testing.T) {
	whatsapp, err := os.ReadFile(filepath.Join("testdata", "whatsapp-simple-icons-16.35.0.svg"))
	if err != nil {
		t.Fatalf("read the fixture: %v", err)
	}
	for name, markup := range map[string][]byte{
		"inline mark":        []byte(cleanMark),
		"internal reference": []byte(`<svg xmlns="http://www.w3.org/2000/svg"><defs><path id="p" d="M0 0"/></defs><use href="#p"/></svg>`),
		"pinned simple icon": whatsapp,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := SanitizeSVG(markup); err != nil {
				t.Fatalf("a clean mark must pass: %v", err)
			}
		})
	}
}

func TestLoadIconChecksEveryFile(t *testing.T) {
	valid := fstest.MapFS{
		"icons/icon.svg": {Data: []byte(cleanMark)},
		"icons/SOURCE":   {Data: []byte("simple-icons@16.35.0:whatsapp\n")},
	}
	icon, err := LoadIcon(valid)
	if err != nil {
		t.Fatalf("a valid package must load: %v", err)
	}
	if icon.SVG != cleanMark || icon.Source != "simple-icons@16.35.0:whatsapp" || icon.DarkSVG != "" {
		t.Fatalf("unexpected icon %+v", icon)
	}

	withDark := fstest.MapFS{
		"icons/icon.svg":      {Data: []byte(cleanMark)},
		"icons/icon-dark.svg": {Data: []byte(cleanMark)},
		"icons/SOURCE":        {Data: []byte("brand-kit:https://brand.example/kit\n")},
	}
	if icon, err := LoadIcon(withDark); err != nil || icon.DarkSVG != cleanMark {
		t.Fatalf("a published dark variant must be kept: %+v, %v", icon, err)
	}

	unsafe := fstest.MapFS{
		"icons/icon.svg": {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" onclick="x()"/>`)},
		"icons/SOURCE":   {Data: []byte("orcon-fallback\n")},
	}
	if _, err := LoadIcon(unsafe); !errors.Is(err, ErrUnsafeIcon) {
		t.Fatalf("an unsafe icon must not load: %v", err)
	}

	badSource := fstest.MapFS{
		"icons/icon.svg": {Data: []byte(cleanMark)},
		"icons/SOURCE":   {Data: []byte("the logo\n")},
	}
	if _, err := LoadIcon(badSource); err == nil {
		t.Fatal("an icon without a valid source must not load")
	}
}
