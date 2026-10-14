// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// maxSVGBytes is the size ceiling of a brand SVG (SPEC-018 BC-1).
const maxSVGBytes = 16 * 1024

// bannedElements are the elements that can carry raster data, run code, load
// something from outside or stand for text that was not converted to outlines.
// Names are compared in lower case and without namespace prefix.
var bannedElements = map[string]bool{
	"image": true, "feimage": true, "foreignobject": true, "script": true,
	"iframe": true, "object": true, "embed": true, "video": true, "audio": true,
	"canvas": true, "text": true, "tspan": true, "textpath": true, "font": true,
	"font-face": true, "style": true,
}

// shapeElements are the elements that draw geometry; a vector asset has at
// least one.
var shapeElements = map[string]bool{
	"path": true, "polygon": true, "polyline": true, "rect": true,
	"circle": true, "ellipse": true, "line": true,
}

// urlReference finds the url() references inside attribute values.
var urlReference = regexp.MustCompile(`(?i)url\(\s*([^)]*)\)`)

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
	depth    int
	hasShape bool
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
			c.depth--
		case xml.CharData:
			c.payload(start, "text node", string(t))
		case xml.Directive:
			c.add(start, "directive <!%s> is not allowed (no DOCTYPE or entity declarations)", shorten(string(t)))
		case xml.ProcInst:
			if t.Target != "xml" {
				c.add(start, "processing instruction <?%s?> is not allowed", t.Target)
			}
		}
	}
	switch {
	case c.roots == 0:
		c.add(0, "no root element")
	case !c.hasShape:
		c.add(0, "no shape element (path, polygon, polyline, rect, circle, ellipse or line): not a drawn vector")
	}
}

func (c *svgChecker) start(offset int64, el xml.StartElement) {
	name := strings.ToLower(el.Name.Local)
	if c.depth == 0 {
		c.roots++
		switch {
		case c.roots > 1:
			c.add(offset, "more than one root element")
		case name != "svg":
			c.add(offset, "root element is <%s>, want <svg>", el.Name.Local)
		case !hasViewBox(el):
			c.add(offset, "root <svg> has no viewBox")
		}
	}
	c.depth++
	if bannedElements[name] {
		c.add(offset, "element <%s> is not allowed in a vector asset", el.Name.Local)
	}
	if shapeElements[name] {
		c.hasShape = true
	}
	for _, a := range el.Attr {
		c.attr(offset, el.Name.Local, a)
	}
}

func hasViewBox(el xml.StartElement) bool {
	for _, a := range el.Attr {
		if a.Name.Space == "" && a.Name.Local == "viewBox" && strings.TrimSpace(a.Value) != "" {
			return true
		}
	}
	return false
}

func (c *svgChecker) attr(offset int64, element string, a xml.Attr) {
	if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
		return
	}
	name := strings.ToLower(a.Name.Local)
	where := fmt.Sprintf("<%s %s>", element, a.Name.Local)
	if len(name) > 2 && strings.HasPrefix(name, "on") {
		c.add(offset, "%s: event handler attributes are not allowed", where)
	}
	c.payload(offset, where, a.Value)
	if (name == "href" || name == "src") && !strings.HasPrefix(strings.TrimSpace(a.Value), "#") {
		c.add(offset, "%s: reference %q is not a local #fragment (no external resources)", where, shorten(a.Value))
	}
	for _, m := range urlReference.FindAllStringSubmatch(a.Value, -1) {
		target := strings.Trim(strings.TrimSpace(m[1]), `"'`)
		if len(target) < 2 || target[0] != '#' {
			c.add(offset, "%s: url(%s) is not a local url(#id)", where, shorten(target))
		}
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
