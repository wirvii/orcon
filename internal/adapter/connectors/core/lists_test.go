// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"slices"
	"testing"

	"github.com/wirvii/orcon/internal/execution"
)

func wantFailure(t *testing.T, err error, code string) {
	t.Helper()
	var ne *execution.NodeError
	if !errors.As(err, &ne) || ne.Code != code || ne.Class != execution.ClassPermanent {
		t.Fatalf("err = %v, want a permanent %s", err, code)
	}
}

func TestCoreNodesDecimalKeyIsOneFormPerValue(t *testing.T) {
	same := [][]string{
		{"1", "1.0", "1e0", "10e-1", "0.1e1", "1.000"},
		{"0", "-0", "0.0", "0e5", "-0.0e-3"},
		{"100", "1e2", "1.0E+2", "0.1e3"},
		{"-12.5", "-1.25e1", "-125e-1"},
		{"9007199254740993", "9007199254740993.0"},
	}
	for _, group := range same {
		for _, other := range group[1:] {
			if a, b := decimalKey(group[0]), decimalKey(other); a != b {
				t.Errorf("decimalKey(%s) = %s, decimalKey(%s) = %s; want equal", group[0], a, other, b)
			}
		}
	}
	different := [][2]string{
		{"1", "-1"}, {"1", "10"}, {"0.1", "0.01"}, {"9007199254740993", "9007199254740992"},
		{"1e2", "1e3"}, {"12", "1.2"},
	}
	for _, p := range different {
		if decimalKey(p[0]) == decimalKey(p[1]) {
			t.Errorf("decimalKey(%s) equals decimalKey(%s)", p[0], p[1])
		}
	}
}

func TestCoreNodesValueKeySeparatesDifferentValues(t *testing.T) {
	equal := [][2]string{
		{`{"a":1,"b":[1,2]}`, `{"b":[1,2],"a":1.0}`},
		{`"x"`, `"x"`},
		{`{"o":{"y":1,"x":2}}`, `{"o":{"x":2,"y":1}}`},
	}
	for _, p := range equal {
		if valueKey(mustParse(t, p[0])) != valueKey(mustParse(t, p[1])) {
			t.Errorf("%s and %s have different keys", p[0], p[1])
		}
	}
	different := [][2]string{
		{`1`, `"1"`}, {`true`, `"true"`}, {`null`, `"null"`}, {`[1,2]`, `[2,1]`}, {`["a","b"]`, `["ab"]`},
		{`["a:b"]`, `["a","b"]`}, {`{"a":1}`, `{"a":"1"}`}, {`[]`, `{}`}, {`{"a":null}`, `{}`},
	}
	for _, p := range different {
		if valueKey(mustParse(t, p[0])) == valueKey(mustParse(t, p[1])) {
			t.Errorf("%s and %s have the same key", p[0], p[1])
		}
	}
}

func TestCoreNodesRemoveDuplicatesParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		wantErr bool
	}{
		{"all fields by default", `{}`, false},
		{"all fields explicit", `{"compare":"ALL_FIELDS"}`, false},
		{"all fields but some", `{"compare":"ALL_FIELDS_EXCEPT","fields":["updatedAt"]}`, false},
		{"selected", `{"compare":"SELECTED_FIELDS","fields":["email","a.b"]}`, false},
		{"selected and remove others", `{"compare":"SELECTED_FIELDS","fields":["email"],"removeOtherFields":true}`, false},
		{"all fields with a list", `{"fields":["a"]}`, true},
		{"except without fields", `{"compare":"ALL_FIELDS_EXCEPT"}`, true},
		{"selected without fields", `{"compare":"SELECTED_FIELDS","fields":[]}`, true},
		{"remove others with all fields", `{"removeOtherFields":true}`, true},
		{"remove others with except", `{"compare":"ALL_FIELDS_EXCEPT","fields":["a"],"removeOtherFields":true}`, true},
		{"bad compare", `{"compare":"allFields"}`, true},
		{"repeated field", `{"compare":"SELECTED_FIELDS","fields":["a","a"]}`, true},
		{"field not a string", `{"compare":"SELECTED_FIELDS","fields":[1]}`, true},
		{"expression field", `{"compare":"SELECTED_FIELDS","fields":["=a"]}`, true},
		{"fields not a list", `{"compare":"SELECTED_FIELDS","fields":"a"}`, true},
		{"remove others not a boolean", `{"compare":"SELECTED_FIELDS","fields":["a"],"removeOtherFields":"yes"}`, true},
		{"unknown key", `{"operation":"x"}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRemoveDuplicates(mustParse(t, tt.params))
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCoreNodesRemoveDuplicatesKeepsTheFirstOfEachGroup(t *testing.T) {
	tests := []struct {
		name   string
		params string
		items  []string
		want   []string
	}{
		{"whole items, member order and number spelling do not matter", `{}`,
			[]string{`{"a":1,"b":2}`, `{"b":2,"a":1.0}`, `{"a":1,"b":3}`, `{"a":1,"b":2}`},
			[]string{`{"a":1,"b":2}`, `{"a":1,"b":3}`}},
		{"a string is not a number", `{}`,
			[]string{`{"a":1}`, `{"a":"1"}`}, []string{`{"a":1}`, `{"a":"1"}`}},
		{"long integers a double cannot tell apart stay apart", `{}`,
			[]string{`{"id":9007199254740993}`, `{"id":9007199254740992}`, `{"id":9007199254740993}`},
			[]string{`{"id":9007199254740993}`, `{"id":9007199254740992}`}},
		{"except ignores a field", `{"compare":"ALL_FIELDS_EXCEPT","fields":["seen"]}`,
			[]string{`{"id":1,"seen":"a"}`, `{"id":1,"seen":"b"}`, `{"id":2,"seen":"a"}`},
			[]string{`{"id":1,"seen":"a"}`, `{"id":2,"seen":"a"}`}},
		{"except ignores a nested field", `{"compare":"ALL_FIELDS_EXCEPT","fields":["meta.at"]}`,
			[]string{`{"id":1,"meta":{"at":1,"x":1}}`, `{"id":1,"meta":{"at":2,"x":1}}`, `{"id":1,"meta":{"at":2,"x":2}}`},
			[]string{`{"id":1,"meta":{"at":1,"x":1}}`, `{"id":1,"meta":{"at":2,"x":2}}`}},
		{"selected compares only those", `{"compare":"SELECTED_FIELDS","fields":["email"]}`,
			[]string{`{"email":"a","n":1}`, `{"email":"b","n":2}`, `{"email":"a","n":3}`},
			[]string{`{"email":"a","n":1}`, `{"email":"b","n":2}`}},
		{"selected treats missing as one value", `{"compare":"SELECTED_FIELDS","fields":["email"]}`,
			[]string{`{"n":1}`, `{"n":2}`, `{"email":null,"n":3}`},
			[]string{`{"n":1}`, `{"email":null,"n":3}`}},
		{"selected pairs do not run together", `{"compare":"SELECTED_FIELDS","fields":["a","b"]}`,
			[]string{`{"a":"x","b":"yz"}`, `{"a":"xy","b":"z"}`},
			[]string{`{"a":"x","b":"yz"}`, `{"a":"xy","b":"z"}`}},
		{"selected by nested path", `{"compare":"SELECTED_FIELDS","fields":["u.id"]}`,
			[]string{`{"u":{"id":1},"n":1}`, `{"u":{"id":1},"n":2}`}, []string{`{"u":{"id":1},"n":1}`}},
		{"remove other fields", `{"compare":"SELECTED_FIELDS","fields":["email"],"removeOtherFields":true}`,
			[]string{`{"email":"a","n":1}`, `{"email":"a","n":2}`, `{"email":"b","n":3}`},
			[]string{`{"email":"a"}`, `{"email":"b"}`}},
		{"no items", `{}`, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := run(t, nil, TypeRemoveDuplicates, tt.params, execution.NodeEnv{}, tt.items...)
			if err != nil {
				t.Fatal(err)
			}
			wantTexts(t, got, tt.want...)
		})
	}
	got, err := run(t, nil, TypeRemoveDuplicates, `{}`, execution.NodeEnv{}, `{"a":1}`, `{"a":1}`, `{"a":2}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Lineage[0].Index != 2 {
		t.Errorf("lineage of the kept items = %+v", got)
	}
}

func TestCoreNodesRemoveDuplicatesFailures(t *testing.T) {
	_, err := run(t, nil, TypeRemoveDuplicates, `{}`, execution.NodeEnv{}, `{"a":1}`, `[1]`)
	wantFailure(t, err, "REMOVE_DUPLICATES_FAILED")
	_, err = run(t, nil, TypeRemoveDuplicates, `{"compare":"SELECTED_FIELDS"}`, execution.NodeEnv{}, `{}`)
	wantParamError(t, err)
}

func TestCoreNodesAggregateParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		wantErr bool
	}{
		{"one field", `{"fields":[{"field":"name"}]}`, false},
		{"renamed fields and flags", `{"mode":"INDIVIDUAL_FIELDS","fields":[{"field":"a.b","outputField":"x"},{"field":"c"}],"mergeLists":true,"keepMissing":true}`, false},
		{"all item data with defaults", `{"mode":"ALL_ITEM_DATA"}`, false},
		{"all item data selected", `{"mode":"ALL_ITEM_DATA","destinationField":"rows","include":"SELECTED_FIELDS","includeFields":["a"]}`, false},
		{"all item data except", `{"mode":"ALL_ITEM_DATA","include":"ALL_FIELDS_EXCEPT","includeFields":["a"]}`, false},
		{"no parameters", `{}`, true},
		{"empty fields", `{"fields":[]}`, true},
		{"field without name", `{"fields":[{"outputField":"x"}]}`, true},
		{"field not an object", `{"fields":["a"]}`, true},
		{"unknown field key", `{"fields":[{"field":"a","as":"b"}]}`, true},
		{"same output twice", `{"fields":[{"field":"a"},{"field":"b","outputField":"a"}]}`, true},
		{"expression name", `{"fields":[{"field":"=a"}]}`, true},
		{"flag not a boolean", `{"fields":[{"field":"a"}],"mergeLists":1}`, true},
		{"destination in individual mode", `{"fields":[{"field":"a"}],"destinationField":"d"}`, true},
		{"fields in all item data mode", `{"mode":"ALL_ITEM_DATA","fields":[{"field":"a"}]}`, true},
		{"flag in all item data mode", `{"mode":"ALL_ITEM_DATA","keepMissing":true}`, true},
		{"selected without names", `{"mode":"ALL_ITEM_DATA","include":"SELECTED_FIELDS"}`, true},
		{"all fields with names", `{"mode":"ALL_ITEM_DATA","includeFields":["a"]}`, true},
		{"bad include", `{"mode":"ALL_ITEM_DATA","include":"some"}`, true},
		{"bad mode", `{"mode":"ALL"}`, true},
		{"empty destination", `{"mode":"ALL_ITEM_DATA","destinationField":""}`, true},
		{"unknown key", `{"fields":[{"field":"a"}],"unique":true}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAggregate(mustParse(t, tt.params))
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCoreNodesAggregateGathersIntoOneItem(t *testing.T) {
	people := []string{`{"name":"Ana","tags":["a","b"],"age":30}`, `{"name":"Luis","tags":["c"]}`, `{"name":"Eva","tags":[],"age":null}`}
	tests := []struct {
		name   string
		params string
		items  []string
		want   string
	}{
		{"one field", `{"fields":[{"field":"name"}]}`, people, `{"name":["Ana","Luis","Eva"]}`},
		{"two fields, one renamed, in order", `{"fields":[{"field":"name","outputField":"who"},{"field":"age"}]}`, people,
			`{"who":["Ana","Luis","Eva"],"age":[30,null]}`},
		{"a missing field adds nothing", `{"fields":[{"field":"age"}]}`, people[:2], `{"age":[30]}`},
		{"keepMissing adds null", `{"fields":[{"field":"age"}],"keepMissing":true}`, people[:2], `{"age":[30,null]}`},
		{"lists stay lists", `{"fields":[{"field":"tags"}]}`, people, `{"tags":[["a","b"],["c"],[]]}`},
		{"mergeLists flattens one level", `{"fields":[{"field":"tags"}],"mergeLists":true}`, people, `{"tags":["a","b","c"]}`},
		{"nested path", `{"fields":[{"field":"u.id"}]}`, []string{`{"u":{"id":1}}`, `{"u":{"id":2}}`}, `{"u.id":[1,2]}`},
		{"whole items", `{"mode":"ALL_ITEM_DATA"}`, []string{`{"a":1}`, `{"a":2}`}, `{"data":[{"a":1},{"a":2}]}`},
		{"whole items to another field", `{"mode":"ALL_ITEM_DATA","destinationField":"rows"}`, []string{`{"a":1}`}, `{"rows":[{"a":1}]}`},
		{"selected fields of each item", `{"mode":"ALL_ITEM_DATA","include":"SELECTED_FIELDS","includeFields":["a","c.d"]}`,
			[]string{`{"a":1,"b":2,"c":{"d":3}}`, `{"b":2}`}, `{"data":[{"a":1,"c.d":3},{}]}`},
		{"all fields but some", `{"mode":"ALL_ITEM_DATA","include":"ALL_FIELDS_EXCEPT","includeFields":["b"]}`,
			[]string{`{"a":1,"b":2}`, `{"a":3,"b":4}`}, `{"data":[{"a":1},{"a":3}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := run(t, nil, TypeAggregate, tt.params, execution.NodeEnv{}, tt.items...)
			if err != nil {
				t.Fatal(err)
			}
			wantTexts(t, got, tt.want)
		})
	}
}

func TestCoreNodesAggregateCarriesTheLineageOfEveryInput(t *testing.T) {
	got, err := run(t, nil, TypeAggregate, `{"fields":[{"field":"a"}]}`, execution.NodeEnv{}, `{"a":1}`, `{"a":2}`, `{"a":3}`)
	if err != nil {
		t.Fatal(err)
	}
	var idx []int
	for _, ref := range got[0].Lineage {
		idx = append(idx, ref.Index)
	}
	if !slices.Equal(idx, []int{0, 1, 2}) {
		t.Errorf("lineage indexes = %v, want [0 1 2]", idx)
	}
}

func TestCoreNodesAggregateWithNoItemsEmitsNone(t *testing.T) {
	got, err := run(t, nil, TypeAggregate, `{"fields":[{"field":"a"}]}`, execution.NodeEnv{})
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v; want no items", got, err)
	}
}

func TestCoreNodesAggregateFailures(t *testing.T) {
	_, err := run(t, nil, TypeAggregate, `{"mode":"ALL_ITEM_DATA"}`, execution.NodeEnv{}, `{"a":1}`, `[1]`)
	wantFailure(t, err, "AGGREGATE_FAILED")
	_, err = run(t, nil, TypeAggregate, `{}`, execution.NodeEnv{}, `{}`)
	wantParamError(t, err)
}

func TestCoreNodesSplitOutParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		wantErr bool
	}{
		{"field only", `{"field":"items"}`, false},
		{"destination", `{"field":"items","destinationField":"item"}`, false},
		{"all other fields", `{"field":"items","include":"ALL_OTHER_FIELDS"}`, false},
		{"selected other fields", `{"field":"items","include":"SELECTED_OTHER_FIELDS","includeFields":["id","a.b"]}`, false},
		{"no field", `{}`, true},
		{"empty field", `{"field":""}`, true},
		{"field not a string", `{"field":1}`, true},
		{"expression field", `{"field":"=x"}`, true},
		{"empty destination", `{"field":"a","destinationField":" "}`, true},
		{"bad include", `{"field":"a","include":"ALL"}`, true},
		{"names without selected", `{"field":"a","includeFields":["b"]}`, true},
		{"selected without names", `{"field":"a","include":"SELECTED_OTHER_FIELDS"}`, true},
		{"unknown key", `{"field":"a","binary":true}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSplitOut(mustParse(t, tt.params))
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCoreNodesSplitOutMakesOneItemPerElement(t *testing.T) {
	tests := []struct {
		name   string
		params string
		items  []string
		want   []string
	}{
		{"elements under the field name", `{"field":"tags"}`,
			[]string{`{"id":1,"tags":["a","b"]}`}, []string{`{"tags":"a"}`, `{"tags":"b"}`}},
		{"elements under a destination", `{"field":"tags","destinationField":"tag"}`,
			[]string{`{"tags":["a","b"]}`}, []string{`{"tag":"a"}`, `{"tag":"b"}`}},
		{"objects stay objects under the destination", `{"field":"people","destinationField":"p"}`,
			[]string{`{"people":[{"n":1},{"n":2}]}`}, []string{`{"p":{"n":1}}`, `{"p":{"n":2}}`}},
		{"every other field", `{"field":"tags","destinationField":"tag","include":"ALL_OTHER_FIELDS"}`,
			[]string{`{"id":1,"tags":["a","b"],"x":true}`}, []string{`{"id":1,"x":true,"tag":"a"}`, `{"id":1,"x":true,"tag":"b"}`}},
		{"every other field with the same name replaces the list", `{"field":"tags","include":"ALL_OTHER_FIELDS"}`,
			[]string{`{"tags":["a","b"],"id":1}`}, []string{`{"id":1,"tags":"a"}`, `{"id":1,"tags":"b"}`}},
		{"selected other fields", `{"field":"tags","destinationField":"tag","include":"SELECTED_OTHER_FIELDS","includeFields":["id","nope"]}`,
			[]string{`{"id":1,"drop":2,"tags":["a"]}`}, []string{`{"id":1,"tag":"a"}`}},
		{"several items in order", `{"field":"v"}`,
			[]string{`{"v":[1,2]}`, `{"v":[3]}`}, []string{`{"v":1}`, `{"v":2}`, `{"v":3}`}},
		{"an empty list gives no item", `{"field":"v"}`,
			[]string{`{"v":[]}`, `{"v":[1]}`}, []string{`{"v":1}`}},
		{"a nested path", `{"field":"data.rows","destinationField":"row"}`,
			[]string{`{"data":{"rows":[1,2]}}`}, []string{`{"row":1}`, `{"row":2}`}},
		{"a nested path keeps the parent when others are kept", `{"field":"data.rows","destinationField":"row","include":"ALL_OTHER_FIELDS"}`,
			[]string{`{"id":9,"data":{"rows":[1],"k":2}}`}, []string{`{"id":9,"data":{"k":2},"row":1}`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := run(t, nil, TypeSplitOut, tt.params, execution.NodeEnv{}, tt.items...)
			if err != nil {
				t.Fatal(err)
			}
			wantTexts(t, got, tt.want...)
		})
	}
	got, err := run(t, nil, TypeSplitOut, `{"field":"v"}`, execution.NodeEnv{}, `{"v":[1]}`, `{"v":[2,3]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Lineage[0].Index != 0 || got[1].Lineage[0].Index != 1 || got[2].Lineage[0].Index != 1 {
		t.Errorf("lineage of the new items = %+v", got)
	}
}

func TestCoreNodesSplitOutFailures(t *testing.T) {
	for name, item := range map[string]string{
		"missing field":  `{"a":1}`,
		"not a list":     `{"v":"x"}`,
		"null":           `{"v":null}`,
		"an object":      `{"v":{"a":1}}`,
		"not an object ": `[1]`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := run(t, nil, TypeSplitOut, `{"field":"v"}`, execution.NodeEnv{}, item)
			wantFailure(t, err, "SPLIT_OUT_FAILED")
		})
	}
	_, err := run(t, nil, TypeSplitOut, `{}`, execution.NodeEnv{}, `{}`)
	wantParamError(t, err)
}
