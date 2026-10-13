// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// gitRunner runs "git <args>" in dir and returns its standard output.
type gitRunner interface {
	Run(dir string, args ...string) ([]byte, error)
}

type execGit struct{}

func (execGit) Run(dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...) //nolint:gosec // fixed binary, arguments built by this tool
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// errNoBase is returned when no base revision can be determined.
var errNoBase = errors.New("no base revision")

// resolveBase decides the revision migrations.sum is compared with: the
// explicit value (flag or DBCHECK_BASE), else the merge base of HEAD and
// origin/main. A base that was asked for but cannot be resolved is an error;
// only the absence of any reference is "no base" (and is declared by the
// caller, never silent).
func resolveBase(g gitRunner, dir, explicit string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("%w: git is not available", errNoBase)
	}
	if explicit != "" {
		out, err := g.Run(dir, "rev-parse", "--verify", "--quiet", explicit+"^{commit}")
		if err != nil {
			return "", fmt.Errorf("base revision %q cannot be resolved: %w", explicit, err)
		}
		return strings.TrimSpace(string(out)), nil
	}
	out, err := g.Run(dir, "merge-base", "HEAD", "origin/main")
	if err != nil {
		return "", fmt.Errorf("%w: no merge base with origin/main (%v)", errNoBase, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// showFile returns the content of rel (relative to dir) at rev, and whether
// the file exists there.
func showFile(g gitRunner, dir, rev, rel string) ([]byte, bool, error) {
	listing, err := g.Run(dir, "ls-tree", "--name-only", rev, "--", "./"+rel)
	if err != nil {
		return nil, false, err
	}
	if len(bytes.TrimSpace(listing)) == 0 {
		return nil, false, nil
	}
	data, err := g.Run(dir, "show", rev+":./"+rel)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// releaseTags lists release tags (v*), newest first.
func releaseTags(g gitRunner, dir string) ([]string, error) {
	if g == nil {
		return nil, fmt.Errorf("%w: git is not available", errNoBase)
	}
	out, err := g.Run(dir, "tag", "--list", "v*", "--sort=-v:refname")
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			tags = append(tags, l)
		}
	}
	return tags, nil
}

// listDir lists the file names directly inside rel (relative to dir) at rev.
func listDir(g gitRunner, dir, rev, rel string) ([]string, error) {
	out, err := g.Run(dir, "ls-tree", "--name-only", rev, "--", "./"+rel+"/")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			names = append(names, l[strings.LastIndex(l, "/")+1:])
		}
	}
	return names, nil
}
