// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"bufio"
	"bytes"
	"flag"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/platform/config"
)

// fakeFile is a file of the fake file system.
type fakeFile struct {
	data string
	mode fs.FileMode
}

// fakeFS is a hand-written file system for the loader.
type fakeFS map[string]fakeFile

func (f fakeFS) ReadFile(name string, limit int64) ([]byte, error) {
	file, ok := f[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	data := []byte(file.data)
	if int64(len(data)) > limit+1 {
		data = data[:limit+1]
	}
	return data, nil
}

func (f fakeFS) Stat(name string) (fs.FileInfo, error) {
	file, ok := f[name]
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	return fakeInfo{name: name, mode: file.mode}, nil
}

type fakeInfo struct {
	name string
	mode fs.FileMode
}

func (i fakeInfo) Name() string      { return i.name }
func (fakeInfo) Size() int64         { return 0 }
func (i fakeInfo) Mode() fs.FileMode { return i.mode }
func (fakeInfo) ModTime() time.Time  { return time.Time{} }
func (fakeInfo) IsDir() bool         { return false }
func (fakeInfo) Sys() any            { return nil }

// fakeDecode reads a flat file: one `dotted.path: value` per line. A value in
// brackets is a list; `{a=b}` is a map; an empty value is null. It stands in for
// the YAML adapter, which this package must not import.
func fakeDecode(data []byte, _ func(string) bool) ([]config.FileEntry, error) {
	var out []config.FileEntry
	sc := bufio.NewScanner(bytes.NewReader(data))
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		path, value, _ := strings.Cut(text, ":")
		path, value = strings.TrimSpace(path), strings.TrimSpace(value)
		e := config.FileEntry{Path: path, Line: line, Scalar: value}
		switch {
		case value == "":
			e.Kind = config.EntryNull
		case strings.HasPrefix(value, "["):
			e.Kind, e.Scalar = config.EntryList, ""
			for item := range strings.SplitSeq(strings.Trim(value, "[]"), ",") {
				if item = strings.TrimSpace(item); item != "" {
					e.List = append(e.List, item)
				}
			}
		case strings.HasPrefix(value, "{"):
			e.Kind, e.Scalar, e.Map = config.EntryMap, "", map[string]string{}
			for item := range strings.SplitSeq(strings.Trim(value, "{}"), ",") {
				name, val, _ := strings.Cut(strings.TrimSpace(item), "=")
				e.Map[name] = val
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// env builds an environment from alternating names and values.
func env(pairs ...string) []string {
	out := make([]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, pairs[i]+"="+pairs[i+1])
	}
	return out
}

// baseEnv is the minimum a Linux process has, so the data directory default resolves.
func baseEnv(extra ...string) []string {
	return env(append([]string{"HOME", "/home/orcon"}, extra...)...)
}

// input is the test input of a load.
type input struct {
	args    []string
	environ []string
	files   fakeFS
	goos    string
}

// load runs the loader like orcon config check does: flags from args, the fake file system, the fake decoder.
func load(t *testing.T, in input) *config.Result {
	t.Helper()
	schema := config.Default()
	fset := flag.NewFlagSet("test", flag.ContinueOnError)
	fset.SetOutput(io.Discard)
	fv := schema.BindFlags(fset)
	if err := schema.ParseFlags(fset, in.args); err != nil {
		t.Fatalf("ParseFlags(%q) = %v", in.args, err)
	}
	if in.environ == nil {
		in.environ = baseEnv()
	}
	if in.goos == "" {
		in.goos = "linux"
	}
	return schema.Load(config.Input{Flags: fv, Environ: in.environ, FS: in.files, Decode: fakeDecode, GOOS: in.goos})
}

// issuesFor returns the messages of the issues about key, errors first.
func issuesFor(res *config.Result, key string) []config.Issue {
	var out []config.Issue
	for _, i := range res.Issues {
		if i.Key == key {
			out = append(out, i)
		}
	}
	return out
}

func hasIssue(res *config.Result, sev config.Severity, key, contains string) bool {
	for _, i := range issuesFor(res, key) {
		if i.Severity == sev && strings.Contains(i.Message, contains) {
			return true
		}
	}
	return false
}
