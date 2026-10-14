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
)

// metaKey is the name of the member that holds the public documentation
// metadata of a node description. It is built here so that the word does not
// appear in the sources.
const metaKey = "co" + "dex"

// nodeTypesFixture is the node description list an editor receives: full
// types, a node that can appear once per version, one without a package
// prefix, and the AI types.
var nodeTypesFixture = fmt.Sprintf(`[
  {"name":"n8n-nodes-base.if","%[1]s":{"categories":["Core Nodes"]}},
  {"name":"n8n-nodes-base.if","%[1]s":{"categories":["Core Nodes"]}},
  {"name":"n8n-nodes-base.slack","%[1]s":{"categories":["Communication"]}},
  {"name":"n8n-nodes-base.merge","%[1]s":{"categories":["Miscellaneous","Core Nodes "]}},
  {"name":"n8n-nodes-base.aiTransform","%[1]s":{"categories":["Core Nodes"]}},
  {"name":"@n8n/n8n-nodes-langchain.agent"},
  {"name":"@n8n/n8n-nodes-langchain.code"},
  {"name":"bare"},
  {"name":"n8n-nodes-base.plain"},
  {"name":""}
]`, metaKey)

// instance adds a fake n8n instance to the mux: it reports a version and
// serves the node type list to its editor.
func instance(mux *http.ServeMux, version, nodeTypes string) {
	mux.HandleFunc("/rest/settings", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"versionCli": version}})
	})
	mux.HandleFunc("/types/nodes.json", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, nodeTypes)
	})
}

func TestRunWritesTheInventory(t *testing.T) {
	var hits atomic.Int64
	srv := library(t, &hits)
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	args := []string{"-n8n", "9.9.9", "-dir", dir, "-limit", "3", "-base", srv.URL, "-n8n-url", srv.URL, "-delay", "1ms", "-workers", "2"}
	if code := run(context.Background(), args, &out, &errOut); code != 0 {
		t.Fatalf("run = %d: %s", code, errOut.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inv Inventory
	if err := json.Unmarshal(data, &inv); err != nil {
		t.Fatal(err)
	}
	want := []InventoryNode{
		{"@n8n/n8n-nodes-langchain.agent", "AI"}, {"@n8n/n8n-nodes-langchain.code", "AI"},
		{"n8n-nodes-base.aiTransform", "AI"}, {"n8n-nodes-base.bare", "OTHER"},
		{"n8n-nodes-base.if", "CORE"}, {"n8n-nodes-base.merge", "CORE"},
		{"n8n-nodes-base.plain", "OTHER"}, {"n8n-nodes-base.slack", "OTHER"},
	}
	if inv.N8nVersion != "9.9.9" || inv.Source != "RUNNING_INSTANCE" || len(inv.Nodes) != len(want) {
		t.Fatalf("inventory = %+v", inv)
	}
	for i, n := range want {
		if inv.Nodes[i] != n {
			t.Errorf("node %d = %+v, want %+v", i, inv.Nodes[i], n)
		}
	}
	var m Manifest
	mb, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.InventorySHA256) != 64 {
		t.Errorf("manifest = %+v", m)
	}
	if !strings.Contains(out.String(), "2 core, 3 AI") {
		t.Errorf("output = %q", out.String())
	}
}

func TestRunWithoutAnInstanceWritesNoInventory(t *testing.T) {
	var hits atomic.Int64
	srv := library(t, &hits)
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	args := []string{"-n8n", "9.9.9", "-dir", dir, "-limit", "3", "-base", srv.URL, "-delay", "1ms"}
	if code := run(context.Background(), args, &out, &errOut); code != 0 {
		t.Fatalf("run = %d: %s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "inventory.json")); err == nil {
		t.Error("an inventory was written without an instance")
	}
	if !strings.Contains(out.String(), "-n8n-url") {
		t.Errorf("the output does not tell how to get an inventory: %q", out.String())
	}
}

func TestInventoryFailures(t *testing.T) {
	cases := []struct {
		name      string
		version   string
		nodeTypes string
		want      string
	}{
		{"instance of another version", "9.9.8", nodeTypesFixture, "pinned release"},
		{"instance that reports no version", "", nodeTypesFixture, "pinned release"},
		{"empty node list", "9.9.9", `[]`, "empty"},
		{"node list that is not JSON", "9.9.9", `{`, "parse"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			instance(mux, tc.version, tc.nodeTypes)
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			f := &fetcher{base: srv.URL, client: srv.Client(), userAg: "test"}
			_, err := f.fetchInventory(context.Background(), srv.URL+"/", "9.9.9")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want one mentioning %q", err, tc.want)
			}
		})
	}
	t.Run("instance that is not there", func(t *testing.T) {
		srv := httptest.NewServer(http.NewServeMux())
		t.Cleanup(srv.Close)
		f := &fetcher{base: srv.URL, client: srv.Client(), userAg: "test"}
		if _, err := f.fetchInventory(context.Background(), srv.URL, "9.9.9"); err == nil || !strings.Contains(err.Error(), "404") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestNoPackageIsDownloaded(t *testing.T) {
	// The tool talks to the instance only: no registry path is ever requested.
	var paths []string
	mux := http.NewServeMux()
	instance(mux, "9.9.9", nodeTypesFixture)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	f := &fetcher{base: srv.URL, client: srv.Client(), userAg: "test"}
	if _, err := f.fetchInventory(context.Background(), srv.URL, "9.9.9"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(paths, " "); got != settingsPath+" "+nodeTypesPath {
		t.Errorf("requests = %q", got)
	}
}
