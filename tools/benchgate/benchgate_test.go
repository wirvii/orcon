// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func sig(name, unit string, delta float64) Row {
	return Row{Name: name, Unit: unit, Delta: delta, HasDelta: true, P: 0.003}
}

func TestGate(t *testing.T) {
	const justified = "Intro.\n\n### Performance regression justification\n\nThe new path validates every item; the extra work is required by ADR-0018.\n\n### Notes\nnone\n"
	tests := []struct {
		name     string
		rows     []Row
		accepted bool
		body     string
		wantPass bool
		wantRegs int
	}{
		{name: "9 percent significant passes", rows: []Row{sig("A", "sec/op", 9)}, wantPass: true},
		{name: "exactly 10 percent passes", rows: []Row{sig("A", "sec/op", 10)}, wantPass: true},
		{name: "11 percent significant fails", rows: []Row{sig("A", "sec/op", 11)}, wantRegs: 1},
		{
			name: "11 percent not significant passes",
			rows: []Row{{Name: "A", Unit: "sec/op", Delta: 11, HasDelta: true, P: 0.2}}, wantPass: true,
		},
		{name: "benchstat tilde passes", rows: []Row{{Name: "A", Unit: "sec/op", P: 1}}, wantPass: true},
		{name: "improvement passes", rows: []Row{sig("A", "sec/op", -40)}, wantPass: true},
		{name: "B/op regression fails", rows: []Row{sig("A", "B/op", 25)}, wantRegs: 1},
		{name: "allocs/op regression fails", rows: []Row{sig("A", "allocs/op", 50)}, wantRegs: 1},
		{
			name: "one bad metric among good ones fails",
			rows: []Row{sig("A", "sec/op", 2), sig("B", "sec/op", 30), sig("C", "allocs/op", -5)}, wantRegs: 1,
		},
		{name: "label and justification accept", rows: []Row{sig("A", "sec/op", 30)}, accepted: true, body: justified, wantPass: true, wantRegs: 1},
		{name: "label without a section fails", rows: []Row{sig("A", "sec/op", 30)}, accepted: true, body: "No section here.\n", wantRegs: 1},
		{
			name: "label with an empty section fails", rows: []Row{sig("A", "sec/op", 30)}, accepted: true,
			body: "### Performance regression justification\n\n### Next\ntext\n", wantRegs: 1,
		},
		{
			name: "label with only a template hint fails", rows: []Row{sig("A", "sec/op", 30)}, accepted: true,
			body: "### Performance regression justification\n<!-- explain why -->\n", wantRegs: 1,
		},
		{name: "justification without the label fails", rows: []Row{sig("A", "sec/op", 30)}, body: justified, wantRegs: 1},
		{name: "label with nothing to accept passes", rows: []Row{sig("A", "sec/op", 1)}, accepted: true, wantPass: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Gate(tt.rows, Options{Accepted: tt.accepted, Body: tt.body})
			if res.Pass != tt.wantPass {
				t.Fatalf("Pass = %v, want %v (%s)", res.Pass, tt.wantPass, res.Message)
			}
			if len(res.Regressions) != tt.wantRegs {
				t.Fatalf("regressions = %v, want %d", res.Regressions, tt.wantRegs)
			}
		})
	}
}

const sampleCSV = `goos: darwin
goarch: arm64
pkg: example.com/x
,base.txt,,head.txt,,,
,sec/op,CI,sec/op,CI,vs base,P
A-8,1.0000000000000002e-06,∞,1.3e-06,∞,+30.00%,p=0.008 n=5
B-8,2.0000000000000003e-06,∞,2.0000000000000003e-06,∞,~,p=1.000 n=5
geomean,1.414213562373097e-06,,1.6124515496597109e-06,,+14.02%,

,base.txt,,head.txt,,,
,B/op,CI,B/op,CI,vs base,P
A-8,64,∞,64,∞,~,p=1.000 n=5
geomean,64,,64,,+0.00%,

,base.txt,,head.txt,,,
,allocs/op,CI,allocs/op,CI,vs base,P
A-8,2,∞,3,∞,+50.00%,p=0.008 n=5
`

func TestParseCSV(t *testing.T) {
	rows, err := ParseCSV(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("rows = %v, want 4 (geomean rows are skipped)", rows)
	}
	want := []Row{
		{Name: "A-8", Unit: "sec/op", Delta: 30, HasDelta: true, P: 0.008},
		{Name: "B-8", Unit: "sec/op", P: 1},
		{Name: "A-8", Unit: "B/op", P: 1},
		{Name: "A-8", Unit: "allocs/op", Delta: 50, HasDelta: true, P: 0.008},
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
	res := Gate(rows, Options{})
	if res.Pass || len(res.Regressions) != 2 {
		t.Fatalf("sample must fail with 2 regressions, got pass=%v regressions=%v", res.Pass, res.Regressions)
	}
}

func TestParseCSVRejectsGarbageDelta(t *testing.T) {
	bad := ",a,,b,,,\n,sec/op,CI,sec/op,CI,vs base,P\nA-8,1,∞,2,∞,huge,p=0.001 n=5\n"
	if _, err := ParseCSV(strings.NewReader(bad)); err == nil {
		t.Fatal("an unrecognized delta must be an error, not a silent pass")
	}
}

func TestJustification(t *testing.T) {
	tests := map[string]string{
		"### Performance regression justification\ntext\n":               "text",
		"### performance regression justification\r\nwindows text\r\n":   "windows text",
		"x\n### Performance regression justification\na\nb\n## Next\nc":  "a\nb",
		"### Performance regression justification\n":                     "",
		"### Performance regression justification\n<!-- hint -->\n   \n": "",
		"nothing relevant": "",
		"#### Performance regression justification\ntoo many hashes is another": "",
	}
	for body, want := range tests {
		if got := Justification(body); got != want {
			t.Errorf("Justification(%q) = %q, want %q", body, got, want)
		}
	}
}

func TestRun(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(nil, strings.NewReader(sampleCSV), &out, &errOut); code != 1 {
		t.Fatalf("regressed input = %d, want 1\n%s%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "regression: A-8 sec/op: +30.00%") {
		t.Fatalf("stdout does not list the regression:\n%s", out.String())
	}
	out.Reset()
	errOut.Reset()
	if code := run(nil, strings.NewReader(",a,,b,,,\n"), &out, &errOut); code != 2 {
		t.Fatalf("input with no rows = %d, want 2", code)
	}
	if code := run([]string{"-csv", "does-not-exist.csv"}, strings.NewReader(""), &out, &errOut); code != 2 {
		t.Fatalf("missing csv file = %d, want 2", code)
	}
}
