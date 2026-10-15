// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func mustParse(t *testing.T, s string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(s))
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return v
}

func TestCoreNodesCatalogDescribesEveryType(t *testing.T) {
	c := NewCatalog()
	for _, typ := range []string{TypeNoOp, TypeSet, TypeStopAndError} {
		nt, ok := c.Lookup(t.Context(), typ)
		if !ok {
			t.Fatalf("Lookup(%s) not found", typ)
		}
		if nt.Purity != flow.PurityPure {
			t.Errorf("%s purity = %s, want PURE", typ, nt.Purity)
		}
		got, ok := c.ResolveCore(t.Context(), &flow.Node{Type: typ})
		if !ok || got != nt {
			t.Errorf("ResolveCore(%s) = %v, %v; want the Lookup answer", typ, got, ok)
		}
	}
}

func TestCoreNodesCatalogRejectsForeignTypes(t *testing.T) {
	c := NewCatalog()
	for _, typ := range []string{"orcon.http@1", "orcon.set@2", "orcon.noop", "slack.post@1", ""} {
		if _, ok := c.Lookup(t.Context(), typ); ok {
			t.Errorf("Lookup(%q) found a type this package does not define", typ)
		}
	}
	if _, ok := c.ResolveCore(t.Context(), nil); ok {
		t.Error("ResolveCore(nil) reported a type")
	}
}

func TestCoreNodesSetParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		wantErr bool
		fields  int
		include bool
	}{
		{"absent is no assignments", ``, false, 0, false},
		{"empty object", `{}`, false, 0, false},
		{"literal and include", `{"assignments":[{"name":"a","value":1}],"includeOtherFields":true}`, false, 1, true},
		{"expression value", `{"assignments":[{"name":"a","value":"=$json.x"}]}`, false, 1, false},
		{"not an object", `[]`, true, 0, false},
		{"unknown parameter", `{"assignment":[]}`, true, 0, false},
		{"assignments not an array", `{"assignments":{}}`, true, 0, false},
		{"assignment not an object", `{"assignments":[1]}`, true, 0, false},
		{"empty name", `{"assignments":[{"name":" ","value":1}]}`, true, 0, false},
		{"missing name", `{"assignments":[{"value":1}]}`, true, 0, false},
		{"missing value", `{"assignments":[{"name":"a"}]}`, true, 0, false},
		{"unknown assignment key", `{"assignments":[{"name":"a","value":1,"type":"string"}]}`, true, 0, false},
		{"duplicate name", `{"assignments":[{"name":"a","value":1},{"name":"a","value":2}]}`, true, 0, false},
		{"include not boolean", `{"includeOtherFields":"yes"}`, true, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var params jsonvalue.Value
			if tt.params != "" {
				params = mustParse(t, tt.params)
			}
			p, err := ParseSet(params)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSet err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(p.Assignments) != tt.fields || p.IncludeOtherFields != tt.include {
				t.Errorf("got %d assignments, include=%v; want %d, %v", len(p.Assignments), p.IncludeOtherFields, tt.fields, tt.include)
			}
		})
	}
}

type fakeEval struct {
	value jsonvalue.Value
	err   error
	got   []string
}

func (f *fakeEval) Evaluate(_ context.Context, expression string, _ jsonvalue.Value) (jsonvalue.Value, error) {
	f.got = append(f.got, expression)
	return f.value, f.err
}

func TestCoreNodesSetApply(t *testing.T) {
	ctx := t.Context()
	item := mustParse(t, `{"id":7,"name":"old"}`)

	t.Run("replaces existing field and drops others by default", func(t *testing.T) {
		p, err := ParseSet(mustParse(t, `{"assignments":[{"name":"name","value":"new"}]}`))
		if err != nil {
			t.Fatal(err)
		}
		out, err := ApplySet(ctx, p, nil, item)
		if err != nil {
			t.Fatal(err)
		}
		if got := marshal(t, out); got != `{"name":"new"}` {
			t.Errorf("got %s", got)
		}
	})

	t.Run("include keeps fields and replaces in place", func(t *testing.T) {
		p, _ := ParseSet(mustParse(t, `{"includeOtherFields":true,"assignments":[{"name":"name","value":"new"},{"name":"added","value":true}]}`))
		out, err := ApplySet(ctx, p, nil, item)
		if err != nil {
			t.Fatal(err)
		}
		if got := marshal(t, out); got != `{"id":7,"name":"new","added":true}` {
			t.Errorf("got %s", got)
		}
		if got := marshal(t, item); got != `{"id":7,"name":"old"}` {
			t.Errorf("input item changed to %s", got)
		}
	})

	t.Run("expression goes to the evaluator with the prefix removed", func(t *testing.T) {
		ev := &fakeEval{value: mustParse(t, `42`)}
		p, _ := ParseSet(mustParse(t, `{"assignments":[{"name":"n","value":"=$json.id"}]}`))
		out, err := ApplySet(ctx, p, ev, item)
		if err != nil {
			t.Fatal(err)
		}
		if got := marshal(t, out); got != `{"n":42}` {
			t.Errorf("got %s", got)
		}
		if len(ev.got) != 1 || ev.got[0] != "$json.id" {
			t.Errorf("evaluator got %v", ev.got)
		}
	})

	t.Run("expression without evaluator fails", func(t *testing.T) {
		p, _ := ParseSet(mustParse(t, `{"assignments":[{"name":"n","value":"=1"}]}`))
		if _, err := ApplySet(ctx, p, nil, item); err == nil {
			t.Error("expected an error without an evaluator")
		}
	})

	t.Run("evaluator error is returned", func(t *testing.T) {
		boom := errors.New("bad expression")
		p, _ := ParseSet(mustParse(t, `{"assignments":[{"name":"n","value":"=x"}]}`))
		if _, err := ApplySet(ctx, p, &fakeEval{err: boom}, item); !errors.Is(err, boom) {
			t.Errorf("err = %v, want %v", err, boom)
		}
	})

	t.Run("nested expression strings stay literal", func(t *testing.T) {
		p, _ := ParseSet(mustParse(t, `{"assignments":[{"name":"o","value":{"x":"=1"}}]}`))
		out, err := ApplySet(ctx, p, nil, item)
		if err != nil {
			t.Fatal(err)
		}
		if got := marshal(t, out); got != `{"o":{"x":"=1"}}` {
			t.Errorf("got %s", got)
		}
	})

	t.Run("non-object item is rejected", func(t *testing.T) {
		p, _ := ParseSet(mustParse(t, `{}`))
		if _, err := ApplySet(ctx, p, nil, mustParse(t, `[1]`)); err == nil {
			t.Error("expected an error for a non-object item")
		}
	})
}

func marshal(t *testing.T, v jsonvalue.Value) string {
	t.Helper()
	b, err := jsonvalue.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCoreNodesStopAndErrorParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		wantErr bool
	}{
		{"code only", `{"errorCode":"PAYMENT_DECLINED"}`, false},
		{"code and message", `{"errorCode":"A1","message":"stop"}`, false},
		{"missing code", `{"message":"x"}`, true},
		{"lower case code", `{"errorCode":"payment"}`, true},
		{"code with space", `{"errorCode":"BAD CODE"}`, true},
		{"empty code", `{"errorCode":""}`, true},
		{"message not string", `{"errorCode":"A","message":1}`, true},
		{"unknown key", `{"errorCode":"A","extra":true}`, true},
		{"not an object", `"A"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseStopAndError(mustParse(t, tt.params))
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCoreNodesStopAndErrorIsPermanentWithCode(t *testing.T) {
	r := NewRunner(nil)
	node := &flow.Node{Type: TypeStopAndError, Parameters: mustParse(t, `{"errorCode":"ORDER_REJECTED","message":"no stock"}`)}
	_, err := r.Execute(t.Context(), execution.NodeCall{Node: node})
	var ne *execution.NodeError
	if !errors.As(err, &ne) {
		t.Fatalf("err = %v, want a *NodeError", err)
	}
	if ne.Code != "ORDER_REJECTED" || ne.Class != execution.ClassPermanent {
		t.Errorf("code %q class %q; want ORDER_REJECTED PERMANENT", ne.Code, ne.Class)
	}
}
