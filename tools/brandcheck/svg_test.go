// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func TestSVGAcceptsCleanVector(t *testing.T) {
	t.Parallel()
	runSVGCases(t, []svgCase{
		{name: "gradient, local use and local url", fixture: "clean"},
	})
}

func TestSVGRejectsImageElement(t *testing.T) {
	t.Parallel()
	runSVGCases(t, []svgCase{
		{name: "lower case image", fixture: "image-lower", reject: "<image>"},
		{name: "upper case IMAGE", fixture: "image-upper", reject: "<IMAGE>"},
		{name: "image with a namespace prefix", fixture: "image-prefixed", reject: "<image>"},
		{name: "feImage filter primitive", fixture: "feimage", reject: "<feImage>"},
		{name: "clean vector is accepted", fixture: "clean"},
	})

	t.Run("finding names the line of the element", func(t *testing.T) {
		t.Parallel()
		doc := "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 10 10\">\n<path d=\"M0 0h10v10z\"/>\n\n<image width=\"1\" height=\"1\"/>\n</svg>\n"
		got := checkSVG("a.svg", []byte(doc))
		if len(got) != 1 || got[0].Line != 4 {
			t.Fatalf("checkSVG(<image> on line 4) = %v, want one finding at line 4", got)
		}
		if s := got[0].String(); !strings.HasPrefix(s, "a.svg:4: BC-1: ") {
			t.Errorf("Finding.String() = %q, want the form file:line: rule: detail", s)
		}
	})
}

func TestSVGRejectsForeignObjectAndScript(t *testing.T) {
	t.Parallel()
	runSVGCases(t, []svgCase{
		{name: "foreignObject", fixture: "foreignobject", reject: "<foreignObject>"},
		{name: "script", fixture: "script", reject: "<script>"},
		{name: "clean vector is accepted", fixture: "clean"},
	})
}

func TestSVGRejectsDataURI(t *testing.T) {
	t.Parallel()
	runSVGCases(t, []svgCase{
		{name: "data: in href", fixture: "data-href", reject: "data: URI"},
		{name: "data: inside fill url()", fixture: "data-fill", reject: "data: URI"},
		{name: "data: in a text node", fixture: "data-textnode", reject: "data: URI"},
		{name: "upper case DATA:", fixture: "data-upper", reject: "data: URI"},
		{name: "clean vector is accepted", fixture: "clean"},
	})
}

func TestSVGRejectsBase64(t *testing.T) {
	t.Parallel()
	runSVGCases(t, []svgCase{
		{name: "base64 in an attribute, any case", fixture: "base64-attr", reject: "base64"},
		{name: "base64 in a text node", fixture: "base64-text", reject: "base64"},
		{name: "base64 behind a data: URI", fixture: "data-fill", reject: "base64"},
		{name: "clean vector is accepted", fixture: "clean"},
	})
}

func TestSVGRejectsExternalHref(t *testing.T) {
	t.Parallel()
	runSVGCases(t, []svgCase{
		{name: "xlink:href to a URL", fixture: "href-external", reject: "not a local #fragment"},
		{name: "href to a relative file", fixture: "href-relative", reject: "not a local #fragment"},
		{name: "src to a URL", fixture: "src-external", reject: "not a local #fragment"},
		{name: "local #fragment href is accepted", fixture: "clean"},
	})
}

func TestSVGRejectsCSSURL(t *testing.T) {
	t.Parallel()
	runSVGCases(t, []svgCase{
		{name: "fill url() to a remote file", fixture: "css-url-external", reject: "not a local url(#id)"},
		{name: "style attribute url() to a file", fixture: "css-url-style-attr", reject: "not a local url(#id)"},
		{name: "url(#id) in fill and style is accepted", fixture: "clean"},
	})
}

func TestSVGRejectsTextElements(t *testing.T) {
	t.Parallel()
	runSVGCases(t, []svgCase{
		{name: "text", fixture: "text-element", reject: "<text>"},
		{name: "tspan", fixture: "tspan-element", reject: "<tspan>"},
		{name: "textPath", fixture: "textpath-element", reject: "<textPath>"},
		{name: "title text is not a text element", fixture: "clean"},
	})
}

func TestSVGRejectsStyleAndEventHandlers(t *testing.T) {
	t.Parallel()
	runSVGCases(t, []svgCase{
		{name: "style element", fixture: "style-element", reject: "<style>"},
		{name: "onclick attribute", fixture: "onclick-attr", reject: "event handler"},
		{name: "onload on the root", fixture: "onload-root", reject: "event handler"},
		{name: "inline style attribute without url is accepted", fixture: "clean"},
	})
}

func TestSVGRequiresViewBoxAndShapes(t *testing.T) {
	t.Parallel()
	runSVGCases(t, []svgCase{
		{name: "no viewBox", fixture: "no-viewbox", reject: "no viewBox"},
		{name: "no drawn shape", fixture: "no-shapes", reject: "no shape element"},
		{name: "root is not svg", fixture: "not-svg-root", reject: "want <svg>"},
		{name: "malformed XML", fixture: "malformed", reject: "not well-formed"},
		{name: "DOCTYPE with an entity", fixture: "doctype", reject: "directive"},
		{name: "viewBox and shapes present", fixture: "clean"},
	})
}

func TestSVGRejectsOversizedFile(t *testing.T) {
	t.Parallel()
	const head = "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 10 10\"><path d=\"M0 0h10v10z\"/><!--"
	const tail = "--></svg>"
	pad := func(total int) []byte {
		return []byte(head + strings.Repeat("x", total-len(head)-len(tail)) + tail)
	}
	tests := []struct {
		name   string
		doc    []byte
		reject bool
	}{
		{"exactly 16 KiB", pad(maxSVGBytes), false},
		{"one byte over 16 KiB", pad(maxSVGBytes + 1), true},
		{"far over 16 KiB", pad(4 * maxSVGBytes), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := checkSVG("big.svg", tc.doc)
			switch {
			case tc.reject && (len(got) != 1 || !strings.Contains(got[0].Detail, "byte limit")):
				t.Errorf("checkSVG(%d bytes) = %v, want one size finding", len(tc.doc), got)
			case !tc.reject && len(got) != 0:
				t.Errorf("checkSVG(%d bytes) = %v, want none", len(tc.doc), got)
			}
		})
	}
}
