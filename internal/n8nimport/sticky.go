// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Limits of an editor sticky note (SPEC-014 R-ED-CAN-08).
const (
	// MaxStickyContent is the longest note, in characters.
	MaxStickyContent = 10_000
	// StickyColors is the number of decorative colors of the editor.
	StickyColors = 6

	defaultStickyWidth  = 240
	defaultStickyHeight = 160
)

// FindAdaptedSticky reports that a sticky note was changed to fit the editor.
const FindAdaptedSticky FindingCode = "ADAPTED"

// StickyNote is an editor sticky note, the Orcon counterpart of the n8n
// Sticky Note node (ADR-0007 section 2: editor data, not a node).
type StickyNote struct {
	// Content is Markdown without raw HTML; the editor enforces that when it
	// renders, the importer keeps the text as written.
	Content       string
	X, Y          int
	Width, Height int
	// Color is one of 1 to StickyColors.
	Color int
}

// StickyNoteOf converts an n8n Sticky Note node. The n8n color is assigned to
// one of the six editor colors by order, and one out of range takes color 1;
// content over [MaxStickyContent] characters is cut and the cut is reported as
// an adaptation. The conversion never fails a template.
func StickyNoteOf(n *Node) (StickyNote, []Finding) {
	note := StickyNote{
		X: n.Position[0], Y: n.Position[1],
		Width: intParam(n, "width", defaultStickyWidth), Height: intParam(n, "height", defaultStickyHeight),
		Color: 1,
	}
	if p, ok := n.Parameters.Get("content"); ok {
		note.Content, _ = p.Str()
	}
	var findings []Finding
	if utf8.RuneCountInString(note.Content) > MaxStickyContent {
		note.Content = string([]rune(note.Content)[:MaxStickyContent])
		findings = append(findings, Finding{
			Code: FindAdaptedSticky, NodeID: n.ID, Node: n.Name, Field: "parameters.content",
			Message: "the note is longer than the editor allows and was cut",
		})
	}
	if c := intParam(n, "color", 0); c >= 1 && c <= StickyColors {
		note.Color = c
	}
	return note, findings
}

func intParam(n *Node, key string, def int) int {
	p, ok := n.Parameters.Get(key)
	if !ok {
		return def
	}
	f, ok := p.Float64()
	if !ok || f < 0 || f > 1<<20 {
		return def
	}
	return int(f)
}

// Value returns the note as the member of editor.stickyNotes[].
func (s StickyNote) Value() jsonvalue.Value {
	v, err := jsonvalue.Object(
		jsonvalue.Member{Key: "content", Value: jsonvalue.String(s.Content)},
		jsonvalue.Member{Key: "x", Value: jsonvalue.Int(int64(s.X))},
		jsonvalue.Member{Key: "y", Value: jsonvalue.Int(int64(s.Y))},
		jsonvalue.Member{Key: "width", Value: jsonvalue.Int(int64(s.Width))},
		jsonvalue.Member{Key: "height", Value: jsonvalue.Int(int64(s.Height))},
		jsonvalue.Member{Key: "color", Value: jsonvalue.Int(int64(s.Color))},
	)
	if err != nil {
		return jsonvalue.Value{}
	}
	return v
}

// StickyNotes converts every sticky note of a template, in node order.
func StickyNotes(t *Template) ([]StickyNote, []Finding) {
	var notes []StickyNote
	var findings []Finding
	for i := range t.Nodes {
		if IsStickyNote(t.Nodes[i].Type) {
			note, f := StickyNoteOf(&t.Nodes[i])
			notes = append(notes, note)
			findings = append(findings, f...)
		}
	}
	return notes, findings
}
