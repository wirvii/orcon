// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// generatedFlows is the number of valid generated flows that must round-trip
// (criterion E-FMT-4 asks for at least 1000).
const generatedFlows = 1000

func TestRoundTripConformance(t *testing.T) {
	flows := conformanceFlows(t)
	supported := 0
	for name, doc := range flows {
		t.Run(name, func(t *testing.T) {
			if deferredFlows[name] {
				_, err := Export(doc)
				var unsupported *UnsupportedError
				if !errors.As(err, &unsupported) {
					t.Fatalf("a deferred flow must fail with UnsupportedError, got %v", err)
				}
				return
			}
			data := roundTrip(t, doc)
			checkStructure(t, data)
		})
		if !deferredFlows[name] {
			supported++
		}
	}
	if supported < 7 {
		t.Fatalf("%d conformance flows round-trip; the C3 set has at least 7", supported)
	}
}

func TestRoundTripKeepsJoinRemaining(t *testing.T) {
	doc := conformanceFlows(t)["discriminator"]
	data := roundTrip(t, doc)
	for _, want := range []string{
		`<orcon:str n="remaining">LET_FINISH</orcon:str>`,
		`<orcon:str n="remaining">CANCEL</orcon:str>`,
		`<bpmn:activationCondition>1</bpmn:activationCondition>`,
	} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("exported XML lacks %s", want)
		}
	}
}

func TestRoundTripNativeMapping(t *testing.T) {
	flows := conformanceFlows(t)
	doc := flows["exclusive-and-inclusive"]
	data := string(roundTrip(t, doc))
	for _, want := range []string{
		`xmlns:orcon="urn:orcon:bpmn:1"`,
		`<bpmn:conditionExpression xsi:type="bpmn:tFormalExpression" language="urn:orcon:expr:1">`,
		`<bpmn:sequenceFlow id="edge_`,
		`<bpmndi:BPMNDiagram`,
	} {
		if !strings.Contains(data, want) {
			t.Errorf("exported XML lacks %s", want)
		}
	}
	loop := flows["loop"]
	if data := string(roundTrip(t, loop)); !strings.Contains(data, `orcon:loopControl="true"`) {
		t.Error("loop control node lacks orcon:loopControl")
	}
	routing := flows["item-routing"]
	if data := string(roundTrip(t, routing)); !strings.Contains(data, `orcon:itemRouting="`) {
		t.Error("item-routing node lacks orcon:itemRouting")
	}
}

func TestRoundTripDeterministic(t *testing.T) {
	for name, doc := range supportedFlows(t) {
		first, err := Export(doc)
		if err != nil {
			t.Fatal(err)
		}
		second, err := Export(doc)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, second) {
			t.Errorf("%s: two exports differ", name)
		}
		back, err := Import(first)
		if err != nil {
			t.Fatal(err)
		}
		again, err := Export(&back)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, again) {
			t.Errorf("%s: export(import(export(f))) differs from export(f)", name)
		}
	}
}

// generatedValidFlows returns n valid flows built from the supported ones with
// random open-ended data. The sequence is the same on every run.
func generatedValidFlows(t *testing.T, n int) []*flow.Document {
	t.Helper()
	flows := supportedFlows(t)
	bases := make([]string, 0, len(flows))
	for name := range flows {
		bases = append(bases, name)
	}
	slices.Sort(bases)

	rng := rand.New(rand.NewPCG(20261013, 14)) //nolint:gosec // G404: deterministic test data
	var out []*flow.Document
	for attempts := 0; len(out) < n; attempts++ {
		if attempts > n*4 {
			t.Fatalf("only %d of %d generated flows were valid", len(out), attempts)
		}
		base := flows[bases[rng.IntN(len(bases))]]
		doc := mutate(rng, base, true)
		if flow.Validate(doc).HasErrors() {
			// Renaming breaks $('Name') references; keep the names then.
			doc = mutate(rng, base, false)
		}
		if flow.Validate(doc).HasErrors() {
			continue
		}
		out = append(out, doc)
	}
	return out
}

func TestRoundTripGenerated(t *testing.T) {
	docs := generatedValidFlows(t, generatedFlows)
	for _, doc := range docs {
		roundTrip(t, doc)
	}
	t.Logf("%d valid generated flows round-tripped", len(docs))
}

func TestExportRefusesUnrepresentableStrings(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // G404: deterministic test data
	doc := mutate(rng, conformanceFlows(t)["linear"], false)
	doc.Name = "bad\x01name"
	_, err := Export(doc)
	var bad *UnrepresentableError
	if !errors.As(err, &bad) {
		t.Fatalf("want UnrepresentableError, got %v", err)
	}
	if bad.Pointer != "/name" {
		t.Errorf("pointer = %q, want /name", bad.Pointer)
	}

	doc = mutate(rng, conformanceFlows(t)["linear"], false)
	v := jsonvalue.String("tab\there \x00")
	obj, err := jsonvalue.Object(jsonvalue.Member{Key: "k/1", Value: v})
	if err != nil {
		t.Fatal(err)
	}
	doc.Nodes[0].Parameters = obj
	_, err = Export(doc)
	if !errors.As(err, &bad) {
		t.Fatalf("want UnrepresentableError, got %v", err)
	}
	if want := "/nodes/0/parameters/k~11"; bad.Pointer != want {
		t.Errorf("pointer = %q, want %q", bad.Pointer, want)
	}
}

func TestExportRefusesInvalidUTF8(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4)) //nolint:gosec // G404: deterministic test data
	for _, name := range []string{"bad\xffname", "cut\xe4\xb8"} {
		doc := mutate(rng, conformanceFlows(t)["linear"], false)
		doc.Name = name
		_, err := Export(doc)
		var bad *UnrepresentableError
		if !errors.As(err, &bad) || bad.Pointer != "/name" {
			t.Errorf("%q: want UnrepresentableError at /name, got %v", name, err)
		}
	}
}

func TestExportRefusesInvalidFlows(t *testing.T) {
	doc := conformanceFlows(t)["linear"]
	doc.Nodes = append(slices.Clone(doc.Nodes), doc.Nodes[0]) // duplicate id
	_, err := Export(doc)
	var ve *flow.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

func TestExportRefusesLaterCuts(t *testing.T) {
	for name := range deferredFlows {
		doc := conformanceFlows(t)[name]
		_, err := Export(doc)
		var un *UnsupportedError
		if !errors.As(err, &un) {
			t.Fatalf("%s: want UnsupportedError, got %v", name, err)
		}
		if !strings.HasPrefix(un.Pointer, "/nodes/") || un.What == "" {
			t.Errorf("%s: unhelpful error %+v", name, un)
		}
	}
}

// mutate returns a copy of base with random but well-formed data in every
// open-ended place: names, description, conditions, parameters, extension
// members and the editor. Names stay unique when rename is set;
// otherwise they are kept, because expressions refer to them. The strings use only
// characters XML 1.0 carries.
func mutate(rng *rand.Rand, base *flow.Document, rename bool) *flow.Document {
	doc := *base
	doc.Nodes = slices.Clone(base.Nodes)
	doc.Edges = slices.Clone(base.Edges)
	g := &gen{rng: rng}

	doc.Name = g.str(1, 40)
	doc.Description = ""
	if rng.IntN(2) == 0 {
		doc.Description = g.str(1, 200)
	}
	for i := range doc.Nodes {
		if rename {
			doc.Nodes[i].Name = fmt.Sprintf("%s #%d", g.str(0, 20), i)
		}
		if rng.IntN(3) > 0 {
			doc.Nodes[i].Parameters = g.object(0)
		}
		if rng.IntN(4) == 0 {
			doc.Nodes[i].Ext = g.extObject(i)
		}
	}
	for i := range doc.Edges {
		if doc.Edges[i].Condition != "" || rng.IntN(3) == 0 {
			doc.Edges[i].Condition = "=" + g.str(1, 60)
		}
	}
	doc.Editor = g.object(0)
	doc.Ext = jsonvalue.Value{}
	if rng.IntN(2) == 0 {
		doc.Ext = g.extObject(-1)
	}
	return &doc
}

type gen struct {
	rng *rand.Rand
}

// alphabet mixes ordinary text with the characters that XML escaping, JSON
// escaping and whitespace handling get wrong most easily.
var alphabet = []rune("abcXYZ019 _-.:/\\\"'<>&;=${}()[]{{ }}\t\n\r  éñ中\U0001F600 ")

func (g *gen) str(minLen, maxLen int) string {
	n := minLen
	if maxLen > minLen {
		n += g.rng.IntN(maxLen - minLen + 1)
	}
	var b strings.Builder
	for range n {
		b.WriteRune(alphabet[g.rng.IntN(len(alphabet))])
	}
	s := b.String()
	if g.rng.IntN(8) == 0 {
		s = " " + s + " "
	}
	return s
}

func (g *gen) value(depth int) jsonvalue.Value {
	pick := g.rng.IntN(8)
	if depth >= 4 && pick >= 6 {
		pick = 0
	}
	switch pick {
	case 0, 1:
		return jsonvalue.String(g.str(0, 30))
	case 2:
		return jsonvalue.Int(g.rng.Int64N(1<<53-1) - g.rng.Int64N(1<<53-1))
	case 3:
		v, err := jsonvalue.Float(float64(g.rng.IntN(100000)) / 64)
		if err != nil {
			return jsonvalue.Null()
		}
		return v
	case 4:
		return jsonvalue.Bool(g.rng.IntN(2) == 0)
	case 5:
		return jsonvalue.Null()
	case 6:
		items := make([]jsonvalue.Value, g.rng.IntN(4))
		for i := range items {
			items[i] = g.value(depth + 1)
		}
		return jsonvalue.Array(items...)
	default:
		return g.object(depth + 1)
	}
}

func (g *gen) object(depth int) jsonvalue.Value {
	n := g.rng.IntN(5)
	members := make([]jsonvalue.Member, 0, n)
	seen := map[string]bool{}
	for range n {
		k := g.str(0, 12)
		if seen[k] {
			continue
		}
		seen[k] = true
		members = append(members, jsonvalue.Member{Key: k, Value: g.value(depth)})
	}
	v, err := jsonvalue.Object(members...)
	if err != nil {
		return jsonvalue.Value{}
	}
	return v
}

// extObject returns an ext object whose keys are namespaced, as the format
// requires.
func (g *gen) extObject(salt int) jsonvalue.Value {
	n := 1 + g.rng.IntN(3)
	members := make([]jsonvalue.Member, 0, n)
	for i := range n {
		members = append(members, jsonvalue.Member{
			Key:   fmt.Sprintf("vendor.k%d_%d", salt+1, i),
			Value: g.value(1),
		})
	}
	v, err := jsonvalue.Object(members...)
	if err != nil {
		return jsonvalue.Value{}
	}
	return v
}
