// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

import (
	"bytes"
	"encoding/xml"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// attr is one attribute of an element being written.
type attr struct{ name, value string }

// xmlWriter writes indented XML. Elements holding text are never indented
// inside, so the text is exact. The first error is kept and later writes do
// nothing.
type xmlWriter struct {
	buf   bytes.Buffer
	depth int
	err   error
	// where is the JSON Pointer reported when a string cannot be written.
	where string
}

func (w *xmlWriter) header() {
	w.buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
}

func (w *xmlWriter) newline() {
	w.buf.WriteByte('\n')
	for range w.depth {
		w.buf.WriteString("  ")
	}
}

// start writes an opening tag and descends, or writes an empty element when
// selfClose is set.
func (w *xmlWriter) start(tag string, attrs []attr, selfClose bool) {
	if w.err != nil {
		return
	}
	w.newline()
	w.openTag(tag, attrs)
	if selfClose {
		w.buf.WriteString("/>")
		return
	}
	w.buf.WriteByte('>')
	w.depth++
}

func (w *xmlWriter) end(tag string) {
	if w.err != nil {
		return
	}
	w.depth--
	w.newline()
	w.buf.WriteString("</" + tag + ">")
}

// leaf writes an element holding only text.
func (w *xmlWriter) leaf(tag string, attrs []attr, text string) {
	if w.err != nil {
		return
	}
	w.newline()
	w.openTag(tag, attrs)
	w.buf.WriteByte('>')
	w.text(text)
	w.buf.WriteString("</" + tag + ">")
}

func (w *xmlWriter) openTag(tag string, attrs []attr) {
	w.buf.WriteByte('<')
	w.buf.WriteString(tag)
	for _, a := range attrs {
		w.buf.WriteByte(' ')
		w.buf.WriteString(a.name)
		w.buf.WriteString(`="`)
		w.text(a.value)
		w.buf.WriteByte('"')
	}
}

// text writes s escaped. A string XML 1.0 cannot carry fails the document
// instead of being altered.
func (w *xmlWriter) text(s string) {
	if w.err != nil {
		return
	}
	if !xmlSafe(s) {
		w.err = &UnrepresentableError{Pointer: w.where}
		return
	}
	if err := xml.EscapeText(&w.buf, []byte(s)); err != nil {
		w.err = err
	}
}

// xmlSafe reports whether s is valid UTF-8 and every rune of it is a Char of
// XML 1.0 (production 2). Invalid bytes would read back as U+FFFD, so they
// are refused like any other character XML cannot carry.
func xmlSafe(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		switch {
		case r == '\t', r == '\n', r == '\r':
		case r >= 0x20 && r <= 0xD7FF:
		case r >= 0xE000 && r <= 0xFFFD:
		case r >= 0x10000 && r <= 0x10FFFF:
		default:
			return false
		}
	}
	return true
}

// pointer appends one token to a JSON Pointer.
func pointer(base, token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	token = strings.ReplaceAll(token, "/", "~1")
	return base + "/" + token
}

// orcon returns the prefixed name of an extension element.
func orcon(local string) string { return "orcon:" + local }

// value writes v as a typed extension element. A member of an object carries
// its key in the attribute n; an item of an array carries none.
func (w *xmlWriter) value(key string, named bool, v jsonvalue.Value, where string) {
	if w.err != nil {
		return
	}
	var attrs []attr
	if named {
		attrs = []attr{{"n", key}}
	}
	w.where = where
	switch v.Kind() {
	case jsonvalue.KindString:
		s, _ := v.Str()
		w.leaf(orcon(kindStr), attrs, s)
	case jsonvalue.KindNumber:
		text, _ := v.NumberText()
		w.leaf(orcon(kindNum), attrs, text)
	case jsonvalue.KindBool:
		b, _ := v.Bool()
		text := "false"
		if b {
			text = "true"
		}
		w.leaf(orcon(kindBool), attrs, text)
	case jsonvalue.KindNull:
		w.start(orcon(kindNull), attrs, true)
	case jsonvalue.KindArray:
		items := v.Items()
		w.start(orcon(kindArr), attrs, len(items) == 0)
		if len(items) == 0 {
			return
		}
		for i, item := range items {
			w.value("", false, item, pointer(where, strconv.Itoa(i)))
		}
		w.end(orcon(kindArr))
	case jsonvalue.KindObject:
		members := v.Members()
		w.start(orcon(kindObj), attrs, len(members) == 0)
		if len(members) == 0 {
			return
		}
		for _, m := range members {
			w.value(m.Key, true, m.Value, pointer(where, m.Key))
		}
		w.end(orcon(kindObj))
	case jsonvalue.KindInvalid:
		// An absent value is never written: callers skip zero values.
	}
}
