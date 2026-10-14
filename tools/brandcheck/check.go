// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// assetDirs are the canonical asset directories below brand/. Every file in
// them must be listed in the manifest (BC-5).
var assetDirs = []string{"brand/logo", "brand/icon", "brand/png", "brand/mascot"}

// appDirs are the places an app consumes brand assets from. A file there that
// equals an unapproved asset is app use of that asset.
var appDirs = []string{"apps/web/public", "packages/js/ui/src/assets/brand", "packages/js/ui/src/assets/mascot"}

// defaultBoards are the reference boards checked when there is no manifest.
var defaultBoards = map[string]Board{
	"primary":   {Path: "brand/reference/orcon-board-primary.png"},
	"secondary": {Path: "brand/reference/orcon-board-secondary.png"},
}

const readmePath = "brand/README.md"

var hexDigest = regexp.MustCompile(`\b[0-9a-fA-F]{64}\b`)

// errSymlink marks a path that goes through a symbolic link. Brand assets and
// their copies must be regular files in regular directories, so that what is
// checked is what is shipped.
var errSymlink = errors.New("path goes through a symbolic link")

// readRel reads the slash path rel below root. It refuses a path with a
// symbolic link in any component.
func readRel(root, rel string) ([]byte, error) {
	cur := root
	for _, part := range strings.Split(rel, "/") {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if err != nil {
			return nil, err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s: %w", rel, errSymlink)
		}
	}
	return os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// walkFiles lists the files below the slash directory dir as slash paths
// relative to root: regular files first, then everything that is neither a
// directory nor a regular file (symbolic links, sockets, devices). Hidden files
// are included. A directory that does not exist is empty.
func walkFiles(root, dir string) (regular, other []string, err error) {
	top := filepath.Join(root, filepath.FromSlash(dir))
	err = filepath.WalkDir(top, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			regular = append(regular, filepath.ToSlash(rel))
		} else {
			other = append(other, filepath.ToSlash(rel))
		}
		return nil
	})
	slices.Sort(regular)
	slices.Sort(other)
	return regular, other, err
}

// checkAll runs BC-1 to BC-6 over the tree at root. notes carry conditions that
// are not findings, such as a missing manifest.
func checkAll(root string) (findings []Finding, notes []string, err error) {
	m, problems, found := loadManifest(root)
	findings = append(findings, problems...)

	boards := defaultBoards
	if m != nil {
		boards = m.Boards
	}

	vectors, err := checkVectors(root, m)
	if err != nil {
		return nil, nil, err
	}
	findings = append(findings, vectors...)
	findings = append(findings, checkBoards(root, boards)...)

	switch {
	case !found:
		notes = append(notes, "no "+manifestPath+": checked only the SVG vector rules and the reference boards")
	case m != nil:
		more, err := checkManifestRules(root, m)
		if err != nil {
			return nil, nil, err
		}
		findings = append(findings, more...)
	}
	sortFindings(findings)
	return findings, notes, nil
}

// checkVectors applies BC-1 to every file that is an SVG by extension, to every
// asset the manifest declares as an SVG, and to the copies of both, so renaming
// a vector does not hide it. Compressed SVG (.svgz), symbolic links and special
// files in brand/ are findings themselves.
func checkVectors(root string, m *Manifest) ([]Finding, error) {
	files, others, err := walkFiles(root, "brand")
	if err != nil {
		return nil, fmt.Errorf("list brand files: %w", err)
	}
	var findings []Finding
	for _, f := range others {
		findings = append(findings, whole(f, ruleVector, "is a symbolic link or special file; brand assets must be regular files"))
	}
	candidates := map[string]bool{}
	for _, f := range files {
		candidates[f] = true
	}
	if m != nil {
		for _, a := range m.Assets {
			isVector := a.Kind == kindSVG || isSVGName(a.Path)
			if a.Kind == kindSVG {
				candidates[a.Path] = true
			}
			for _, c := range a.Copies {
				if isVector || isSVGName(c) {
					candidates[c] = true
				}
			}
		}
	}
	names := make([]string, 0, len(candidates))
	for f := range candidates {
		names = append(names, f)
	}
	slices.Sort(names)
	for _, f := range names {
		if strings.EqualFold(path.Ext(f), ".svgz") {
			findings = append(findings, whole(f, ruleVector, "compressed SVG (.svgz) is not allowed; brand vectors are plain .svg"))
			continue
		}
		if !candidates[f] || !(isSVGName(f) || declaredSVG(m, f)) {
			continue
		}
		data, err := readRel(root, f)
		if errors.Is(err, fs.ErrNotExist) {
			continue // a missing file is a BC-5 or BC-2 finding
		}
		if errors.Is(err, errSymlink) {
			findings = append(findings, whole(f, ruleVector, "%v; brand assets must be regular files", err))
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		findings = append(findings, checkSVG(f, data)...)
	}
	return findings, nil
}

func isSVGName(name string) bool { return strings.EqualFold(path.Ext(name), ".svg") }

// declaredSVG reports whether the manifest treats name as an SVG: an asset of
// kind svg, or a copy of one.
func declaredSVG(m *Manifest, name string) bool {
	if m == nil {
		return false
	}
	for _, a := range m.Assets {
		if a.Kind != kindSVG {
			continue
		}
		if a.Path == name || slices.Contains(a.Copies, name) {
			return true
		}
	}
	return false
}

// checkManifestRules runs BC-2 to BC-5 against a valid manifest.
func checkManifestRules(root string, m *Manifest) ([]Finding, error) {
	var findings []Finding
	listed := map[string]bool{}
	canonical := map[string][]byte{}
	boards := newBoardImages(root, m.Boards)

	for _, a := range m.Assets {
		listed[a.Path] = true
		data, err := readRel(root, a.Path)
		if errors.Is(err, fs.ErrNotExist) {
			findings = append(findings, whole(a.Path, ruleManifest, "declared in the manifest but the file does not exist"))
			continue
		}
		if errors.Is(err, errSymlink) {
			findings = append(findings, whole(a.Path, ruleManifest, "%v; assets must be regular files", err))
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", a.Path, err)
		}
		canonical[a.Path] = data
		switch a.Kind {
		case kindPNG:
			findings = append(findings, checkPNGSize(a, data)...)
		case kindICO:
			findings = append(findings, checkICO(a, data)...)
		case kindMascot:
			findings = append(findings, checkMascot(a, data, boards)...)
		}
		if len(a.Copies) > 0 && !a.Approval.approved() {
			findings = append(findings, whole(a.Path, ruleManifest,
				"wired to the app through %d copy(ies) but not approved by the owner (approval.status=%q, by=%q, date=%q)",
				len(a.Copies), a.Approval.Status, a.Approval.By, a.Approval.Date))
		}
		copyFindings, err := checkCopies(root, a, data)
		if err != nil {
			return nil, err
		}
		findings = append(findings, copyFindings...)
	}

	for _, dir := range assetDirs {
		files, _, err := walkFiles(root, dir) // links and special files are reported by checkVectors
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", dir, err)
		}
		for _, f := range files {
			if !listed[f] {
				findings = append(findings, whole(f, ruleManifest, "asset is not listed in %s", manifestPath))
			}
		}
	}

	use, err := checkUnapprovedUse(root, m, canonical)
	if err != nil {
		return nil, err
	}
	return append(findings, use...), nil
}

// checkCopies applies BC-2 to the declared copies of one asset.
func checkCopies(root string, a Asset, data []byte) ([]Finding, error) {
	var findings []Finding
	for _, c := range a.Copies {
		got, err := readRel(root, c)
		if errors.Is(err, fs.ErrNotExist) {
			findings = append(findings, whole(c, ruleCopies, "declared copy of %s does not exist", a.Path))
			continue
		}
		if errors.Is(err, errSymlink) {
			findings = append(findings, whole(c, ruleCopies, "%v; a copy must be a regular file with the bytes of %s", err, a.Path))
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", c, err)
		}
		if !bytes.Equal(got, data) {
			findings = append(findings, whole(c, ruleCopies, "is not byte-identical to %s (sha256 %.12s, want %.12s)", a.Path, digest(got), digest(data)))
		}
	}
	return findings, nil
}

// checkUnapprovedUse blocks app use of assets the owner has not approved. The
// manifest marks wired assets through copies (checked in checkManifestRules);
// this also catches a file dropped into an app directory that equals an
// unapproved canonical asset without being declared as its copy.
func checkUnapprovedUse(root string, m *Manifest, canonical map[string][]byte) ([]Finding, error) {
	unapproved := map[string]string{} // sha256 -> asset path
	declared := map[string]bool{}
	for _, a := range m.Assets {
		for _, c := range a.Copies {
			declared[c] = true
		}
		if data, ok := canonical[a.Path]; ok && !a.Approval.approved() {
			unapproved[digest(data)] = a.Path
		}
	}
	if len(unapproved) == 0 {
		return nil, nil
	}
	var findings []Finding
	for _, dir := range appDirs {
		regular, other, err := walkFiles(root, dir)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", dir, err)
		}
		// A symbolic link into an app directory is app use of its target, so it
		// is hashed through the link like any other file.
		for _, f := range append(regular, other...) {
			if declared[f] {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
			if err != nil {
				if slices.Contains(other, f) {
					continue // a dangling link or a special file has no content to match
				}
				return nil, fmt.Errorf("read %s: %w", f, err)
			}
			if asset, ok := unapproved[digest(data)]; ok {
				findings = append(findings, whole(f, ruleManifest, "is a copy of %s, which the owner has not approved", asset))
			}
		}
	}
	return findings, nil
}

// checkBoards applies BC-6: each reference board keeps the SHA-256 recorded in
// the manifest (when there is one) and in brand/README.md.
func checkBoards(root string, boards map[string]Board) []Finding {
	readme, readmeErr := readRel(root, readmePath)
	var findings []Finding
	if readmeErr != nil {
		findings = append(findings, whole(readmePath, ruleBoards, "cannot read the README that records the board hashes: %v", readmeErr))
	}
	names := make([]string, 0, len(boards))
	for n := range boards {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, name := range names {
		b := boards[name]
		data, err := readRel(root, b.Path)
		if err != nil {
			findings = append(findings, whole(b.Path, ruleBoards, "cannot read the %s board: %v", name, err))
			continue
		}
		sum := digest(data)
		if b.SHA256 != "" && b.SHA256 != sum {
			findings = append(findings, whole(b.Path, ruleBoards, "%s board changed: sha256 is %s, the manifest records %s", name, sum, b.SHA256))
		}
		if readmeErr != nil {
			continue
		}
		line, recorded, problem := recordedDigest(readme, path.Base(b.Path))
		switch {
		case problem != "":
			findings = append(findings, Finding{File: readmePath, Line: max(line, 1), Rule: ruleBoards, Detail: problem})
		case recorded != sum:
			findings = append(findings, Finding{File: readmePath, Line: line, Rule: ruleBoards,
				Detail: fmt.Sprintf("records sha256 %s for %s, the file is %s", recorded, b.Path, sum)})
		}
	}
	return findings
}

// recordedDigest finds the SHA-256 the README records for file. A line counts
// only when it names exactly that file (not a longer name that contains it) and
// the digest must be unique: a line with two different digests, or two lines
// with different digests, is ambiguous. It returns the 1-based line of the first
// digest and the digest lower-cased; problem is set when there is none or it is
// ambiguous.
func recordedDigest(readme []byte, file string) (line int, sum, problem string) {
	named := regexp.MustCompile(`(^|[^A-Za-z0-9._-])` + regexp.QuoteMeta(file) + `($|[^A-Za-z0-9._-])`)
	seen := map[string]bool{}
	for i, l := range strings.Split(string(readme), "\n") {
		if !named.MatchString(l) {
			continue
		}
		for _, m := range hexDigest.FindAllString(l, -1) {
			m = strings.ToLower(m)
			if line == 0 {
				line, sum = i+1, m
			}
			seen[m] = true
		}
	}
	switch {
	case len(seen) == 0:
		return 0, "", "does not record a sha256 for " + file
	case len(seen) > 1:
		return line, "", "records more than one sha256 on the lines that name " + file
	}
	return line, sum, ""
}
