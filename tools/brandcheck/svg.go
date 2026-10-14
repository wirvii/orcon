// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// maxSVGBytes is the size ceiling of a brand SVG (SPEC-018 BC-1).
const maxSVGBytes = 16 * 1024

// Namespaces a brand SVG may declare.
const (
	svgNamespace   = "http://www.w3.org/2000/svg"
	xlinkNamespace = "http://www.w3.org/1999/xlink"
)

// The vector check is an allowlist: a brand SVG may contain only the elements
// and attributes below, and every value is validated. Anything not listed is a
// finding, so a feature nobody thought of (animation, metadata, switch, CSS
// escapes, entity tricks) fails closed instead of slipping through.

// paintAttrs are the presentation attributes every drawn element accepts.
var paintAttrs = []string{
	"id", "transform", "fill", "fill-opacity", "fill-rule", "stroke", "stroke-width",
	"stroke-linecap", "stroke-linejoin", "stroke-miterlimit", "stroke-opacity",
	"stroke-dasharray", "stroke-dashoffset", "opacity", "clip-path", "clip-rule", "mask",
}

// elementAttrs maps each allowed element to the attributes it accepts, besides
// namespace declarations. Names are case sensitive, as in SVG.
var elementAttrs = map[string]map[string]bool{
	"svg":            attrSet(paintAttrs, "viewBox", "width", "height", "preserveAspectRatio", "version"),
	"g":              attrSet(paintAttrs),
	"defs":           attrSet(paintAttrs),
	"path":           attrSet(paintAttrs, "d"),
	"rect":           attrSet(paintAttrs, "x", "y", "width", "height", "rx", "ry"),
	"circle":         attrSet(paintAttrs, "cx", "cy", "r"),
	"ellipse":        attrSet(paintAttrs, "cx", "cy", "rx", "ry"),
	"line":           attrSet(paintAttrs, "x1", "y1", "x2", "y2"),
	"polyline":       attrSet(paintAttrs, "points"),
	"polygon":        attrSet(paintAttrs, "points"),
	"use":            attrSet(paintAttrs, "href", "x", "y", "width", "height"),
	"linearGradient": attrSet(nil, "id", "gradientUnits", "gradientTransform", "spreadMethod", "x1", "y1", "x2", "y2", "href"),
	"radialGradient": attrSet(nil, "id", "gradientUnits", "gradientTransform", "spreadMethod", "cx", "cy", "r", "fx", "fy", "href"),
	"stop":           attrSet(nil, "id", "offset", "stop-color", "stop-opacity"),
	"clipPath":       attrSet(paintAttrs, "clipPathUnits"),
	"mask":           attrSet(paintAttrs, "maskUnits", "maskContentUnits", "x", "y", "width", "height"),
	"title":          attrSet(nil, "id"),
	"desc":           attrSet(nil, "id"),
}

// shapeElements are the elements that draw geometry; a vector asset has at
// least one.
var shapeElements = map[string]bool{
	"path": true, "polygon": true, "polyline": true, "rect": true,
	"circle": true, "ellipse": true, "line": true,
}

// textElements may hold character data; everywhere else only whitespace is
// allowed.
var textElements = map[string]bool{"title": true, "desc": true}

func attrSet(base []string, extra ...string) map[string]bool {
	set := make(map[string]bool, len(base)+len(extra))
	for _, n := range base {
		set[n] = true
	}
	for _, n := range extra {
		set[n] = true
	}
	return set
}

// checkSVG applies BC-1 to one SVG document; file only labels the findings.
func checkSVG(file string, data []byte) []Finding {
	c := svgChecker{file: file, data: data}
	if len(data) > maxSVGBytes {
		c.add(0, "file is %d bytes, over the %d byte limit of a brand vector", len(data), maxSVGBytes)
	}
	c.run()
	return c.findings
}

type svgChecker struct {
	file     string
	data     []byte
	findings []Finding

	roots    int
	stack    []string
	hasShape bool

	owners   []string // nearest element id (or "" for the document) per open element
	ids      map[string]bool
	refs     map[string][]refEdge
	spdx     map[string]bool
	rootSeen bool
}

// add records a finding at a byte offset of the document.
func (c *svgChecker) add(offset int64, format string, args ...any) {
	line := 1 + bytes.Count(c.data[:min(offset, int64(len(c.data)))], []byte{'\n'})
	c.findings = append(c.findings, Finding{File: c.file, Line: line, Rule: ruleVector, Detail: fmt.Sprintf(format, args...)})
}

func (c *svgChecker) run() {
	dec := xml.NewDecoder(bytes.NewReader(c.data))
	for {
		start := dec.InputOffset()
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			c.add(start, "not well-formed XML: %v", err)
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			c.start(start, t)
		case xml.EndElement:
			if len(c.stack) > 0 {
				c.stack = c.stack[:len(c.stack)-1]
				c.owners = c.owners[:len(c.owners)-1]
			}
		case xml.CharData:
			c.chars(start, string(t))
		case xml.Directive:
			c.add(start, "directive <!%s> is not allowed (no DOCTYPE or entity declarations)", shorten(string(t)))
		case xml.ProcInst:
			c.add(start, "processing instruction <?%s?> is not allowed (not even the XML declaration)", t.Target)
		case xml.Comment:
			c.comment(start, string(t))
		}
	}
	switch {
	case c.roots == 0:
		c.add(0, "no root element")
	case !c.hasShape:
		c.add(0, "no shape element (path, polygon, polyline, rect, circle, ellipse or line): not a drawn vector")
	}
	c.checkReferences()
}

// spdxComments are the only comments a brand vector may carry: the license
// header every file in the repository starts with, before the root element.
var spdxComments = map[string]bool{
	" SPDX-FileCopyrightText: The Orcon Authors ": true,
	" SPDX-License-Identifier: Apache-2.0 ":       true,
}

func (c *svgChecker) comment(offset int64, text string) {
	if c.rootSeen || len(c.stack) > 0 || !spdxComments[text] || c.spdx[text] {
		c.add(offset, "comment is not allowed (only the two SPDX header lines before the root element)")
		return
	}
	if c.spdx == nil {
		c.spdx = map[string]bool{}
	}
	c.spdx[text] = true
}

func (c *svgChecker) start(offset int64, el xml.StartElement) {
	name := el.Name.Local
	if len(c.stack) == 0 {
		c.roots++
		switch {
		case c.roots > 1:
			c.add(offset, "more than one root element")
		case name != "svg":
			c.add(offset, "root element is <%s>, want <svg>", name)
		case !hasViewBox(el):
			c.add(offset, "root <svg> has no viewBox")
		}
	}
	nested := len(c.stack) > 0 && name == "svg"
	c.stack = append(c.stack, name)
	c.rootSeen = true

	allowed, known := elementAttrs[name]
	switch {
	case el.Name.Space != svgNamespace:
		c.add(offset, "element <%s> is not in the SVG namespace", name)
		known = false
	case !known:
		c.add(offset, "element <%s> is not in the allowed vector element set", name)
	case nested:
		c.add(offset, "nested <svg> is not allowed; only the root is an <svg>")
		known = false
	}
	if shapeElements[name] && known {
		c.hasShape = true
	}
	for _, a := range el.Attr {
		c.attr(offset, name, a, allowed, known)
	}
	c.track(offset, name, el, known)
}

// track records the id and the references of an element for the cycle check.
func (c *svgChecker) track(offset int64, name string, el xml.StartElement, known bool) {
	parent := ""
	if n := len(c.owners); n > 0 {
		parent = c.owners[n-1]
	}
	owner := parent
	if known {
		for _, a := range el.Attr {
			if a.Name.Space != "" || a.Name.Local != "id" || !idValue.MatchString(a.Value) {
				continue
			}
			if c.ids[a.Value] {
				c.add(offset, "<%s id>: duplicate id %q", name, a.Value)
			}
			if c.ids == nil {
				c.ids = map[string]bool{}
			}
			c.ids[a.Value] = true
			owner = a.Value
			c.addRef(parent, owner, false, offset)
		}
		for _, a := range el.Attr {
			if a.Name.Space != "" {
				continue
			}
			switch a.Name.Local {
			case "href":
				if localRef.MatchString(a.Value) {
					c.addRef(owner, a.Value[1:], name == "use", offset)
				}
			case "fill", "stroke", "clip-path", "mask":
				if m := urlRef.FindStringSubmatch(a.Value); m != nil {
					c.addRef(owner, m[1], false, offset)
				}
			}
		}
	}
	c.owners = append(c.owners, owner)
}

func hasViewBox(el xml.StartElement) bool {
	for _, a := range el.Attr {
		if a.Name.Space == "" && a.Name.Local == "viewBox" && strings.TrimSpace(a.Value) != "" {
			return true
		}
	}
	return false
}

func (c *svgChecker) attr(offset int64, element string, a xml.Attr, allowed map[string]bool, known bool) {
	name := a.Name.Local
	where := fmt.Sprintf("<%s %s>", element, name)
	c.payload(offset, where, a.Value)

	// Namespace declarations are the only namespaced attributes, and they may
	// only bind the SVG and XLink namespaces.
	if a.Name.Space == "xmlns" || (a.Name.Space == "" && name == "xmlns") {
		if a.Value != svgNamespace && a.Value != xlinkNamespace {
			c.add(offset, "%s: namespace %q is not allowed", where, shorten(a.Value))
		}
		return
	}
	if !known {
		return // the element is already a finding; do not pile on its attributes
	}
	switch {
	case a.Name.Space != "":
		c.add(offset, "%s: namespaced attribute %s:%s is not allowed", where, a.Name.Space, name)
	case strings.HasPrefix(strings.ToLower(name), "on"):
		c.add(offset, "%s: event handler attributes are not allowed", where)
	case name == "style":
		c.add(offset, "%s: style attributes are not allowed (use presentation attributes)", where)
	case !allowed[name]:
		c.add(offset, "%s: attribute is not allowed on <%s>", where, element)
	default:
		if problem := checkValue(name, a.Value); problem != "" {
			c.add(offset, "%s: %s", where, problem)
		}
	}
}

// chars checks character data: only <title> and <desc> may carry text, and it
// must be short plain words. Elsewhere only ASCII white space is allowed.
func (c *svgChecker) chars(offset int64, text string) {
	c.payload(offset, "text node", text)
	if strings.Trim(text, " \t\r\n") == "" {
		return
	}
	parent := ""
	if len(c.stack) > 0 {
		parent = c.stack[len(c.stack)-1]
	}
	if !textElements[parent] {
		c.add(offset, "text %q is only allowed inside <title> and <desc>", shorten(text))
		return
	}
	if problem := checkText(text); problem != "" {
		c.add(offset, "<%s>: %s", parent, problem)
	}
}

// payload reports a data: URI or a base64 payload in value.
func (c *svgChecker) payload(offset int64, where, value string) {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "data:") {
		c.add(offset, "%s: contains a data: URI (embedded raster or payload)", where)
	}
	if strings.Contains(lower, "base64") {
		c.add(offset, "%s: contains a base64 payload", where)
	}
}

func shorten(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 48 {
		return s[:48] + "..."
	}
	return s
}
