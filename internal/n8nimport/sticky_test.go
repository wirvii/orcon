// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/n8nimport"
)

func sticky(t *testing.T, params string) n8nimport.Node {
	t.Helper()
	return importNode(t, `{"id":"s","name":"Note","type":"n8n-nodes-base.stickyNote","typeVersion":1,"position":[120.4,-80],"parameters":`+params+`}`)
}

func TestStickyNoteConversion(t *testing.T) {
	t.Run("fields and position", func(t *testing.T) {
		n := sticky(t, `{"content":"## Title","width":300,"height":180,"color":4}`)
		note, findings := n8nimport.StickyNoteOf(&n)
		want := n8nimport.StickyNote{Content: "## Title", X: 120, Y: -80, Width: 300, Height: 180, Color: 4}
		if note != want || len(findings) != 0 {
			t.Fatalf("note = %+v findings %v, want %+v", note, findings, want)
		}
		if got := fmt.Sprint(note.Value()); !strings.Contains(got, "Title") {
			t.Errorf("value = %s", got)
		}
	})
	t.Run("defaults", func(t *testing.T) {
		n := sticky(t, `{}`)
		note, _ := n8nimport.StickyNoteOf(&n)
		if note.Width != 240 || note.Height != 160 || note.Color != 1 || note.Content != "" {
			t.Errorf("note = %+v", note)
		}
	})
	t.Run("colors", func(t *testing.T) {
		for color, want := range map[string]int{"1": 1, "6": 6, "7": 1, "0": 1, "-3": 1, "99": 1, `"red"`: 1} {
			n := sticky(t, `{"color":`+color+`}`)
			if note, _ := n8nimport.StickyNoteOf(&n); note.Color != want {
				t.Errorf("color %s -> %d, want %d", color, note.Color, want)
			}
		}
	})
	t.Run("long content is cut and reported", func(t *testing.T) {
		n := sticky(t, `{"content":"`+strings.Repeat("é", n8nimport.MaxStickyContent+5)+`"}`)
		note, findings := n8nimport.StickyNoteOf(&n)
		if got := len([]rune(note.Content)); got != n8nimport.MaxStickyContent {
			t.Errorf("content has %d characters", got)
		}
		if len(findings) != 1 || findings[0].Code != n8nimport.FindAdapted || findings[0].Field != "parameters.content" || findings[0].Node != "Note" {
			t.Errorf("findings = %+v", findings)
		}
	})
	t.Run("absurd sizes fall back", func(t *testing.T) {
		n := sticky(t, `{"width":-5,"height":1e12}`)
		if note, _ := n8nimport.StickyNoteOf(&n); note.Width != 240 || note.Height != 160 {
			t.Errorf("note = %+v", note)
		}
	})
}

func TestStickyNotesAreNotNodes(t *testing.T) {
	tp, err := n8nimport.ParseTemplate([]byte(tpl(1, 1,
		node("Start", manual, 1, ""),
		`{"name":"A","type":"n8n-nodes-base.stickyNote","typeVersion":1,"position":[1,2],"parameters":{"content":"a"}}`,
		`{"name":"B","type":"n8n-nodes-base.stickyNote","typeVersion":1,"position":[3,4],"parameters":{"content":"b","color":2}}`)))
	if err != nil {
		t.Fatal(err)
	}
	notes, findings := n8nimport.StickyNotes(&tp)
	if len(notes) != 2 || len(findings) != 0 || notes[0].Content != "a" || notes[1].Color != 2 {
		t.Fatalf("notes = %+v findings %v", notes, findings)
	}
	res := n8nimport.Classify(&tp, nil)
	if res.Nodes != 1 || res.StickyNotes != 2 || !res.Importable() {
		t.Errorf("result = %+v: sticky notes are not nodes and never block", res)
	}
	r := n8nimport.Measure(&n8nimport.Corpus{Templates: []n8nimport.Template{tp}}, nil, "core")
	for _, ts := range r.Types {
		if ts.Type == "n8n-nodes-base.stickyNote" {
			t.Error("the sticky note is listed as a node type")
		}
	}
	if r.Counts.StickyNotes != 2 {
		t.Errorf("sticky notes counted = %d", r.Counts.StickyNotes)
	}
}
