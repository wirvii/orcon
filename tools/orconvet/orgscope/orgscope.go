// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package orgscope is wall 2(a) of organization isolation (ADR-0046 section 3,
// SPEC-004 R-ISO-02). It enforces two rules.
//
// Rule 1, SQL stays in the data-access layer. Any reference to a symbol of a SQL
// driver package (database/sql, pgx, pgconn, the SQLite drivers, lib/pq, sqlx)
// from a package outside internal/adapter/sqlite and internal/adapter/postgres
// is reported: a call, a method value such as db.Exec passed around, a type
// name, a variable. So is a call to any function that returns a driver value
// (a helper handing out a *sql.DB). The declared exceptions are
// internal/adapter/storetest (the contract suites) and
// internal/adapter/connectors (SQL against the user's own systems, never
// Orcon's store).
//
// Rule 2, SQL text is a constant. The statement passed to a driver call
// (Exec, Query, QueryRow, Prepare, Queue, ... and their Context forms; every
// string parameter except a prepared statement's name) or to a method that the
// data-access layer itself declares (the sqlc DBTX interface) must be a
// constant expression. sqlc emits its statements as constants, so generated code
// passes. A concatenation, fmt.Sprintf, a variable, a parameter, a field, a
// closure result, a helper's result: all are reported, whatever they are made
// of. The rule holds in every package except the two exceptions above.
//
// The one escape is a directive on the call line or the line above:
//
//	//orcon:dynamic-sql <reason>
//
// for the few places that must run text they did not write (a migration runner
// executing an embedded file). It needs a reason, and it is honoured only in the
// files listed in internal/adapter/storetest/dbrules/policy/dynamic_sql_allowlist.txt
// (path, reason, date, approved-by, added with owner approval); in any other file
// the annotation is itself a finding.
//
// What this does not do: it does not read the statement (tools/sqlcheck does,
// on the query files), and it does not check that a store method receives an
// orgscope.Org (that type arrives with the first tables). Import rules of the
// depguard configuration remain the backstop for the drivers' import paths;
// lint-fixtures plants an import of database/sql and of pgx to prove it.
package orgscope

import (
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

// Analyzer is the orgscope analyzer.
var Analyzer = newAnalyzer()

// driverPrefixes are the import path prefixes of SQL drivers and wrappers.
var driverPrefixes = []string{
	"database/sql",
	"github.com/jackc/pgx",
	"github.com/jackc/pgconn",
	"github.com/lib/pq",
	"github.com/mattn/go-sqlite3",
	"github.com/jmoiron/sqlx",
	"modernc.org/sqlite",
}

// dataAccessPackages are the packages that hold Orcon's SQL; a package whose
// import path contains one of them as a run of path elements is part of it.
var dataAccessPackages = []string{
	"internal/adapter/sqlite",
	"internal/adapter/postgres",
}

// exemptPackages may use a driver, and build statement text, without being the
// data-access layer.
var exemptPackages = []string{
	"internal/adapter/storetest",
	"internal/adapter/connectors",
}

// sinkNames are the methods and functions that take SQL text.
var sinkNames = map[string]bool{
	"Exec": true, "ExecContext": true, "MustExec": true, "MustExecContext": true,
	"Query": true, "QueryContext": true, "Queryx": true, "QueryxContext": true,
	"QueryRow": true, "QueryRowContext": true, "QueryRowx": true, "QueryRowxContext": true,
	"Prepare": true, "PrepareContext": true, "Preparex": true, "PreparexContext": true,
	"PrepareNamed": true, "PrepareNamedContext": true,
	"NamedExec": true, "NamedExecContext": true, "NamedQuery": true, "NamedQueryContext": true,
	"Get": true, "GetContext": true, "Select": true, "SelectContext": true,
	"Queue": true,
}

// handleMethods are the methods of a database handle interface that the layer
// declares (sqlc's DBTX).
var handleMethods = map[string]bool{
	"Exec": true, "ExecContext": true, "Query": true, "QueryContext": true,
	"QueryRow": true, "QueryRowContext": true, "Prepare": true, "PrepareContext": true,
}

// directive marks a reviewed exception to rule 2.
const directive = "//orcon:dynamic-sql"

func run(pass *analysis.Pass) (any, error) {
	pkgPath := strings.TrimSuffix(pass.Pkg.Path(), "_test")
	exempt := containsAny(pkgPath, exemptPackages)
	inLayer := exempt || containsAny(pkgPath, dataAccessPackages)
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint:errcheck,forcetypeassert // the Requires clause guarantees the type
	allowed := directiveLines(pass)

	if !inLayer {
		insp.Preorder([]ast.Node{(*ast.Ident)(nil), (*ast.CallExpr)(nil)}, func(n ast.Node) {
			switch n := n.(type) {
			case *ast.Ident:
				reportDriverUse(pass, n)
			case *ast.CallExpr:
				reportDriverValue(pass, n)
			}
		})
	}
	if exempt {
		return nil, nil //nolint:nilnil // analyzers without a result return nil
	}
	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		call := n.(*ast.CallExpr) //nolint:errcheck,forcetypeassert // filtered by the node types above
		checkSink(pass, call, allowed)
	})
	if inLayer {
		reportSinkValues(pass, insp)
	}
	return nil, nil //nolint:nilnil // analyzers without a result return nil
}

// reportDriverUse reports an identifier that resolves to a symbol of a driver package.
func reportDriverUse(pass *analysis.Pass, id *ast.Ident) {
	obj := pass.TypesInfo.Uses[id]
	if obj == nil || obj.Pkg() == nil || !isDriver(obj.Pkg().Path()) {
		return
	}
	if _, isPkg := obj.(*types.PkgName); isPkg {
		return
	}
	pass.Reportf(id.Pos(), "SQL access (%s.%s) outside the data-access layer; SQL lives only in internal/adapter/sqlite and internal/adapter/postgres", obj.Pkg().Name(), obj.Name())
}

// reportDriverValue reports a call, to code that is not a driver itself, whose
// result holds a driver value (a helper that hands out a *sql.DB).
func reportDriverValue(pass *analysis.Pass, call *ast.CallExpr) {
	if fn, ok := typeutil.Callee(pass.TypesInfo, call).(*types.Func); ok && fn.Pkg() != nil && isDriver(fn.Pkg().Path()) {
		return // reported as a use of the driver symbol
	}
	tv, ok := pass.TypesInfo.Types[call]
	if !ok || tv.Type == nil {
		return
	}
	if tuple, isTuple := tv.Type.(*types.Tuple); isTuple {
		for i := range tuple.Len() {
			if holdsDriver(tuple.At(i).Type(), 0) {
				pass.Reportf(call.Pos(), "call returns a SQL driver value outside the data-access layer; SQL lives only in internal/adapter/sqlite and internal/adapter/postgres")
				return
			}
		}
		return
	}
	if holdsDriver(tv.Type, 0) {
		pass.Reportf(call.Pos(), "call returns a SQL driver value outside the data-access layer; SQL lives only in internal/adapter/sqlite and internal/adapter/postgres")
	}
}

// holdsDriver reports whether t is, or is made of, a type of a driver package.
func holdsDriver(t types.Type, depth int) bool {
	if depth > 4 || t == nil {
		return false
	}
	switch t := t.(type) {
	case *types.Named:
		if t.Obj().Pkg() != nil && isDriver(t.Obj().Pkg().Path()) {
			return true
		}
		return false
	case *types.Pointer:
		return holdsDriver(t.Elem(), depth+1)
	case *types.Slice:
		return holdsDriver(t.Elem(), depth+1)
	case *types.Array:
		return holdsDriver(t.Elem(), depth+1)
	case *types.Map:
		return holdsDriver(t.Elem(), depth+1)
	case *types.Chan:
		return holdsDriver(t.Elem(), depth+1)
	}
	return false
}

// checkSink applies rule 2 to a call.
func checkSink(pass *analysis.Pass, call *ast.CallExpr, allowed map[lineKey]string) {
	fn, ok := typeutil.Callee(pass.TypesInfo, call).(*types.Func)
	if !ok {
		return
	}
	sig, ok := sinkSignature(fn)
	if !ok {
		return
	}
	params := sig.Params()
	for i := range params.Len() {
		p := params.At(i)
		basic, isBasic := p.Type().Underlying().(*types.Basic)
		if !isBasic || basic.Kind() != types.String || p.Name() == "name" || i >= len(call.Args) {
			continue
		}
		arg := call.Args[i]
		if tv, known := pass.TypesInfo.Types[arg]; known && tv.Value != nil {
			continue // a constant, however it is spelled
		}
		if reason, found := directiveFor(pass, call, allowed); found {
			if reason == "" {
				pass.Reportf(call.Pos(), "%s needs a reason after the directive", directive)
			}
			continue
		}
		pass.Reportf(arg.Pos(), "SQL text must be a constant: write the statement as a constant (sqlc does) and pass values as parameters (orcon-db TI-8); a reviewed exception takes %s <reason>", directive)
	}
}

// sinkSignature reports whether fn takes SQL text and returns its signature.
// Outside the drivers only the handle interface of the layer (sqlc's DBTX) is a
// sink; a store method named Get or Select is not.
func sinkSignature(fn *types.Func) (*types.Signature, bool) {
	if fn.Pkg() == nil || !sinkNames[fn.Name()] {
		return nil, false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return nil, false
	}
	if isDriver(fn.Pkg().Path()) {
		return sig, true
	}
	if !containsAny(fn.Pkg().Path(), dataAccessPackages) || sig.Recv() == nil || !types.IsInterface(sig.Recv().Type()) || !handleMethods[fn.Name()] {
		return nil, false
	}
	return sig, true
}

// reportSinkValues reports a sink method that is not called where it is named:
// a method value or expression (f := db.QueryContext) would let the statement
// reach the driver through a call the analyzer cannot follow.
func reportSinkValues(pass *analysis.Pass, insp *inspector.Inspector) {
	called := map[*ast.Ident]bool{}
	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		switch fun := ast.Unparen(n.(*ast.CallExpr).Fun).(type) { //nolint:errcheck,forcetypeassert // filtered by the node types above
		case *ast.Ident:
			called[fun] = true
		case *ast.SelectorExpr:
			called[fun.Sel] = true
		}
	})
	insp.Preorder([]ast.Node{(*ast.Ident)(nil)}, func(n ast.Node) {
		id := n.(*ast.Ident) //nolint:errcheck,forcetypeassert // filtered by the node types above
		fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
		if !ok || called[id] {
			return
		}
		if _, sink := sinkSignature(fn); sink {
			pass.Reportf(id.Pos(), "SQL method %s used as a value; call it directly with a constant statement so the text can be checked", fn.Name())
		}
	})
}

type lineKey struct {
	file string
	line int
}

// allowlistPath is where the files allowed to carry the directive are listed,
// relative to the repository root. Format of a line, fields separated by two or
// more spaces (like exceptions.txt):
//
//	<path of the Go file from the repository root>  <reason>  <YYYY-MM-DD>  approved-by:<owner>
//
// A malformed line grants nothing.
const allowlistPath = "internal/adapter/storetest/dbrules/policy/dynamic_sql_allowlist.txt"

var allowlistLine = regexp.MustCompile(`^(\S+) {2,}(\S.*?) {2,}(\d{4}-\d{2}-\d{2}) {2,}approved-by:(\S+)$`)

// allowlistFlag, when set (-orgscope.allowlist=<file>), names the allowlist
// explicitly; its entries are then relative to the repository root that the
// file path implies (the file path without allowlistPath). The tests use it;
// the vet run leaves it empty and the root is found from the source file.
var allowlistFlag string

func newAnalyzer() *analysis.Analyzer {
	a := &analysis.Analyzer{
		Name:     "orgscope",
		Doc:      "reports SQL driver use outside the data-access layer and SQL text that is not a constant",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      run,
	}
	a.Flags.StringVar(&allowlistFlag, "allowlist", "", "path of dynamic_sql_allowlist.txt (default: the one at the repository root)")
	return a
}

// repoRoot finds the root of the repository that holds dir: the outermost
// ancestor that has a .git entry, else the outermost one with a go.work. It is
// the outermost, never the nearest, so a copy of the allowlist planted in a
// subdirectory cannot become the authority. It returns "" when neither exists.
func repoRoot(dir string) string {
	for _, marker := range []string{".git", "go.work"} {
		root := ""
		for d := dir; ; d = filepath.Dir(d) {
			if _, err := os.Stat(filepath.Join(d, marker)); err == nil {
				root = d
			}
			if filepath.Dir(d) == d {
				break
			}
		}
		if root != "" {
			return root
		}
	}
	return ""
}

// allowedFiles returns the files, as absolute slash paths, that may carry the
// directive, read from the allowlist at the repository root of dir (or from the
// explicit flag). When there is none, no file may.
func allowedFiles(dir string) map[string]bool {
	var root, file string
	if allowlistFlag != "" {
		file = allowlistFlag
		root = strings.TrimSuffix(filepath.ToSlash(file), "/"+allowlistPath)
		root = filepath.FromSlash(root)
	} else if root = repoRoot(dir); root != "" {
		file = filepath.Join(root, filepath.FromSlash(allowlistPath))
	}
	out := map[string]bool{}
	if file == "" {
		return out
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := allowlistLine.FindStringSubmatch(line); m != nil {
			out[filepath.ToSlash(filepath.Join(root, filepath.FromSlash(m[1])))] = true
		}
	}
	return out
}

// directiveLines maps the lines holding the dynamic-sql directive to its reason.
// The directive counts only in a file of the allowlist; anywhere else the
// annotation is itself reported.
func directiveLines(pass *analysis.Pass) map[lineKey]string {
	out := map[lineKey]string{}
	for _, f := range pass.Files {
		var allowedHere map[string]bool
		for _, group := range f.Comments {
			for _, c := range group.List {
				rest, ok := strings.CutPrefix(c.Text, directive)
				if !ok || (rest != "" && rest[0] != ' ' && rest[0] != '\t') {
					continue
				}
				pos := pass.Fset.Position(c.Slash)
				abs, err := filepath.Abs(pos.Filename)
				if err != nil {
					abs = pos.Filename
				}
				if allowedHere == nil {
					allowedHere = allowedFiles(filepath.Dir(abs))
				}
				if !allowedHere[filepath.ToSlash(abs)] {
					pass.Reportf(c.Slash, "%s is honoured only in the files of %s; this file is not listed (owner approval is needed to add it)", directive, allowlistPath)
					continue
				}
				out[lineKey{pos.Filename, pos.Line}] = strings.TrimSpace(rest)
			}
		}
	}
	return out
}

// directiveFor finds the directive on the line of the call or the line above.
func directiveFor(pass *analysis.Pass, call *ast.CallExpr, allowed map[lineKey]string) (string, bool) {
	pos := pass.Fset.Position(call.Pos())
	for _, line := range []int{pos.Line, pos.Line - 1} {
		if reason, ok := allowed[lineKey{pos.Filename, line}]; ok {
			return reason, true
		}
	}
	return "", false
}

// containsAny reports whether path holds one of the element runs.
func containsAny(path string, runs []string) bool {
	for _, s := range runs {
		if path == s || strings.HasPrefix(path, s+"/") || strings.HasSuffix(path, "/"+s) || strings.Contains(path, "/"+s+"/") {
			return true
		}
	}
	return false
}

func isDriver(path string) bool {
	for _, p := range driverPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
