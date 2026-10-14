// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/wazero"
)

// repoRoot finds the directory with the go.mod of the module.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// goFiles lists the Go files of the module, without dependencies and caches.
func goFiles(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Hidden directories hold caches, tools, worktrees and agent files.
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			switch d.Name() {
			case "node_modules", "testdata", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestProductionStopMode enforces SPEC-001 amendment 10 (B.1, which replaces
// A.1): ModeCooperativeAbandon is the only production mode of expression
// evaluations. A sandbox built without options uses it, nothing outside the
// adapter and the measurement harness may name the other two modes or select a
// mode at all, and configuration, flags and cmd/orcon expose no way to choose.
func TestProductionStopMode(t *testing.T) {
	if got := wazero.New().Mode(); got != wazero.ModeCooperativeAbandon {
		t.Errorf("the default mode is %v, want %v", got, wazero.ModeCooperativeAbandon)
	}
	root := repoRoot(t)
	allowed := []string{
		filepath.Join(root, "internal", "adapter", "wazero") + string(filepath.Separator),
		filepath.Join(root, "internal", "expr", "bench") + string(filepath.Separator),
	}
	names := []string{"ModeHardLimit", "ModeCooperativeLimit", "WithMode"}
	words := []string{"cooperative", "close-on-context", "hard-limit", "hard limit"}
	files := goFiles(t)
	if len(files) < 20 {
		t.Fatalf("only %d Go files found; the walk is broken", len(files))
	}
	wired := false
	for _, path := range files {
		rel := strings.TrimPrefix(path, root+string(filepath.Separator))
		if slices.ContainsFunc(allowed, func(a string) bool { return strings.HasPrefix(path, a) }) {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if slices.Contains(names, x.Name) {
					t.Errorf("%s names %s; only internal/adapter/wazero and internal/expr/bench may select a stop mode", rel, x.Name)
				}
			case *ast.BasicLit:
				lit := strings.ToLower(x.Value)
				if slices.ContainsFunc(words, func(w string) bool { return strings.Contains(lit, w) }) && !strings.HasSuffix(path, "_test.go") {
					t.Errorf("%s has the literal %s; configuration and flags must not expose a stop mode", rel, x.Value)
				}
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && id.Name == "wazero" && x.Sel.Name == "New" && rel == filepath.Join("cmd", "orcon", "wire_serve.go") {
					wired = true
				}
			}
			return true
		})
	}
	if !wired {
		t.Error("cmd/orcon/wire_serve.go does not build the sandbox with wazero.New; the production wiring moved")
	}
}

// TestGojaIsOnlyTheParser enforces ADR-0018 section 3: goja is used for static
// analysis only, never to execute code. No non-test package may import the
// root package of goja, which is the executor; its parser packages are fine.
func TestGojaIsOnlyTheParser(t *testing.T) {
	root := repoRoot(t)
	parsers := 0
	for _, path := range goFiles(t) {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if p == "github.com/dop251/goja" {
				t.Errorf("%s imports the goja executor; only its parser packages may be used", strings.TrimPrefix(path, root+string(filepath.Separator)))
			}
			if strings.HasPrefix(p, "github.com/dop251/goja/parser") {
				parsers++
			}
		}
	}
	if parsers == 0 {
		t.Error("the check found no use of the goja parser; the walk is broken")
	}
}
