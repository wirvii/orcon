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

// readRel reads the slash path rel below root.
func readRel(root, rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// walkFiles lists the regular files below the slash directory dir, as slash
// paths relative to root. A directory that does not exist is empty. Names that
// start with a dot (editor and OS litter) are skipped.
func walkFiles(root, dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && p != filepath.Join(root, filepath.FromSlash(dir)) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	slices.Sort(files)
	return files, err
}

// checkAll runs BC-1 to BC-6 over the tree at root. notes carry conditions that
// are not findings, such as a missing manifest.
func checkAll(root string) (findings []Finding, notes []string, err error) {
	m, problems, found := loadManifest(root)
	findings = append(findings, problems...)

	boards := defaultBoards
	var copies []string
	if m != nil {
		boards = m.Boards
		for _, a := range m.Assets {
			copies = append(copies, a.Copies...)
		}
	}

	vectors, err := checkVectors(root, copies)
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

// checkVectors applies BC-1 to every SVG below brand/ and to every SVG copy
// the manifest declares.
func checkVectors(root string, copies []string) ([]Finding, error) {
	files, err := walkFiles(root, "brand")
	if err != nil {
		return nil, fmt.Errorf("list brand files: %w", err)
	}
	for _, c := range copies {
		if !slices.Contains(files, c) {
			files = append(files, c)
		}
	}
	var findings []Finding
	for _, f := range files {
		if !strings.EqualFold(path.Ext(f), ".svg") {
			continue
		}
		data, err := readRel(root, f)
		if errors.Is(err, fs.ErrNotExist) {
			continue // a missing copy is a BC-2 finding
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		findings = append(findings, checkSVG(f, data)...)
	}
	return findings, nil
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
		files, err := walkFiles(root, dir)
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
		files, err := walkFiles(root, dir)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", dir, err)
		}
		for _, f := range files {
			if declared[f] {
				continue
			}
			data, err := readRel(root, f)
			if err != nil {
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
		line, recorded := recordedDigest(readme, path.Base(b.Path))
		switch {
		case recorded == "":
			findings = append(findings, whole(readmePath, ruleBoards, "does not record a sha256 for %s", b.Path))
		case recorded != sum:
			findings = append(findings, Finding{File: readmePath, Line: line, Rule: ruleBoards,
				Detail: fmt.Sprintf("records sha256 %s for %s, the file is %s", recorded, b.Path, sum)})
		}
	}
	return findings
}

// recordedDigest finds the line of the README that names file and carries a
// SHA-256, and returns its 1-based number and the lower-cased digest.
func recordedDigest(readme []byte, file string) (line int, digest string) {
	for i, l := range strings.Split(string(readme), "\n") {
		if !strings.Contains(l, file) {
			continue
		}
		if m := hexDigest.FindString(l); m != "" {
			return i + 1, strings.ToLower(m)
		}
	}
	return 0, ""
}
