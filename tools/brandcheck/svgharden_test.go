// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// doc wraps svg content in a valid root with one shape.
func doc(inner string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><path d="M0 0h10v10z"/>` + inner + `</svg>`
}

const (
	spdxCopyright = "<!-- SPDX-FileCopyrightText: The Orcon Authors -->"
	spdxLicense   = "<!-- SPDX-License-Identifier: Apache-2.0 -->"
)

type docCase struct {
	name   string
	doc    string
	reject string // empty: the document must be accepted
}

func runDocCases(t *testing.T, cases []docCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := checkSVG("x.svg", []byte(tc.doc))
			if tc.reject == "" {
				if len(got) != 0 {
					t.Fatalf("checkSVG found:\n%swant the document accepted", render(got))
				}
				return
			}
			if !strings.Contains(render(got), tc.reject) {
				t.Fatalf("checkSVG found:\n%swant a finding containing %q", render(got), tc.reject)
			}
		})
	}
}

// TestSVGProbesFromReview runs the second round of review probes: only the two
// fixtures that are valid vectors may pass.
func TestSVGProbesFromReview(t *testing.T) {
	t.Parallel()
	accepted := map[string]bool{"clean.svg": true, "use_ok.svg": true}
	entries, err := os.ReadDir(filepath.Join("testdata", "svg", "probes"))
	if err != nil {
		t.Fatal(err)
	}
	const wantFiles = 34
	if len(entries) != wantFiles {
		t.Fatalf("testdata/svg/probes holds %d fixtures, want %d: a fixture was lost", len(entries), wantFiles)
	}
	for _, e := range entries {
		t.Run(e.Name(), func(t *testing.T) {
			t.Parallel()
			got := checkSVG(e.Name(), readFixture(t, "svg/probes/"+e.Name()))
			switch {
			case accepted[e.Name()] && len(got) != 0:
				t.Errorf("checkSVG(%s) found:\n%swant it accepted", e.Name(), render(got))
			case !accepted[e.Name()] && len(got) == 0:
				t.Errorf("checkSVG(%s) accepted a probe that must fail", e.Name())
			}
		})
	}
}

func TestSVGBansCommentsAndProcessingInstructions(t *testing.T) {
	t.Parallel()
	runDocCases(t, []docCase{
		{name: "both SPDX lines before the root", doc: spdxCopyright + "\n" + spdxLicense + "\n" + doc("")},
		{name: "comment inside the root", doc: doc("<!-- hello -->"), reject: "comment is not allowed"},
		{name: "payload in a comment", doc: doc("<!-- iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk -->"), reject: "comment is not allowed"},
		{name: "other comment before the root", doc: "<!-- note -->" + doc(""), reject: "comment is not allowed"},
		{name: "SPDX line after the root", doc: doc("") + spdxCopyright, reject: "comment is not allowed"},
		{name: "SPDX line twice", doc: spdxCopyright + spdxCopyright + doc(""), reject: "comment is not allowed"},
		{name: "SPDX line with extra words", doc: "<!-- SPDX-FileCopyrightText: The Orcon Authors and more -->" + doc(""), reject: "comment is not allowed"},
		{name: "xml declaration", doc: `<?xml version="1.0" encoding="UTF-8"?>` + doc(""), reject: "processing instruction"},
		{name: "stylesheet instruction", doc: doc(`<?xml-stylesheet href="x.css"?>`), reject: "processing instruction"},
	})
}

func TestSVGValidatesPathDataAndPoints(t *testing.T) {
	t.Parallel()
	path := func(d string) string {
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><path d="` + d + `"/></svg>`
	}
	poly := func(p string) string {
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><polygon points="` + p + `"/></svg>`
	}
	runDocCases(t, []docCase{
		{name: "real-world path with glued signs", doc: path("M141.87 168.38c-6.44 .52-13.03 .18-19.49 .17l1-2h-3v4Zm0 0 1 1")},
		{name: "arc", doc: path("M0 0a5 5 0 0 1 5 5")},
		{name: "exponent", doc: path("M1e2 -2.5E-1L3 4")},
		{name: "points as pairs", doc: poly("31,86 84.1,113.7 151,81.6")},
		{name: "base64 as path data", doc: path("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk"), reject: "path data: unexpected"},
		{name: "letters inside the data", doc: path("M0 0 hello"), reject: "path data"},
		{name: "unknown command letter", doc: path("M0 0 X5"), reject: "path data"},
		{name: "number before any command", doc: path("5 5 M0 0"), reject: "path data"},
		{name: "wrong arity for curves", doc: path("M0 0c1 2 3 4 5"), reject: "want a multiple of 6"},
		{name: "wrong arity for lines", doc: path("M0 0 1"), reject: "want a multiple of 2"},
		{name: "close with numbers", doc: path("M0 0z5"), reject: "takes no numbers"},
		{name: "empty path data", doc: path(""), reject: "path data is empty"},
		{name: "huge coordinate in path", doc: path("M0 0L1000001 0"), reject: "beyond"},
		{name: "huge exponent in path", doc: path("M0 0L1e99 0"), reject: "beyond"},
		{name: "semicolon in path", doc: path("M0 0;5"), reject: "path data"},
		{name: "odd number of points", doc: poly("1,2 3"), reject: "want pairs"},
		{name: "text in points", doc: poly("1,2 a,b"), reject: "not a list of numbers"},
		{name: "huge point", doc: poly("1,2 3,-1e7"), reject: "beyond"},
	})
}

func TestSVGBoundsNumbersAndAttributes(t *testing.T) {
	t.Parallel()
	rect := func(attrs string) string {
		return doc(`<rect width="1" height="1" ` + attrs + `/>`)
	}
	runDocCases(t, []docCase{
		{name: "ordinary numbers", doc: rect(`x="-5.5" y="1e2" rx="50%" stroke-width="2px" opacity=".5"`)},
		{name: "number at the limit", doc: rect(`x="1000000"`)},
		{name: "number over the limit", doc: rect(`x="1000001"`), reject: "beyond"},
		{name: "negative number over the limit", doc: rect(`x="-2000000"`), reject: "beyond"},
		{name: "overflowing exponent", doc: rect(`x="1e999999999"`), reject: "optional px"},
		{name: "three digit exponent", doc: rect(`x="1e400"`), reject: "optional px"},
		{name: "text as a number", doc: rect(`rx="zzzz"`), reject: "optional px"},
		{name: "huge transform argument", doc: rect(`transform="scale(2000000 1)"`), reject: "beyond"},
		{name: "overflowing transform exponent", doc: rect(`transform="scale(1e308 1e308)"`), reject: "transform list"},
		{name: "transform rotate with an overflowing exponent", doc: rect(`transform="rotate(1e400)"`), reject: "transform list"},
		{name: "transform with a function inside", doc: rect(`transform="translate(url(#a))"`), reject: "transform list"},
		{name: "unknown enum value", doc: rect(`fill-rule="nonzero evenodd"`), reject: "is not one of"},
		{name: "viewBox with text", doc: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 a b"><path d="M0 0h1"/></svg>`, reject: "viewBox"},
		{name: "viewBox with a negative size", doc: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 -4 4"><path d="M0 0h1"/></svg>`, reject: "positive"},
		{name: "viewBox with three numbers", doc: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4"><path d="M0 0h1"/></svg>`, reject: "viewBox"},
		{name: "stop color that is a url", doc: doc(`<linearGradient id="g"><stop offset="0" stop-color="url(#g)"/></linearGradient>`), reject: "plain color"},
		{name: "paint with a fallback", doc: rect(`fill="url(#a) red"`), reject: "plain paint"},
		{name: "href with leading white space", doc: doc(`<g id="a"/><use href=" #a"/>`), reject: "local #fragment"},
		{name: "href with a zero width space", doc: doc("<g id=\"a\"/><use href=\"​#a\"/>"), reject: "local #fragment"},
	})
}

func TestSVGLimitsIdsAndText(t *testing.T) {
	t.Parallel()
	runDocCases(t, []docCase{
		{name: "short title", doc: doc("<title>Orcon Orchestration Controller</title>")},
		{name: "title with indentation", doc: doc("<title>\n  Orcon (symbol)\n</title>")},
		{name: "title over 200 characters", doc: doc("<title>" + strings.Repeat("word ", 41) + "</title>"), reject: "over the 200 limit"},
		{name: "base64-like run in a title", doc: doc("<title>iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk</title>"), reject: "unbroken run"},
		{name: "URL in a description", doc: doc("<desc>see https://evil.example/x.png</desc>"), reject: "only letters"},
		{name: "markup in a description", doc: doc("<desc><![CDATA[<image/>]]></desc>"), reject: "only letters"},
		{name: "no-break space in a title", doc: doc("<title>a b</title>"), reject: "only letters"},
		{name: "id with a dot", doc: doc(`<g id="a.b"/>`), reject: "safe identifier"},
		{name: "id starting with a digit", doc: doc(`<g id="1a"/>`), reject: "safe identifier"},
		{name: "id with a no-break space", doc: doc(`<g id="a&#160;b"/>`), reject: "safe identifier"},
		{name: "id of 32 characters", doc: doc(`<g id="` + strings.Repeat("a", 32) + `"/>`)},
		{name: "id of 33 characters", doc: doc(`<g id="` + strings.Repeat("a", 33) + `"/>`), reject: "safe identifier"},
		{name: "duplicate id", doc: doc(`<g id="a"/><g id="a"/>`), reject: "duplicate id"},
		{name: "nested svg", doc: doc(`<svg viewBox="0 0 1 1"><path d="M0 0h1"/></svg>`), reject: "nested <svg>"},
	})
}

func TestSVGRejectsReferenceCyclesAndDeepUse(t *testing.T) {
	t.Parallel()
	chain := func(n int) string {
		var b strings.Builder
		b.WriteString(`<g id="n0"><path d="M0 0h1"/></g>`)
		for i := 1; i <= n; i++ {
			b.WriteString(`<g id="n` + string(rune('0'+i)) + `"><use href="#n` + string(rune('0'+i-1)) + `"/></g>`)
		}
		b.WriteString(`<use href="#n` + string(rune('0'+n)) + `"/>`)
		return doc(b.String())
	}
	runDocCases(t, []docCase{
		{name: "use of a defined group", doc: doc(`<g id="a"><path d="M0 0h1"/></g><use href="#a"/>`)},
		{name: "use chain at the depth limit", doc: chain(2)},
		{name: "use chain over the depth limit", doc: chain(4), reject: "nested 5 deep, over the limit of 3"},
		{name: "use of itself through its group", doc: doc(`<g id="a"><use href="#a"/></g>`), reject: "reference cycle"},
		{name: "two groups using each other", doc: doc(`<g id="a"><use href="#b"/></g><g id="b"><use href="#a"/></g>`), reject: "reference cycle"},
		{name: "use of an ancestor of the use", doc: doc(`<g id="a"><g id="b"><use href="#a"/></g></g>`), reject: "reference cycle"},
		{name: "gradient href loop", doc: doc(`<linearGradient id="g1" href="#g2"/><linearGradient id="g2" href="#g1"/>`), reject: "reference cycle"},
		{name: "gradient inheriting from another", doc: doc(`<linearGradient id="g1"><stop offset="0" stop-color="red"/></linearGradient><linearGradient id="g2" href="#g1"/>`)},
		{name: "clip path loop", doc: doc(`<clipPath id="c1" clip-path="url(#c2)"><rect width="1" height="1"/></clipPath><clipPath id="c2" clip-path="url(#c1)"><rect width="1" height="1"/></clipPath>`), reject: "reference cycle"},
		{name: "mask referring to itself", doc: doc(`<mask id="m" mask="url(#m)"><path d="M0 0h1"/></mask>`), reject: "reference cycle"},
		{name: "fill referring to its own element", doc: doc(`<rect id="r" width="1" height="1" fill="url(#r)"/>`), reject: "reference cycle"},
		{name: "reference to an id that is never defined", doc: doc(`<use href="#missing"/>`)},
	})
}
