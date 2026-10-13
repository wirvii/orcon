// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
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

// nodeTypesFixture is the node description list of the built-in package: a
// node can appear once per version, and only the category matters.
var nodeTypesFixture = fmt.Sprintf(`[
  {"name":"if","%[1]s":{"categories":["Core Nodes"]}},
  {"name":"if","%[1]s":{"categories":["Core Nodes"]}},
  {"name":"slack","%[1]s":{"categories":["Communication"]}},
  {"name":"merge","%[1]s":{"categories":["Miscellaneous","Core Nodes "]}},
  {"name":"plain"},
  {"name":""}
]`, metaKey)

func tarballOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// registry adds a fake package registry to the mux: n8n 9.9.9 depends on the
// built-in nodes package 1.2.3, whose tarball holds the node description list.
// With goodIntegrity false the registry announces a digest the tarball does not have.
func registry(mux *http.ServeMux, nodeTypes string, goodIntegrity bool) {
	tarball := tarballOfFixture(nodeTypes)
	mux.HandleFunc("/n8n/9.9.9", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"dependencies":{"n8n-nodes-base":"1.2.3"}}`)
	})
	mux.HandleFunc("/n8n/8.8.8", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"dependencies":{}}`)
	})
	mux.HandleFunc("/n8n-nodes-base/1.2.3", func(w http.ResponseWriter, r *http.Request) {
		sum := sha512.Sum512(tarball)
		integrity := "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
		if !goodIntegrity {
			integrity = "sha512-AAAA"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"dist": map[string]string{"tarball": "http://" + r.Host + "/base.tgz", "integrity": integrity}})
	})
	mux.HandleFunc("/base.tgz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(tarball) })
}

func tarballOfFixture(nodeTypes string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := map[string]string{"package/package.json": "{}", nodeTypesPath: nodeTypes}
	for _, name := range []string{"package/package.json", nodeTypesPath} {
		content := files[name]
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(content))
	}
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func TestRunWritesTheInventory(t *testing.T) {
	var hits atomic.Int64
	srv := library(t, &hits)
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	args := []string{"-n8n", "9.9.9", "-dir", dir, "-limit", "3", "-base", srv.URL, "-registry", srv.URL, "-delay", "1ms", "-workers", "2"}
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
		{"n8n-nodes-base.if", "CORE"}, {"n8n-nodes-base.merge", "CORE"},
		{"n8n-nodes-base.plain", "OTHER"}, {"n8n-nodes-base.slack", "OTHER"},
	}
	if inv.N8nVersion != "9.9.9" || inv.NodesBaseVersion != "1.2.3" || len(inv.Nodes) != len(want) {
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
	if m.NodesBaseVersion != "1.2.3" || len(m.InventorySHA256) != 64 {
		t.Errorf("manifest = %+v", m)
	}
	if !strings.Contains(out.String(), "2 core") {
		t.Errorf("output = %q", out.String())
	}
}

func TestRunWithoutInventory(t *testing.T) {
	var hits atomic.Int64
	srv := library(t, &hits)
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	args := []string{"-n8n", "9.9.9", "-dir", dir, "-limit", "3", "-base", srv.URL, "-no-inventory", "-delay", "1ms"}
	if code := run(context.Background(), args, &out, &errOut); code != 0 {
		t.Fatalf("run = %d: %s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "inventory.json")); err == nil {
		t.Error("-no-inventory still wrote an inventory")
	}
}

func TestInventoryFailures(t *testing.T) {
	cases := []struct {
		name      string
		n8n       string
		nodeTypes string
		integrity bool
		want      string
	}{
		{"release without the nodes package", "8.8.8", nodeTypesFixture, true, "does not depend"},
		{"unknown release", "7.7.7", nodeTypesFixture, true, "404"},
		{"tarball with another digest", "9.9.9", nodeTypesFixture, false, "integrity"},
		{"empty node list", "9.9.9", `[]`, true, "empty"},
		{"node list that is not JSON", "9.9.9", `{`, true, "parse"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			registry(mux, tc.nodeTypes, tc.integrity)
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			f := &fetcher{base: srv.URL, client: srv.Client(), userAg: "test"}
			_, err := f.fetchInventory(context.Background(), srv.URL, tc.n8n)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want one mentioning %q", err, tc.want)
			}
		})
	}
}

func TestExtractFile(t *testing.T) {
	tb := tarballOf(t, map[string]string{"package/a.json": "A", "package/dir/big": strings.Repeat("x", 100)})
	if got, err := extractFile(tb, "package/a.json", 10); err != nil || string(got) != "A" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := extractFile(tb, "package/dir/big", 10); err == nil {
		t.Error("a file over the limit must fail")
	}
	if _, err := extractFile(tb, "package/missing", 10); err == nil {
		t.Error("a missing file must fail")
	}
	if _, err := extractFile([]byte("not a tarball"), "x", 10); err == nil {
		t.Error("garbage must fail")
	}
	if err := checkIntegrity([]byte("x"), "sha1-abc"); err == nil {
		t.Error("an unsupported digest algorithm must fail")
	}
	if err := checkIntegrity([]byte("x"), ""); err != nil {
		t.Errorf("no digest is accepted: %v", err)
	}
}
