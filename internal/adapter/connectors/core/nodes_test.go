// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// funcEval is an evaluator whose answer is a function of the template and the
// item, so a test can give each item its own verdict.
type funcEval struct {
	fn   func(template string, item jsonvalue.Value) (jsonvalue.Value, error)
	seen []string
}

func (f *funcEval) Evaluate(_ context.Context, template string, item jsonvalue.Value) (jsonvalue.Value, error) {
	f.seen = append(f.seen, template)
	return f.fn(template, item)
}

// okMember answers true for an item whose "ok" member is true.
func okMember() *funcEval {
	return &funcEval{fn: func(_ string, item jsonvalue.Value) (jsonvalue.Value, error) {
		v, _ := item.Get("ok")
		return v, nil
	}}
}

// run executes one node of the runner over the items and returns its main
// output.
func run(t *testing.T, ev Evaluator, typ, params string, env execution.NodeEnv, items ...string) ([]execution.Item, error) {
	t.Helper()
	call := execution.NodeCall{
		Node:  &flow.Node{Type: typ, Parameters: mustParse(t, params)},
		Input: input(t, items...),
		Env:   env,
	}
	out, err := NewRunner(ev).Execute(t.Context(), call)
	if err != nil {
		return nil, err
	}
	for _, p := range out.Ports {
		if p.Port == outputPort {
			return p.Batch.Items(), nil
		}
	}
	return nil, nil
}

func texts(t *testing.T, items []execution.Item) []string {
	t.Helper()
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = marshal(t, it.JSON)
	}
	return out
}

func wantTexts(t *testing.T, got []execution.Item, want ...string) {
	t.Helper()
	if g := texts(t, got); !slices.Equal(g, want) {
		t.Errorf("items = %v, want %v", g, want)
	}
}

func wantParamError(t *testing.T, err error) {
	t.Helper()
	var ne *execution.NodeError
	if !errors.As(err, &ne) || ne.Code != codeInvalidParameters || ne.Class != execution.ClassPermanent {
		t.Fatalf("err = %v, want a permanent %s", err, codeInvalidParameters)
	}
}

func TestCoreNodesFilterParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		wantErr bool
	}{
		{"plain text condition", `{"condition":"$json.age >= 18"}`, false},
		{"template condition", `{"condition":"={{ $json.age >= 18 }}"}`, false},
		{"missing", `{}`, true},
		{"empty", `{"condition":" "}`, true},
		{"not a string", `{"condition":true}`, true},
		{"unknown key", `{"condition":"true","keepDiscarded":true}`, true},
		{"not an object", `[]`, true},
		{"too long", `{"condition":"` + strings.Repeat("a", maxConditionBytes+1) + `"}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFilter(mustParse(t, tt.params))
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCoreNodesFilterKeepsMatchingItemsInOrder(t *testing.T) {
	ev := okMember()
	got, err := run(t, ev, TypeFilter, `{"condition":"$json.ok"}`, execution.NodeEnv{},
		`{"id":1,"ok":true}`, `{"id":2,"ok":false}`, `{"id":3,"ok":true}`)
	if err != nil {
		t.Fatal(err)
	}
	wantTexts(t, got, `{"id":1,"ok":true}`, `{"id":3,"ok":true}`)
	if got[1].Lineage[0].Index != 2 {
		t.Errorf("lineage of a kept item = %+v, want the one of input item 2", got[1].Lineage)
	}
	// Plain text is the code of one template segment; every item is tested alone.
	if len(ev.seen) != 3 || ev.seen[0] != "={{ $json.ok }}" {
		t.Errorf("evaluator saw %v", ev.seen)
	}
}

func TestCoreNodesFilterKeepsNothingAndEverything(t *testing.T) {
	none := &funcEval{fn: func(string, jsonvalue.Value) (jsonvalue.Value, error) { return jsonvalue.Bool(false), nil }}
	got, err := run(t, none, TypeFilter, `{"condition":"=false"}`, execution.NodeEnv{}, `{"a":1}`, `{"a":2}`)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want no items", got, err)
	}
	all := &funcEval{fn: func(string, jsonvalue.Value) (jsonvalue.Value, error) { return jsonvalue.Bool(true), nil }}
	got, err = run(t, all, TypeFilter, `{"condition":"=true"}`, execution.NodeEnv{}, `{"a":1}`, `{"a":2}`)
	if err != nil {
		t.Fatal(err)
	}
	wantTexts(t, got, `{"a":1}`, `{"a":2}`)
	got, err = run(t, all, TypeFilter, `{"condition":"=true"}`, execution.NodeEnv{})
	if err != nil || len(got) != 0 {
		t.Fatalf("empty input: got %v, %v", got, err)
	}
}

func TestCoreNodesFilterFailures(t *testing.T) {
	t.Run("a verdict that is not a boolean fails the node", func(t *testing.T) {
		ev := &funcEval{fn: func(string, jsonvalue.Value) (jsonvalue.Value, error) { return jsonvalue.String("yes"), nil }}
		_, err := run(t, ev, TypeFilter, `{"condition":"$json.x"}`, execution.NodeEnv{}, `{"x":1}`)
		var ne *execution.NodeError
		if !errors.As(err, &ne) || ne.Code != "FILTER_FAILED" || ne.Class != execution.ClassPermanent || !errors.Is(err, errNotBoolean) {
			t.Fatalf("err = %v, want a permanent FILTER_FAILED wrapping errNotBoolean", err)
		}
	})
	t.Run("an evaluator error fails the node", func(t *testing.T) {
		boom := errors.New("bad expression")
		ev := &funcEval{fn: func(string, jsonvalue.Value) (jsonvalue.Value, error) { return jsonvalue.Value{}, boom }}
		if _, err := run(t, ev, TypeFilter, `{"condition":"$json.x"}`, execution.NodeEnv{}, `{"x":1}`); !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
		}
	})
	t.Run("no evaluator fails the node", func(t *testing.T) {
		if _, err := run(t, nil, TypeFilter, `{"condition":"true"}`, execution.NodeEnv{}, `{}`); err == nil {
			t.Fatal("a filter without an evaluator ran")
		}
	})
	t.Run("invalid parameters", func(t *testing.T) {
		_, err := run(t, okMember(), TypeFilter, `{}`, execution.NodeEnv{}, `{}`)
		wantParamError(t, err)
	})
}

// bindingEval records the environment each activation is bound to.
type bindingEval struct {
	funcEval
	bound []execution.NodeEnv
}

func (b *bindingEval) Bind(env execution.NodeEnv) Evaluator { //nolint:ireturn // the port returns an evaluator
	b.bound = append(b.bound, env)
	return b
}

func TestCoreNodesRunnerBindsTheActivationEnvironment(t *testing.T) {
	ev := &bindingEval{funcEval: *okMember()}
	env := execution.NodeEnv{Seed: 99}
	if _, err := run(t, ev, TypeFilter, `{"condition":"$json.ok"}`, env, `{"ok":true}`); err != nil {
		t.Fatal(err)
	}
	if len(ev.bound) != 1 || ev.bound[0] != env {
		t.Errorf("bound to %v, want the environment of the activation", ev.bound)
	}
}

func TestCoreNodesLimitParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		want    LimitParams
		wantErr bool
	}{
		{"first by default", `{"maxItems":3}`, LimitParams{MaxItems: 3, Keep: KeepFirst}, false},
		{"last", `{"maxItems":2,"keep":"LAST_ITEMS"}`, LimitParams{MaxItems: 2, Keep: KeepLast}, false},
		{"missing maxItems", `{}`, LimitParams{}, true},
		{"zero", `{"maxItems":0}`, LimitParams{}, true},
		{"negative", `{"maxItems":-1}`, LimitParams{}, true},
		{"fraction", `{"maxItems":1.5}`, LimitParams{}, true},
		{"too many", `{"maxItems":1000000001}`, LimitParams{}, true},
		{"as text", `{"maxItems":"3"}`, LimitParams{}, true},
		{"expression", `{"maxItems":"=1"}`, LimitParams{}, true},
		{"bad keep", `{"maxItems":1,"keep":"first"}`, LimitParams{}, true},
		{"unknown key", `{"maxItems":1,"extra":1}`, LimitParams{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLimit(mustParse(t, tt.params))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCoreNodesLimitKeepsFirstOrLast(t *testing.T) {
	items := []string{`{"n":1}`, `{"n":2}`, `{"n":3}`}
	tests := []struct {
		name   string
		params string
		want   []string
	}{
		{"first two", `{"maxItems":2}`, []string{`{"n":1}`, `{"n":2}`}},
		{"last two", `{"maxItems":2,"keep":"LAST_ITEMS"}`, []string{`{"n":2}`, `{"n":3}`}},
		{"more than there are", `{"maxItems":10}`, items},
		{"exactly all", `{"maxItems":3,"keep":"LAST_ITEMS"}`, items},
		{"one", `{"maxItems":1}`, []string{`{"n":1}`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := run(t, nil, TypeLimit, tt.params, execution.NodeEnv{}, items...)
			if err != nil {
				t.Fatal(err)
			}
			wantTexts(t, got, tt.want...)
		})
	}
	got, err := run(t, nil, TypeLimit, `{"maxItems":2,"keep":"LAST_ITEMS"}`, execution.NodeEnv{}, items...)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Lineage[0].Index != 1 {
		t.Errorf("lineage of the first kept item = %+v, want input index 1", got[0].Lineage)
	}
	if got, err := run(t, nil, TypeLimit, `{"maxItems":2}`, execution.NodeEnv{}); err != nil || len(got) != 0 {
		t.Errorf("empty input: %v, %v", got, err)
	}
	_, err = run(t, nil, TypeLimit, `{}`, execution.NodeEnv{}, `{}`)
	wantParamError(t, err)
}

func TestCoreNodesSortParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		wantErr bool
	}{
		{"one field", `{"fields":[{"field":"age"}]}`, false},
		{"two fields with orders", `{"mode":"SIMPLE","fields":[{"field":"a","order":"DESCENDING"},{"field":"b.c","order":"ASCENDING"}]}`, false},
		{"random", `{"mode":"RANDOM"}`, false},
		{"random with empty fields", `{"mode":"RANDOM","fields":[]}`, false},
		{"random with fields", `{"mode":"RANDOM","fields":[{"field":"a"}]}`, true},
		{"simple without fields", `{}`, true},
		{"simple with empty fields", `{"fields":[]}`, true},
		{"bad mode", `{"mode":"CODE","fields":[{"field":"a"}]}`, true},
		{"field not an object", `{"fields":["a"]}`, true},
		{"field without name", `{"fields":[{"order":"ASCENDING"}]}`, true},
		{"empty name", `{"fields":[{"field":" "}]}`, true},
		{"expression name", `{"fields":[{"field":"=x"}]}`, true},
		{"bad order", `{"fields":[{"field":"a","order":"asc"}]}`, true},
		{"unknown field key", `{"fields":[{"field":"a","type":"number"}]}`, true},
		{"repeated field", `{"fields":[{"field":"a"},{"field":"a","order":"DESCENDING"}]}`, true},
		{"unknown key", `{"fields":[{"field":"a"}],"code":"x"}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSort(mustParse(t, tt.params))
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCoreNodesSortOrders(t *testing.T) {
	tests := []struct {
		name   string
		params string
		items  []string
		want   []string
	}{
		{"numbers ascending by value not text", `{"fields":[{"field":"n"}]}`,
			[]string{`{"n":10}`, `{"n":9}`, `{"n":100}`, `{"n":-1}`},
			[]string{`{"n":-1}`, `{"n":9}`, `{"n":10}`, `{"n":100}`}},
		{"numbers descending", `{"fields":[{"field":"n","order":"DESCENDING"}]}`,
			[]string{`{"n":1}`, `{"n":3}`, `{"n":2}`},
			[]string{`{"n":3}`, `{"n":2}`, `{"n":1}`}},
		{"strings by code unit", `{"fields":[{"field":"s"}]}`,
			[]string{`{"s":"b"}`, `{"s":"B"}`, `{"s":"a"}`},
			[]string{`{"s":"B"}`, `{"s":"a"}`, `{"s":"b"}`}},
		{"second field breaks ties", `{"fields":[{"field":"a"},{"field":"b","order":"DESCENDING"}]}`,
			[]string{`{"a":1,"b":1}`, `{"a":0,"b":5}`, `{"a":1,"b":2}`},
			[]string{`{"a":0,"b":5}`, `{"a":1,"b":2}`, `{"a":1,"b":1}`}},
		{"ties keep input order", `{"fields":[{"field":"a"}]}`,
			[]string{`{"a":1,"id":"x"}`, `{"a":0,"id":"y"}`, `{"a":1,"id":"z"}`},
			[]string{`{"a":0,"id":"y"}`, `{"a":1,"id":"x"}`, `{"a":1,"id":"z"}`}},
		{"missing and null come first when ascending", `{"fields":[{"field":"n"}]}`,
			[]string{`{"n":2}`, `{"x":1}`, `{"n":null}`, `{"n":1}`},
			[]string{`{"x":1}`, `{"n":null}`, `{"n":1}`, `{"n":2}`}},
		{"kinds rank boolean number string", `{"fields":[{"field":"v"}]}`,
			[]string{`{"v":"a"}`, `{"v":1}`, `{"v":true}`, `{"v":false}`},
			[]string{`{"v":false}`, `{"v":true}`, `{"v":1}`, `{"v":"a"}`}},
		{"nested path", `{"fields":[{"field":"u.age"}]}`,
			[]string{`{"u":{"age":30}}`, `{"u":{"age":20}}`},
			[]string{`{"u":{"age":20}}`, `{"u":{"age":30}}`}},
		{"a key with a dot wins over a path", `{"fields":[{"field":"a.b"}]}`,
			[]string{`{"a.b":2,"a":{"b":9}}`, `{"a.b":1,"a":{"b":0}}`},
			[]string{`{"a.b":1,"a":{"b":0}}`, `{"a.b":2,"a":{"b":9}}`}},
		{"objects by canonical text", `{"fields":[{"field":"o"}]}`,
			[]string{`{"o":{"b":1}}`, `{"o":{"a":2}}`},
			[]string{`{"o":{"a":2}}`, `{"o":{"b":1}}`}},
		{"a single item", `{"fields":[{"field":"a"}]}`, []string{`{"a":1}`}, []string{`{"a":1}`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := run(t, nil, TypeSort, tt.params, execution.NodeEnv{}, tt.items...)
			if err != nil {
				t.Fatal(err)
			}
			wantTexts(t, got, tt.want...)
		})
	}
}

func TestCoreNodesSortRandomIsAPermutationFixedByTheSeed(t *testing.T) {
	var items []string
	for i := range 20 {
		items = append(items, fmt.Sprintf(`{"n":%d}`, i))
	}
	first, err := run(t, nil, TypeSort, `{"mode":"RANDOM"}`, execution.NodeEnv{Seed: 7}, items...)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := run(t, nil, TypeSort, `{"mode":"RANDOM"}`, execution.NodeEnv{Seed: 7}, items...)
	other, _ := run(t, nil, TypeSort, `{"mode":"RANDOM"}`, execution.NodeEnv{Seed: 8}, items...)
	if !slices.Equal(texts(t, first), texts(t, again)) {
		t.Error("the same seed gave two orders")
	}
	if slices.Equal(texts(t, first), texts(t, other)) {
		t.Error("two seeds gave the same order of 20 items")
	}
	if slices.Equal(texts(t, first), items) {
		t.Error("the random order is the input order")
	}
	sorted := texts(t, first)
	slices.Sort(sorted)
	want := slices.Clone(items)
	slices.Sort(want)
	if !slices.Equal(sorted, want) {
		t.Errorf("the random order is not a permutation of the input: %v", sorted)
	}
}

func TestCoreNodesSortDoesNotChangeItsInput(t *testing.T) {
	in := input(t, `{"n":2}`, `{"n":1}`)
	call := execution.NodeCall{Node: &flow.Node{Type: TypeSort, Parameters: mustParse(t, `{"fields":[{"field":"n"}]}`)}, Input: in}
	if _, err := NewRunner(nil).Execute(t.Context(), call); err != nil {
		t.Fatal(err)
	}
	wantTexts(t, in.Items(), `{"n":2}`, `{"n":1}`)
}

func TestCoreNodesSortInvalidParameters(t *testing.T) {
	_, err := run(t, nil, TypeSort, `{}`, execution.NodeEnv{}, `{}`)
	wantParamError(t, err)
}
