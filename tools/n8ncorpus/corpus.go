// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// source is where the public template library is read from. The library is
// community-submitted content; Orcon downloads it at test time into a
// directory outside version control and commits only counts, hashes and
// scores (SPEC-001 decision P8, ADR-0018 section 6).
const defaultBase = "https://api.n8n.io"

// Expression is one expression found in a template.
type Expression struct {
	Template   int    `json:"template"`
	Views      int    `json:"views"`
	Node       string `json:"node"`
	NodeType   string `json:"type"`
	Path       string `json:"path"`
	Expression string `json:"expression"`
}

// Manifest describes a fetched corpus.
type Manifest struct {
	// N8nVersion is the pinned n8n release the corpus is measured against.
	N8nVersion string `json:"n8nVersion"`
	Source     string `json:"source"`
	FetchedAt  string `json:"fetchedAt"`
	Templates  int    `json:"templates"`
	// Expressions counts the expression strings found (parameter values that
	// start with "=").
	Expressions int `json:"expressions"`
	// CorpusSHA256 identifies the corpus without containing it: the digest of
	// the sorted (template, path, SHA-256 of the expression) triples.
	CorpusSHA256 string `json:"corpusSha256"`
	// NodesBaseVersion and InventorySHA256 describe inventory.json, the node
	// types of the pinned release that travel with the corpus.
	NodesBaseVersion string `json:"nodesBaseVersion,omitempty"`
	InventorySHA256  string `json:"inventorySha256,omitempty"`
}

type fetcher struct {
	base    string
	client  *http.Client
	delay   time.Duration
	workers int
	userAg  string
}

type listing struct {
	Workflows []struct {
		ID         int `json:"id"`
		TotalViews int `json:"totalViews"`
	} `json:"workflows"`
}

type template struct {
	Workflow struct {
		ID         int `json:"id"`
		TotalViews int `json:"totalViews"`
		Workflow   struct {
			Nodes []struct {
				Name       string          `json:"name"`
				Type       string          `json:"type"`
				Parameters json.RawMessage `json:"parameters"`
			} `json:"nodes"`
		} `json:"workflow"`
	} `json:"workflow"`
}

func (f *fetcher) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	u := f.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return f.fetch(ctx, u, 32<<20)
}

// fetch downloads one URL, retrying on rate limits and server errors, and
// reads at most max bytes.
func (f *fetcher) fetch(ctx context.Context, u string, max int64) ([]byte, error) {
	for attempt := range 4 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", f.userAg)
		resp, err := f.client.Do(req)
		if err != nil {
			return nil, err
		}
		body, rerr := io.ReadAll(io.LimitReader(resp.Body, max))
		resp.Body.Close()
		if rerr != nil {
			return nil, rerr
		}
		switch {
		case resp.StatusCode == http.StatusOK:
			return body, nil
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 2 * time.Second):
			}
		default:
			return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
		}
	}
	return nil, fmt.Errorf("GET %s: too many failed attempts", u)
}

// topTemplates lists the ids of the most viewed templates, with their views.
func (f *fetcher) topTemplates(ctx context.Context, limit int) ([][2]int, error) {
	const pageSize = 100
	var out [][2]int
	for page := 1; len(out) < limit; page++ {
		q := url.Values{"page": {strconv.Itoa(page)}, "rows": {strconv.Itoa(pageSize)}, "sort": {"views:desc"}}
		body, err := f.get(ctx, "/api/templates/search", q)
		if err != nil {
			return nil, err
		}
		var l listing
		if err := json.Unmarshal(body, &l); err != nil {
			return nil, fmt.Errorf("search page %d: %w", page, err)
		}
		if len(l.Workflows) == 0 {
			break
		}
		for _, w := range l.Workflows {
			if len(out) < limit {
				out = append(out, [2]int{w.ID, w.TotalViews})
			}
		}
		time.Sleep(f.delay)
	}
	return out, nil
}

// fetchAll downloads the templates that are not cached yet into
// dir/templates.
func (f *fetcher) fetchAll(ctx context.Context, dir string, ids [][2]int) error {
	tdir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(tdir, 0o750); err != nil {
		return err
	}
	jobs := make(chan [2]int)
	errs := make(chan error, len(ids))
	var wg sync.WaitGroup
	for range f.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				path := filepath.Join(tdir, strconv.Itoa(id[0])+".json")
				if _, err := os.Stat(path); err == nil {
					continue
				}
				body, err := f.get(ctx, "/api/templates/workflows/"+strconv.Itoa(id[0]), nil)
				if err != nil {
					errs <- err
					continue
				}
				if err := os.WriteFile(path, body, 0o600); err != nil {
					errs <- err
				}
				time.Sleep(f.delay)
			}
		}()
	}
	for _, id := range ids {
		select {
		case jobs <- id:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	close(errs)
	var failed []error
	for err := range errs {
		failed = append(failed, err)
	}
	if len(failed) > len(ids)/10 {
		return fmt.Errorf("%d of %d templates failed to download, first: %w", len(failed), len(ids), failed[0])
	}
	return nil
}

// collect walks the parameters of a node and returns the expression strings
// with their paths.
func collect(raw json.RawMessage, path string, visit func(path, expr string)) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return
	}
	var walk func(v any, path string)
	walk = func(v any, path string) {
		switch t := v.(type) {
		case string:
			if strings.HasPrefix(t, "=") {
				visit(path, t)
			}
		case []any:
			for i, x := range t {
				walk(x, path+"["+strconv.Itoa(i)+"]")
			}
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(t[k], path+"."+k)
			}
		}
	}
	walk(v, path)
}

// extract reads the cached templates and writes expressions.jsonl and
// manifest.json.
func extract(dir, n8nVersion, source string, ids [][2]int) (Manifest, error) {
	var exprs []Expression
	templates := 0
	for _, id := range ids {
		body, err := os.ReadFile(filepath.Join(dir, "templates", strconv.Itoa(id[0])+".json"))
		if err != nil {
			continue // the download failed; the manifest counts what exists
		}
		var t template
		if err := json.Unmarshal(body, &t); err != nil {
			continue
		}
		templates++
		views := id[1]
		for _, n := range t.Workflow.Workflow.Nodes {
			collect(n.Parameters, "parameters", func(path, e string) {
				exprs = append(exprs, Expression{Template: id[0], Views: views, Node: n.Name, NodeType: n.Type, Path: path, Expression: e})
			})
		}
	}
	sort.SliceStable(exprs, func(i, j int) bool {
		a, b := exprs[i], exprs[j]
		if a.Template != b.Template {
			return a.Template < b.Template
		}
		if a.Node != b.Node {
			return a.Node < b.Node
		}
		return a.Path < b.Path
	})
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	h := sha256.New()
	for _, e := range exprs {
		if err := enc.Encode(e); err != nil {
			return Manifest{}, err
		}
		sum := sha256.Sum256([]byte(e.Expression))
		fmt.Fprintf(h, "%d\t%s\t%s\t%s\n", e.Template, e.Node, e.Path, hex.EncodeToString(sum[:]))
	}
	if err := os.WriteFile(filepath.Join(dir, "expressions.jsonl"), out.Bytes(), 0o600); err != nil {
		return Manifest{}, err
	}
	m := Manifest{
		N8nVersion:   n8nVersion,
		Source:       source,
		FetchedAt:    time.Now().UTC().Format(time.RFC3339),
		Templates:    templates,
		Expressions:  len(exprs),
		CorpusSHA256: hex.EncodeToString(h.Sum(nil)),
	}
	mb, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), append(mb, '\n'), 0o600); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

var errNoTemplates = errors.New("n8ncorpus: the library returned no templates")
