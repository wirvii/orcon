// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package authoring holds the checks an official connector package passes beyond
// the manifest rules of package connector. This file covers the brand icon of a
// package (orcon-connector IC-1, IC-6 and IC-8; SPEC-019 R-UX-ICO-08).
package authoring

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
)

// Rule identifiers of the icon checks, as orcon connector lint reports them.
const (
	RuleIconMissing       = "icon-missing"
	RuleIconSourceMissing = "icon-source-missing"
	RuleIconPlaceholder   = "icon-placeholder"
)

const (
	iconFile   = "icons/icon.svg"
	sourceFile = "icons/SOURCE"
)

// sourceLine is the one line IC-6 allows in icons/SOURCE: a Simple Icons version and
// slug (simple-icons@<version>:<slug>), a brand press kit address (brand-kit:<url>),
// or the Orcon fallback glyph (orcon-fallback).
var sourceLine = regexp.MustCompile(`^(simple-icons@[0-9]+\.[0-9]+\.[0-9]+:[a-z0-9-]+|brand-kit:https://\S+|orcon-fallback)$`)

// Finding is one problem an icon rule found in a connector package.
type Finding struct {
	// Rule is the rule identifier, one of the Rule constants.
	Rule string
	// Message says what is wrong and what fixes it, in English.
	Message string
}

// Placeholders is the set of SHA-256 digests (lowercase hex) of generic marks that
// must not ship as the icon of a connector.
type Placeholders map[string]bool

// ParsePlaceholders reads the placeholder register: one lowercase SHA-256 hex digest
// per line, optionally followed by a space and a note. Lines starting with # are
// comments and blank lines are skipped.
func ParsePlaceholders(text string) (Placeholders, error) {
	set := Placeholders{}
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		digest, _, _ := strings.Cut(line, " ")
		if len(digest) != sha256.Size*2 {
			return nil, fmt.Errorf("placeholder register line %d: %q is not a SHA-256 digest", i+1, digest)
		}
		if _, err := hex.DecodeString(digest); err != nil || strings.ToLower(digest) != digest {
			return nil, fmt.Errorf("placeholder register line %d: %q is not a lowercase hex digest", i+1, digest)
		}
		set[digest] = true
	}
	return set, nil
}

// CheckIcon applies the icon rules to one connector package whose root is fsys. It
// returns no findings for a package with a real brand icon and a valid source.
func CheckIcon(fsys fs.FS, placeholders Placeholders) []Finding {
	icon, err := fs.ReadFile(fsys, iconFile)
	if err != nil {
		return []Finding{{
			Rule:    RuleIconMissing,
			Message: "the package has no icons/icon.svg; add the service's own mark (orcon-connector IC-1)",
		}}
	}
	var findings []Finding
	source, err := fs.ReadFile(fsys, sourceFile)
	if err != nil || !sourceLine.MatchString(strings.TrimSpace(string(source))) {
		findings = append(findings, Finding{
			Rule:    RuleIconSourceMissing,
			Message: "icons/SOURCE is missing or is not one line of simple-icons@<version>:<slug>, brand-kit:<url> or orcon-fallback (orcon-connector IC-6)",
		})
	}
	sum := sha256.Sum256(icon)
	if placeholders[hex.EncodeToString(sum[:])] {
		findings = append(findings, Finding{
			Rule:    RuleIconPlaceholder,
			Message: "icons/icon.svg is a generic placeholder mark; replace it with the service's own mark (orcon-connector IC-8)",
		})
	}
	return findings
}
