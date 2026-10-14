// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
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
		{name: "xlink:href to a URL", fixture: "href-external", reject: "namespaced attribute"},
		{name: "href to a relative file", fixture: "href-relative", reject: "not a local #fragment"},
		{name: "href on use to a URL", fixture: "use-href-external", reject: "not a local #fragment"},
		{name: "src to a URL", fixture: "src-external", reject: "attribute is not allowed"},
		{name: "local #fragment href is accepted", fixture: "clean"},
	})
}

func TestSVGRejectsCSSURL(t *testing.T) {
	t.Parallel()
	runSVGCases(t, []svgCase{
		{name: "fill url() to a remote file", fixture: "css-url-external", reject: "a local url(#id)"},
		{name: "style attribute url() to a file", fixture: "css-url-style-attr", reject: "style attributes are not allowed"},
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
		{name: "style attribute", fixture: "style-attribute", reject: "style attributes are not allowed"},
		{name: "plain presentation attributes are accepted", fixture: "clean"},
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
	const head = "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 10 10\"><path d=\"M0 0h10v10z\"/>"
	const tail = "</svg>"
	pad := func(total int) []byte {
		return []byte(head + strings.Repeat(" ", total-len(head)-len(tail)) + tail)
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

// TestSVGRejectsAllowlistBypasses runs every bypass found in review against the
// checker: animation of href or style, metadata, switch, CSS escapes, entities,
// namespace tricks and the rest. Each must fail.
func TestSVGRejectsAllowlistBypasses(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(filepath.Join("testdata", "svg", "bypass"))
	if err != nil {
		t.Fatal(err)
	}
	const wantFiles = 21
	if len(entries) != wantFiles {
		t.Fatalf("testdata/svg/bypass holds %d fixtures, want %d: a fixture was lost", len(entries), wantFiles)
	}
	for _, e := range entries {
		t.Run(e.Name(), func(t *testing.T) {
			t.Parallel()
			got := checkSVG(e.Name(), readFixture(t, "svg/bypass/"+e.Name()))
			if len(got) == 0 {
				t.Fatalf("checkSVG(%s) accepted a bypass", e.Name())
			}
		})
	}
}

func TestSVGAllowlistCoversRealAssets(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		doc    string
		reject string
	}{
		{"gradient, clip path, use and transform", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><defs><radialGradient id="r" cx="2" cy="2" r="2"><stop offset=".5" stop-color="#fff" stop-opacity=".5"/></radialGradient><mask id="m"><rect width="4" height="4" fill="#fff"/></mask></defs><g transform="matrix(1 0 0 1 0 0) rotate(5)"><polygon points="0,0 4,0 4,4" fill="url(#r)" mask="url(#m)"/></g></svg>`, ""},
		{"xmlns:xlink declaration is allowed", `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 4 4"><path d="M0 0h4v4z"/></svg>`, ""},
		{"foreign namespace declaration", `<svg xmlns="http://www.w3.org/2000/svg" xmlns:x="http://example.com/x" viewBox="0 0 4 4"><path d="M0 0h4v4z"/></svg>`, "namespace"},
		{"element outside the SVG namespace", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><path d="M0 0h4v4z"/><path xmlns="http://www.w3.org/1999/xlink" d="M0 0"/></svg>`, "not in the SVG namespace"},
		{"text outside title", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><path d="M0 0h4v4z"/><g>hello</g></svg>`, "only allowed inside <title>"},
		{"fill with a function", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><path d="M0 0h4v4z" fill="rgb(1,2,3)"/></svg>`, "plain paint"},
		{"backslash in a value", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><path d="M0 0h4v4z" id="a\b"/></svg>`, "backslash"},
		{"parenthesis in an arbitrary attribute", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><path d="M0 0h4v4z" stroke-width="calc(1px)"/></svg>`, "optional px"},
		{"url with a fallback is not exact url(#id)", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><path d="M0 0h4v4z" fill="url(#a) url(https://x/y)"/></svg>`, "plain paint"},
		{"upper case attribute name", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><use HREF="#a"/><path d="M0 0h4v4z"/></svg>`, "not allowed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := checkSVG("x.svg", []byte(tc.doc))
			if tc.reject == "" {
				if len(got) != 0 {
					t.Fatalf("checkSVG found:\n%swant it accepted", render(got))
				}
				return
			}
			if !strings.Contains(render(got), tc.reject) {
				t.Fatalf("checkSVG found:\n%swant a finding containing %q", render(got), tc.reject)
			}
		})
	}
}
