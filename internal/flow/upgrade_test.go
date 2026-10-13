// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// renameStep is a test upgrade step: it renames the top-level "title" member
// to "name", the way a real step would adapt an older shape.
func renameStep(v jsonvalue.Value) (jsonvalue.Value, error) {
	title, ok := v.Get("title")
	if !ok {
		return jsonvalue.Value{}, errors.New("document has no title")
	}
	return v.Without("title").With("name", title), nil
}

// markStep records that it ran, by adding a description.
func markStep(v jsonvalue.Value) (jsonvalue.Value, error) {
	return v.With("description", jsonvalue.String("upgraded")), nil
}

func TestUpgradeRunsTheChainOnRead(t *testing.T) {
	t.Parallel()
	u, err := flow.NewUpgrader(3, map[int]flow.UpgradeStep{1: renameStep, 2: markStep})
	if err != nil {
		t.Fatal(err)
	}
	old, err := jsonvalue.Parse([]byte(`{"format":"orcon/flow@1","title":"Old name","keep":1}`))
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := jsonvalue.Marshal(old)

	got, err := u.Upgrade(old)
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if f, _ := got.Get("format"); !jsonvalue.Equal(f, jsonvalue.String("orcon/flow@3")) {
		t.Errorf("format after upgrade = %v, want orcon/flow@3", f)
	}
	if n, _ := got.Get("name"); !jsonvalue.Equal(n, jsonvalue.String("Old name")) {
		t.Error("step 1 did not run")
	}
	if d, _ := got.Get("description"); !jsonvalue.Equal(d, jsonvalue.String("upgraded")) {
		t.Error("step 2 did not run")
	}
	if _, has := got.Get("title"); has {
		t.Error("the old member survived the upgrade")
	}

	// Reading never rewrites what is stored: the input value is unchanged.
	after, _ := jsonvalue.Marshal(old)
	if !bytes.Equal(stored, after) {
		t.Errorf("Upgrade modified its input: %s became %s", stored, after)
	}
}

func TestUpgradeStartsAtTheDocumentVersion(t *testing.T) {
	t.Parallel()
	ran := []int{}
	step := func(n int) flow.UpgradeStep {
		return func(v jsonvalue.Value) (jsonvalue.Value, error) {
			ran = append(ran, n)
			return v, nil
		}
	}
	u, err := flow.NewUpgrader(4, map[int]flow.UpgradeStep{1: step(1), 2: step(2), 3: step(3)})
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := jsonvalue.Parse([]byte(`{"format":"orcon/flow@3"}`))
	if _, err := u.Upgrade(doc); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 1 || ran[0] != 3 {
		t.Errorf("steps run = %v, want only step 3", ran)
	}
	current, _ := jsonvalue.Parse([]byte(`{"format":"orcon/flow@4"}`))
	ran = ran[:0]
	got, err := u.Upgrade(current)
	if err != nil || len(ran) != 0 || !jsonvalue.Equal(got, current) {
		t.Errorf("a current document was changed: steps=%v err=%v", ran, err)
	}
	if u.Current() != 4 {
		t.Errorf("Current() = %d, want 4", u.Current())
	}
}

func TestUpgradeStepFailureIsReported(t *testing.T) {
	t.Parallel()
	u, err := flow.NewUpgrader(2, map[int]flow.UpgradeStep{1: renameStep})
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := jsonvalue.Parse([]byte(`{"format":"orcon/flow@1"}`))
	_, err = u.Upgrade(doc)
	if err == nil || !strings.Contains(err.Error(), "orcon/flow@1 to orcon/flow@2") {
		t.Errorf("Upgrade error = %v, want it to name the failing step", err)
	}
}

func TestUpgradeBuiltInChainIsIdentityAtOne(t *testing.T) {
	t.Parallel()
	for _, name := range conformanceNames(t) {
		raw := conformanceValue(t, name)
		got, err := flow.Upgrade(raw)
		if err != nil || !jsonvalue.Equal(got, raw) {
			t.Errorf("Upgrade(%s) = changed=%v err=%v, want the document unchanged", name, !jsonvalue.Equal(got, raw), err)
		}
	}
}

func TestNewUpgraderRejectsBadChains(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		current int
		steps   map[int]flow.UpgradeStep
	}{
		{"current below one", 0, nil},
		{"missing step", 3, map[int]flow.UpgradeStep{1: markStep}},
		{"step beyond current", 2, map[int]flow.UpgradeStep{1: markStep, 2: markStep}},
		{"step for version zero", 1, map[int]flow.UpgradeStep{0: markStep}},
	}
	for _, tt := range tests {
		if _, err := flow.NewUpgrader(tt.current, tt.steps); err == nil {
			t.Errorf("%s: NewUpgrader succeeded, want error", tt.name)
		}
	}
}

func TestUnknownFormatIsRejectedWithTheSupportedRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, doc string
	}{
		{"newer major", `{"format":"orcon/flow@2"}`},
		{"far newer major", `{"format":"orcon/flow@99"}`},
		{"version zero", `{"format":"orcon/flow@0"}`},
		{"leading zero", `{"format":"orcon/flow@01"}`},
		{"signed", `{"format":"orcon/flow@+1"}`},
		{"not a number", `{"format":"orcon/flow@x"}`},
		{"other family", `{"format":"bpmn/flow@1"}`},
		{"no version", `{"format":"orcon/flow"}`},
		{"empty", `{"format":""}`},
		{"not a string", `{"format":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v, err := jsonvalue.Parse([]byte(tt.doc))
			if err != nil {
				t.Fatal(err)
			}
			_, err = flow.Upgrade(v)
			var unknown *flow.UnknownFormatError
			if !errors.As(err, &unknown) {
				t.Fatalf("Upgrade error = %v, want *UnknownFormatError", err)
			}
			msg := err.Error()
			if !strings.Contains(msg, "orcon/flow@1 through orcon/flow@1") {
				t.Errorf("error %q does not name the supported range", msg)
			}
			if f, _ := mustMember(t, v, "format").Str(); f != "" && !strings.Contains(msg, f) {
				t.Errorf("error %q does not name the format %q", msg, f)
			}
			if _, readErr := flow.Read(v); !errors.As(readErr, &unknown) {
				t.Errorf("Read error = %v, want *UnknownFormatError", readErr)
			}
		})
	}
}

func TestUnknownFormatMissingTag(t *testing.T) {
	t.Parallel()
	v, _ := jsonvalue.Parse([]byte(`{"id":"x"}`))
	_, err := flow.Upgrade(v)
	var ve *flow.ValidationError
	if !errors.As(err, &ve) || !ve.Report.Has(flow.CodeFormat, flow.SeverityError) {
		t.Errorf("Upgrade error = %v, want a FORMAT validation error", err)
	}
}

func mustMember(t *testing.T, v jsonvalue.Value, key string) jsonvalue.Value {
	t.Helper()
	m, ok := v.Get(key)
	if !ok {
		t.Fatalf("no member %q", key)
	}
	return m
}

func TestUpgradeStepMustReturnAnObject(t *testing.T) {
	t.Parallel()
	for name, result := range map[string]jsonvalue.Value{
		"string": jsonvalue.String("x"), "array": jsonvalue.Array(), "null": jsonvalue.Null(), "absent": {},
	} {
		step := func(jsonvalue.Value) (jsonvalue.Value, error) { return result, nil }
		u, err := flow.NewUpgrader(2, map[int]flow.UpgradeStep{1: step})
		if err != nil {
			t.Fatal(err)
		}
		doc, _ := jsonvalue.Parse([]byte(`{"format":"orcon/flow@1"}`))
		_, err = u.Upgrade(doc)
		if err == nil || !strings.Contains(err.Error(), "not an object") {
			t.Errorf("%s: Upgrade error = %v, want it to say the step did not return an object", name, err)
		}
	}
}
