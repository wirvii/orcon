// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Limits of corpus loading. A corpus is untrusted input from a public
// library, so every dimension is bounded.
const (
	maxTemplateBytes = 32 << 20
	maxTemplates     = 200_000
)

// Manifest describes a fetched corpus (the manifest.json that
// tools/n8ncorpus writes).
type Manifest struct {
	// N8nVersion is the pinned n8n release the corpus is measured against.
	N8nVersion string
	Source     string
	FetchedAt  string
	// CorpusSHA256 identifies the corpus without containing it.
	CorpusSHA256 string
}

// Corpus is a set of templates ready to measure.
type Corpus struct {
	Manifest  Manifest
	Templates []Template
	// Unreadable counts template files that could not be parsed; they are
	// left out of every figure and reported.
	Unreadable int
}

// ErrEmptyCorpus marks a directory with no readable template.
var ErrEmptyCorpus = errors.New("n8nimport: the corpus holds no readable template")

// LoadCorpus reads a corpus directory: templates/<id>.json, one file per
// template, and an optional manifest.json. Templates come out sorted by id.
func LoadCorpus(fsys fs.FS) (*Corpus, error) {
	c := &Corpus{}
	if data, err := fs.ReadFile(fsys, "manifest.json"); err == nil {
		if v, perr := jsonvalue.Parse(data); perr == nil {
			c.Manifest = Manifest{
				N8nVersion:   strMember(v, "n8nVersion"),
				Source:       strMember(v, "source"),
				FetchedAt:    strMember(v, "fetchedAt"),
				CorpusSHA256: strMember(v, "corpusSha256"),
			}
		}
	}
	entries, err := fs.ReadDir(fsys, "templates")
	if err != nil {
		return nil, fmt.Errorf("n8nimport: read corpus: %w", err)
	}
	if len(entries) > maxTemplates {
		return nil, fmt.Errorf("n8nimport: the corpus holds more than %d templates", maxTemplates)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		t, err := readTemplate(fsys, "templates/"+name)
		if err != nil {
			c.Unreadable++
			continue
		}
		if id, perr := strconv.Atoi(strings.TrimSuffix(name, ".json")); perr == nil && t.ID == 0 {
			t.ID = id
		}
		c.Templates = append(c.Templates, t)
	}
	if len(c.Templates) == 0 {
		return nil, ErrEmptyCorpus
	}
	slices.SortFunc(c.Templates, func(a, b Template) int { return a.ID - b.ID })
	return c, nil
}

func readTemplate(fsys fs.FS, name string) (Template, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return Template{}, fmt.Errorf("open %s: %w", name, err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxTemplateBytes+1))
	if err != nil {
		return Template{}, fmt.Errorf("read %s: %w", name, err)
	}
	if len(data) > maxTemplateBytes {
		return Template{}, fmt.Errorf("%s is larger than %d bytes", name, maxTemplateBytes)
	}
	return ParseTemplate(data)
}
