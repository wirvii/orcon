// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"time"
)

// manifestPath is the brand manifest, relative to the repository root.
const manifestPath = "brand/manifest.json"

// Asset kinds of the manifest.
const (
	kindSVG    = "svg"
	kindPNG    = "png"
	kindICO    = "ico"
	kindMascot = "mascot"
)

// Approval states of an asset.
const (
	statusPending  = "pending"
	statusApproved = "approved"
	statusRejected = "rejected"
)

var (
	sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)
	masks     = []string{"brand-hue", "light-on-dark", "dark-on-light", "tile"}
)

// Manifest is the content of brand/manifest.json. Unknown fields are an error:
// a misspelled key must not silently disable a check.
type Manifest struct {
	SchemaVersion int              `json:"schema_version"`
	Boards        map[string]Board `json:"boards"`
	Assets        []Asset          `json:"assets"`
}

// Board is a reference board and the SHA-256 it must keep.
type Board struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Asset is one production asset.
type Asset struct {
	Path     string     `json:"path"`
	Kind     string     `json:"kind"`
	Source   *Source    `json:"source,omitempty"`
	Fidelity []Fidelity `json:"fidelity,omitempty"`
	From     string     `json:"from,omitempty"`
	Width    int        `json:"width,omitempty"`
	Height   int        `json:"height,omitempty"`
	Entries  []int      `json:"entries,omitempty"`
	MaxBytes int        `json:"max_bytes,omitempty"`
	Copies   []string   `json:"copies"`
	Approval Approval   `json:"approval"`
}

// Source is the board region an asset was traced or cut from. Region is
// [x, y, width, height] in board pixels.
type Source struct {
	Board  string `json:"board"`
	Region []int  `json:"region"`
}

// Fidelity is a visual comparison measured by the review page, not here;
// brandcheck only checks that the list is well formed.
type Fidelity struct {
	Name   string  `json:"name"`
	Region []int   `json:"region"`
	Mask   string  `json:"mask"`
	IoUMin float64 `json:"iou_min"`
}

// Approval is the owner's decision about an asset.
type Approval struct {
	Status string `json:"status"`
	By     string `json:"by,omitempty"`
	Date   string `json:"date,omitempty"`
	Note   string `json:"note,omitempty"`
}

// approved reports whether the owner approved the asset, naming who and when.
func (a Approval) approved() bool {
	return a.Status == statusApproved && a.By != "" && a.Date != ""
}

// loadManifest reads brand/manifest.json under root. The result found is false
// when the file does not exist. A file that cannot be read, parsed or validated yields
// findings and a nil manifest.
func loadManifest(root string) (m *Manifest, findings []Finding, found bool) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manifestPath)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, false
	}
	if err != nil {
		return nil, []Finding{whole(manifestPath, ruleManifest, "cannot read manifest: %v", err)}, true
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var parsed Manifest
	if err := dec.Decode(&parsed); err != nil {
		return nil, []Finding{whole(manifestPath, ruleManifest, "invalid manifest JSON: %v", err)}, true
	}
	if dec.More() {
		return nil, []Finding{whole(manifestPath, ruleManifest, "invalid manifest JSON: trailing content after the document")}, true
	}
	if problems := validateManifest(&parsed); len(problems) > 0 {
		return nil, problems, true
	}
	return &parsed, nil, true
}

// validateManifest checks the shape of a parsed manifest.
func validateManifest(m *Manifest) []Finding {
	v := manifestValidator{}
	if m.SchemaVersion != 1 {
		v.add("schema_version is %d, want 1", m.SchemaVersion)
	}
	for _, name := range []string{"primary", "secondary"} {
		b, ok := m.Boards[name]
		switch {
		case !ok:
			v.add("boards.%s is missing", name)
		case !fs.ValidPath(b.Path) || b.Path == ".":
			v.add("boards.%s.path %q is not a clean relative path", name, b.Path)
		case !sha256Hex.MatchString(b.SHA256):
			v.add("boards.%s.sha256 is not 64 lower-case hex digits", name)
		}
	}
	seen := map[string]bool{}
	for _, a := range m.Assets {
		v.asset(m, a, seen)
	}
	return v.findings
}

type manifestValidator struct{ findings []Finding }

func (v *manifestValidator) add(format string, args ...any) {
	v.findings = append(v.findings, whole(manifestPath, ruleManifest, format, args...))
}

func (v *manifestValidator) asset(m *Manifest, a Asset, seen map[string]bool) {
	if !fs.ValidPath(a.Path) || !isUnder(a.Path, "brand") {
		v.add("asset path %q is not a clean relative path below brand/", a.Path)
		return
	}
	if seen[a.Path] {
		v.add("asset %s is listed twice", a.Path)
	}
	seen[a.Path] = true
	switch a.Kind {
	case kindSVG:
	case kindPNG:
		if a.Width <= 0 || a.Height <= 0 {
			v.add("asset %s: png needs a positive width and height", a.Path)
		}
		if a.From != "" && !fs.ValidPath(a.From) {
			v.add("asset %s: from %q is not a clean relative path", a.Path, a.From)
		}
	case kindICO:
		if len(a.Entries) == 0 {
			v.add("asset %s: ico needs a non-empty entries list", a.Path)
		}
	case kindMascot:
		if a.Source == nil {
			v.add("asset %s: mascot needs a source board region", a.Path)
		}
	default:
		v.add("asset %s: kind %q is not one of svg, png, ico, mascot", a.Path, a.Kind)
	}
	if a.Source != nil {
		if _, ok := m.Boards[a.Source.Board]; !ok {
			v.add("asset %s: source board %q is not declared in boards", a.Path, a.Source.Board)
		}
		v.region(a.Path+" source", a.Source.Region)
	}
	for i, f := range a.Fidelity {
		label := a.Path + " fidelity[" + strconv.Itoa(i) + "]"
		if f.Name == "" {
			v.add("%s: name is empty", label)
		}
		v.region(label, f.Region)
		if !slices.Contains(masks, f.Mask) {
			v.add("%s: mask %q is not one of %v", label, f.Mask, masks)
		}
		if f.IoUMin <= 0 || f.IoUMin > 1 {
			v.add("%s: iou_min %v is outside (0, 1]", label, f.IoUMin)
		}
	}
	for _, c := range a.Copies {
		if !fs.ValidPath(c) || c == a.Path {
			v.add("asset %s: copy %q is not a clean relative path distinct from the asset", a.Path, c)
		}
	}
	v.approval(a)
}

func (v *manifestValidator) region(label string, r []int) {
	if len(r) != 4 || r[0] < 0 || r[1] < 0 || r[2] <= 0 || r[3] <= 0 {
		v.add("%s: region must be [x, y, width, height] with x,y >= 0 and width,height > 0", label)
	}
}

func (v *manifestValidator) approval(a Asset) {
	switch a.Approval.Status {
	case statusPending, statusRejected:
	case statusApproved:
		if a.Approval.By == "" {
			v.add("asset %s: approved without approval.by", a.Path)
		}
		if _, err := time.Parse(time.DateOnly, a.Approval.Date); err != nil {
			v.add("asset %s: approved without a YYYY-MM-DD approval.date", a.Path)
		}
	default:
		v.add("asset %s: approval.status %q is not one of pending, approved, rejected", a.Path, a.Approval.Status)
	}
}

// isUnder reports whether the slash path p is inside directory dir.
func isUnder(p, dir string) bool {
	return len(p) > len(dir)+1 && p[:len(dir)] == dir && p[len(dir)] == '/'
}
