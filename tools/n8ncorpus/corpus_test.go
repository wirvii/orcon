// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// library is a fake template library.
func library(t *testing.T, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	workflow := func(id int) string {
		return fmt.Sprintf(`{"workflow":{"id":%d,"workflow":{"nodes":[
			{"name":"Set","type":"n8n-nodes-base.set","parameters":{"values":{"string":[{"name":"a","value":"={{ $json.a }}"},{"name":"b","value":"plain"}]},"other":"=text {{ $json.b }}"}},
			{"name":"Empty","type":"x","parameters":{}},
			{"name":"Broken","type":"x","parameters":"not an object"}]}}}`, id)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/templates/search", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Query().Get("page") {
		case "1":
			fmt.Fprint(w, `{"workflows":[{"id":11,"totalViews":500},{"id":12,"totalViews":300}]}`)
		case "2":
			fmt.Fprint(w, `{"workflows":[{"id":13,"totalViews":100}]}`)
		default:
			fmt.Fprint(w, `{"workflows":[]}`)
		}
	})
	mux.HandleFunc("/api/templates/workflows/", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		var id int
		if _, err := fmt.Sscanf(strings.TrimPrefix(r.URL.Path, "/api/templates/workflows/"), "%d", &id); err != nil {
			http.NotFound(w, r)
			return
		}
		if id == 13 {
			fmt.Fprint(w, `{`) // a template that does not parse is skipped
			return
		}
		fmt.Fprint(w, workflow(id))
	})
	instance(mux, "9.9.9", nodeTypesFixture)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestRunDownloadsExtractsAndResumes(t *testing.T) {
	var hits atomic.Int64
	srv := library(t, &hits)
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	args := []string{"-n8n", "9.9.9", "-dir", dir, "-limit", "3", "-base", srv.URL, "-n8n-url", srv.URL, "-delay", "1ms", "-workers", "2"}
	if code := run(context.Background(), args, &out, &errOut); code != 0 {
		t.Fatalf("run = %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "2 templates, 4 expressions") {
		t.Errorf("output = %q", out.String())
	}

	mb, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	if m.N8nVersion != "9.9.9" || m.Templates != 2 || m.Expressions != 4 || len(m.CorpusSHA256) != 64 || !strings.HasSuffix(m.Source, "/api/templates") {
		t.Errorf("manifest = %+v", m)
	}

	lines, err := os.ReadFile(filepath.Join(dir, "expressions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var got []Expression
	for _, l := range strings.Split(strings.TrimSpace(string(lines)), "\n") {
		var e Expression
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			t.Fatal(err)
		}
		got = append(got, e)
	}
	if len(got) != 4 || got[0].Template != 11 || got[0].Views != 500 || got[0].Node != "Set" || got[0].NodeType != "n8n-nodes-base.set" ||
		got[0].Path != "parameters.other" || got[0].Expression != "=text {{ $json.b }}" || got[1].Path != "parameters.values.string[0].value" {
		t.Errorf("expressions = %+v", got)
	}

	// A second run reuses the downloaded templates and yields the same digest.
	before := hits.Load()
	var out2 bytes.Buffer
	if code := run(context.Background(), args, &out2, &errOut); code != 0 {
		t.Fatalf("second run = %d: %s", code, errOut.String())
	}
	if hits.Load()-before > 3 { // only the search pages
		t.Errorf("the second run downloaded %d templates again", hits.Load()-before)
	}
	if out2.String() != out.String() {
		t.Errorf("the corpus digest changed between runs: %q vs %q", out.String(), out2.String())
	}
}

func TestRunRejectsBadUse(t *testing.T) {
	var out, errOut bytes.Buffer
	for _, args := range [][]string{
		{},                             // no -n8n
		{"-n8n", "1", "-limit", "0"},   // bad limit
		{"-n8n", "1", "-workers", "0"}, // bad workers
		{"-nope"},                      // unknown flag
	} {
		errOut.Reset()
		if code := run(context.Background(), args, &out, &errOut); code != 2 {
			t.Errorf("run(%v) = %d, want 2 (%s)", args, code, errOut.String())
		}
	}
	if code := run(context.Background(), []string{"-h"}, &out, &errOut); code != 0 {
		t.Errorf("-h = %d", code)
	}
}

func TestRunReportsLibraryFailures(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "gone", http.StatusNotFound) }))
	defer down.Close()
	var out, errOut bytes.Buffer
	args := []string{"-n8n", "1", "-dir", t.TempDir(), "-base", down.URL, "-delay", "1ms"}
	if code := run(context.Background(), args, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "404") {
		t.Errorf("run = %d: %s", code, errOut.String())
	}

	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"workflows":[]}`) }))
	defer empty.Close()
	errOut.Reset()
	args = []string{"-n8n", "1", "-dir", t.TempDir(), "-base", empty.URL, "-delay", "1ms"}
	if code := run(context.Background(), args, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "no templates") {
		t.Errorf("empty library: %d: %s", code, errOut.String())
	}

	// A canceled context stops the download.
	var hits atomic.Int64
	srv := library(t, &hits)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	errOut.Reset()
	args = []string{"-n8n", "1", "-dir", t.TempDir(), "-base", srv.URL, "-delay", "1ms"}
	if code := run(ctx, args, &out, &errOut); code != 1 {
		t.Errorf("canceled run = %d", code)
	}
}

func TestCollectFindsExpressionsAtEveryDepth(t *testing.T) {
	var found []string
	collect(json.RawMessage(`{"a":"=x","b":["=y",{"c":"=z"}],"d":"no","e":5,"f":{"g":{"h":"=deep"}}}`), "p", func(path, e string) {
		found = append(found, path+" "+e)
	})
	want := []string{"p.a =x", "p.b[0] =y", "p.b[1].c =z", "p.f.g.h =deep"}
	if strings.Join(found, "|") != strings.Join(want, "|") {
		t.Errorf("found %v, want %v", found, want)
	}
	collect(json.RawMessage(`{`), "p", func(string, string) { t.Error("visited an unreadable value") })
}
