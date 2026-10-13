// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// docWith builds a small document JSON text with the given pieces filled in.
func docWith(settings, nodes, edges, top string) string {
	return fmt.Sprintf(`{"format":"orcon/flow@1","id":"01930000-0000-7000-8000-0000000000aa","name":"X","settings":{%s},"variables":[],"nodes":[%s],"edges":[%s],"editor":{}%s}`,
		settings, nodes, edges, top)
}

const goodNode = `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{}}`

func TestFromValueAcceptsMinimalDocument(t *testing.T) {
	t.Parallel()
	v, err := jsonvalue.Parse([]byte(docWith("", goodNode, "", "")))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := flow.FromValue(v)
	if err != nil {
		t.Fatalf("FromValue error: %v", err)
	}
	if doc.ID != "01930000-0000-7000-8000-0000000000aa" || len(doc.Nodes) != 1 || doc.Nodes[0].ID != "a" {
		t.Errorf("FromValue = %+v", doc)
	}
}

func TestFromValueRejectsMalformedShape(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		json string
		want string // fragment of the issue path or message
	}{
		{"not an object", `[]`, "must be an object"},
		{"unknown top-level property", docWith("", goodNode, "", `,"surprise":1`), `unknown property "surprise"`},
		{"missing id", `{"format":"orcon/flow@1","name":"X","settings":{},"variables":[],"nodes":[],"edges":[],"editor":{}}`, `missing required property "id"`},
		{"missing editor", `{"format":"orcon/flow@1","id":"01930000-0000-7000-8000-0000000000aa","name":"X","settings":{},"variables":[],"nodes":[],"edges":[]}`, `missing required property "editor"`},
		{"name is a number", `{"format":"orcon/flow@1","id":"01930000-0000-7000-8000-0000000000aa","name":7,"settings":{},"variables":[],"nodes":[],"edges":[],"editor":{}}`, "must be a string"},
		{"empty description", docWith("", goodNode, "", `,"description":""`), "must not be empty"},
		{"settings not an object", `{"format":"orcon/flow@1","id":"01930000-0000-7000-8000-0000000000aa","name":"X","settings":[],"variables":[],"nodes":[],"edges":[],"editor":{}}`, "must be an object"},
		{"unknown setting", docWith(`"turbo":true`, goodNode, "", ""), `unknown property "turbo"`},
		{"concurrency decimal", docWith(`"concurrency":1.5`, goodNode, "", ""), "must be an integer from"},
		{"concurrency negative", docWith(`"concurrency":-2`, goodNode, "", ""), "must be an integer from"},
		{"concurrency zero is not the same as absent", docWith(`"concurrency":0`, goodNode, "", ""), "must be an integer from"},
		{"memory limit zero", docWith(`"memoryLimit":0`, goodNode, "", ""), "must be an integer from"},
		{"retry factor zero", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{},"retry":{"maxAttempts":2,"factor":0}}`, "", ""), "number other than zero"},
		{"keyword concurrency zero", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{},"concurrency":0}`, "", ""), "keyword string"},
		{"nodes not an array", `{"format":"orcon/flow@1","id":"01930000-0000-7000-8000-0000000000aa","name":"X","settings":{},"variables":[],"nodes":{},"edges":[],"editor":{}}`, "must be an array"},
		{"node not an object", docWith("", `7`, "", ""), "must be an object"},
		{"node missing parameters", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1"}`, "", ""), `missing required property "parameters"`},
		{"node unknown property", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{},"shade":"red"}`, "", ""), `unknown property "shade"`},
		{"parameters not an object", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":[]}`, "", ""), "must be an object"},
		{"disabled not a boolean", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{},"disabled":"no"}`, "", ""), "must be a boolean"},
		{"join missing data", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{},"join":{"mode":"ALL"}}`, "", ""), `missing required property "data"`},
		{"join unknown property", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{},"join":{"mode":"ALL","data":"APPEND","x":1}}`, "", ""), `unknown property "x"`},
		{"concurrency wrong type", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{},"concurrency":true}`, "", ""), "keyword string"},
		{"call missing version", docWith("", `{"id":"a","name":"A","element":"callActivity","type":"orcon.callFlow@1","parameters":{},"call":{"target":"y","mode":"WAIT","perItem":false}}`, "", ""), `missing required property "version"`},
		{"retry decimal attempts", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{},"retry":{"maxAttempts":2.5}}`, "", ""), "must be an integer from"},
		{"retry factor string", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{},"retry":{"maxAttempts":2,"factor":"2"}}`, "", ""), "number other than zero"},
		{"memory limit negative", docWith(`"memoryLimit":-1`, goodNode, "", ""), "must be an integer from"},
		{"edge missing kind", docWith("", goodNode, `{"id":"e","from":{"node":"a","port":"main"},"to":{"node":"a"}}`, ""), `missing required property "kind"`},
		{"edge from not an object", docWith("", goodNode, `{"id":"e","from":"a","to":{"node":"a"},"kind":"SEQUENCE"}`, ""), "must be an object"},
		{"edge order string", docWith("", goodNode, `{"id":"e","from":{"node":"a","port":"main"},"to":{"node":"a"},"kind":"SEQUENCE","order":"1"}`, ""), "must be an integer from 1 to"},
		{"writes is not a member of the format", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{},"writes":["v"]}`, "", ""), `unknown property "writes"`},
		{"effect is not a member of the format", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{},"effect":{"effect":"READ"}}`, "", ""), `unknown property "effect"`},
		{"edge order zero", docWith("", goodNode, `{"id":"e","from":{"node":"a","port":"main"},"to":{"node":"a"},"kind":"SEQUENCE","order":0}`, ""), "must be an integer from 1 to"},
		{"empty optional array", docWith("", `{"id":"a","name":"A","element":"callActivity","type":"orcon.callFlow@1","parameters":{},"call":{"target":"=x","allowedTargets":[],"version":"CURRENT","mode":"WAIT","perItem":false}}`, "", ""), "must not be empty"},
		{"empty loop carry", docWith("", `{"id":"a","name":"A","element":"task","type":"orcon.loop@1","parameters":{},"loop":{"maxIterations":2,"carry":[]}}`, "", ""), "must not be empty"},
		{"variable missing scope", `{"format":"orcon/flow@1","id":"01930000-0000-7000-8000-0000000000aa","name":"X","settings":{},"variables":[{"name":"v"}],"nodes":[],"edges":[],"editor":{}}`, `missing required property "scope"`},
		{"importedFrom incomplete", docWith(`"importedFrom":{"sourceVersion":"1"}`, goodNode, "", ""), `missing required property "reportId"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v, err := jsonvalue.Parse([]byte(tt.json))
			if err != nil {
				t.Fatalf("test input is not JSON: %v", err)
			}
			_, err = flow.FromValue(v)
			var ve *flow.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("FromValue error = %v, want *ValidationError", err)
			}
			found := false
			for _, issue := range ve.Report.Issues {
				if issue.Code != flow.CodeStructure {
					t.Errorf("issue code = %s, want %s", issue.Code, flow.CodeStructure)
				}
				if issue.Path == "" && tt.json != "[]" {
					t.Errorf("issue %q carries no JSON Pointer", issue.Message)
				}
				found = found || strings.Contains(issue.Message, tt.want)
			}
			if !found {
				t.Errorf("no issue mentions %q: %v", tt.want, ve.Report.Issues)
			}
		})
	}
}

func TestFromValueBoundsTheNumberOfIssues(t *testing.T) {
	t.Parallel()
	var nodes []string
	for range 500 {
		nodes = append(nodes, `7`)
	}
	v, err := jsonvalue.Parse([]byte(docWith("", strings.Join(nodes, ","), "", "")))
	if err != nil {
		t.Fatal(err)
	}
	_, err = flow.FromValue(v)
	var ve *flow.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("FromValue error = %v", err)
	}
	if got := len(ve.Report.Issues); got > 101 {
		t.Errorf("FromValue listed %d issues, want a bounded report of at most 101", got)
	}
}

func TestFromValueEscapesPointerSegments(t *testing.T) {
	t.Parallel()
	v, err := jsonvalue.Parse([]byte(docWith("", goodNode, "", `,"a/b~c":1`)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = flow.FromValue(v)
	if err == nil {
		t.Fatal("FromValue succeeded, want an unknown property error")
	}
	// Unknown properties are reported at the object that holds them.
	if !strings.Contains(err.Error(), "a/b~c") {
		t.Errorf("error %q does not quote the property", err)
	}
}

func TestValueOmitsAbsentAndKeepsPresentFalse(t *testing.T) {
	t.Parallel()
	doc := load(t, "effects-calls-and-settings")
	v := doc.Value()
	nodes, _ := v.Get("nodes")
	var lookup jsonvalue.Value
	for _, n := range nodes.Items() {
		if id, _ := mustMember(t, n, "id").Str(); id == "lookup" {
			lookup = n
		}
	}
	if d, ok := lookup.Get("disabled"); !ok || !jsonvalue.Equal(d, jsonvalue.Bool(false)) {
		t.Errorf("explicit disabled:false was lost: %v %v", d, ok)
	}
	if _, ok := lookup.Get("routing"); ok {
		t.Error("an absent routing was written")
	}
}
