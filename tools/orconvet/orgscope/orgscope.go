// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package orgscope is wall 2(a) of organization isolation (ADR-0046 section 3,
// SPEC-004 R-ISO-02). It reports two things:
//
//   - a call into a SQL driver (database/sql, pgx, the SQLite driver) from a
//     package outside the data-access layer, which is internal/adapter/sqlite
//     and internal/adapter/postgres plus the declared exceptions
//     internal/adapter/storetest (the contract suites) and
//     internal/adapter/connectors (SQL against the user's own systems, never
//     Orcon's store);
//   - query text that is built at run time instead of being a constant, in any
//     package except those two exceptions (the contract suites build EXPLAIN,
//     PRAGMA and CREATE DATABASE text, and a connector runs the user's SQL): a string concatenation, fmt.Sprintf and friends, a
//     strings.Builder or bytes.Buffer, or a variable that holds one of these,
//     passed as the query of a driver call or of a method declared by the
//     data-access layer (the sqlc DBTX interface).
//
// Queries are parameterized only (orcon-db TI-8). The analyzer is deliberately
// conservative about what it flags: a query that arrives as a plain parameter,
// a field or a function result is not reported, because nothing in the type
// information says it was built unsafely; the constant text of generated code
// is the norm.
//
// The part of wall 2 that reads the statements themselves (every statement of
// scope org filters by org_id) is tools/sqlcheck. The check that a store method
// receives an orgscope.Org arrives with the type itself (SPEC-004 SE1b).
package orgscope

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

// Analyzer is the orgscope analyzer.
var Analyzer = &analysis.Analyzer{
	Name:     "orgscope",
	Doc:      "reports SQL driver calls outside the data-access layer and query text built by concatenation",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// driverPrefixes are the import path prefixes of SQL drivers and of the
// standard database package.
var driverPrefixes = []string{
	"database/sql",
	"github.com/jackc/pgx",
	"github.com/jackc/pgconn",
	"modernc.org/sqlite",
}

// dataAccessPackages are the packages that hold Orcon's SQL; a package whose
// import path contains one of them as a run of path elements is part of it.
var dataAccessPackages = []string{
	"internal/adapter/sqlite",
	"internal/adapter/postgres",
}

// exemptPackages may call a driver without being the data-access layer.
var exemptPackages = []string{
	"internal/adapter/storetest",
	"internal/adapter/connectors",
}

// sinkNames are the methods and functions whose first string parameter is SQL text.
var sinkNames = map[string]bool{
	"Exec": true, "ExecContext": true,
	"Query": true, "QueryContext": true,
	"QueryRow": true, "QueryRowContext": true,
	"Prepare": true, "PrepareContext": true,
	"Queue": true,
}

func run(pass *analysis.Pass) (any, error) {
	pkgPath := strings.TrimSuffix(pass.Pkg.Path(), "_test")
	exempt := containsAny(pkgPath, exemptPackages)
	inLayer := exempt || containsAny(pkgPath, dataAccessPackages)
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint:errcheck,forcetypeassert // the Requires clause guarantees the type
	flow := collectAssignments(pass, insp)

	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		call := n.(*ast.CallExpr) //nolint:errcheck,forcetypeassert // filtered by the node types above
		fn, ok := typeutil.Callee(pass.TypesInfo, call).(*types.Func)
		if !ok || fn.Pkg() == nil {
			return
		}
		fromDriver := isDriver(fn.Pkg().Path())
		if fromDriver && !inLayer {
			pass.Reportf(call.Pos(), "SQL access (%s.%s) outside the data-access layer; SQL lives only in internal/adapter/sqlite and internal/adapter/postgres", fn.Pkg().Name(), fn.Name())
		}
		if exempt || !sinkNames[fn.Name()] || (!fromDriver && !containsAny(fn.Pkg().Path(), dataAccessPackages)) {
			return
		}
		if arg := queryArg(fn, call); arg != nil && flow.dynamic(arg, 0) {
			pass.Reportf(arg.Pos(), "SQL text is built at run time; write the statement as a constant and pass values as parameters (orcon-db TI-8)")
		}
	})
	return nil, nil //nolint:nilnil // analyzers without a result return nil
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

// queryArg returns the argument bound to the first string parameter of fn.
func queryArg(fn *types.Func, call *ast.CallExpr) ast.Expr {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return nil
	}
	params := sig.Params()
	for i := range params.Len() {
		basic, isBasic := params.At(i).Type().Underlying().(*types.Basic)
		if !isBasic || basic.Kind() != types.String {
			continue
		}
		if i < len(call.Args) {
			return call.Args[i]
		}
		return nil
	}
	return nil
}

// assignments records, per variable, the expressions assigned to it and
// whether it was ever extended with +=.
type assignments struct {
	pass     *analysis.Pass
	values   map[types.Object][]ast.Expr
	extended map[types.Object]bool
}

func collectAssignments(pass *analysis.Pass, insp *inspector.Inspector) *assignments {
	a := &assignments{pass: pass, values: map[types.Object][]ast.Expr{}, extended: map[types.Object]bool{}}
	insp.Preorder([]ast.Node{(*ast.AssignStmt)(nil), (*ast.ValueSpec)(nil)}, func(n ast.Node) {
		switch s := n.(type) {
		case *ast.AssignStmt:
			a.recordAssign(s)
		case *ast.ValueSpec:
			if len(s.Names) != len(s.Values) {
				return
			}
			for i, id := range s.Names {
				if obj := pass.TypesInfo.ObjectOf(id); obj != nil {
					a.values[obj] = append(a.values[obj], s.Values[i])
				}
			}
		}
	})
	return a
}

func (a *assignments) recordAssign(s *ast.AssignStmt) {
	for i, lhs := range s.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		obj := a.pass.TypesInfo.ObjectOf(id)
		if obj == nil {
			continue
		}
		if s.Tok == token.ADD_ASSIGN {
			a.extended[obj] = true
			continue
		}
		if len(s.Lhs) == len(s.Rhs) {
			a.values[obj] = append(a.values[obj], s.Rhs[i])
		}
	}
}

// maxDepth bounds the chase through variables.
const maxDepth = 8

// dynamic reports whether expr is query text assembled at run time.
func (a *assignments) dynamic(expr ast.Expr, depth int) bool {
	if depth > maxDepth {
		return false
	}
	if tv, ok := a.pass.TypesInfo.Types[expr]; ok && tv.Value != nil {
		return false // a constant, however it is spelled
	}
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return a.dynamic(e.X, depth+1)
	case *ast.BinaryExpr:
		return e.Op == token.ADD
	case *ast.CallExpr:
		return a.dynamicCall(e)
	case *ast.Ident:
		obj := a.pass.TypesInfo.ObjectOf(e)
		if obj == nil {
			return false
		}
		if a.extended[obj] {
			return true
		}
		for _, v := range a.values[obj] {
			if a.dynamic(v, depth+1) {
				return true
			}
		}
	}
	return false
}

// dynamicCall reports whether call builds a string: fmt.Sprint*, strings
// functions that compose text, or the String method of a builder or buffer.
func (a *assignments) dynamicCall(call *ast.CallExpr) bool {
	fn, ok := typeutil.Callee(a.pass.TypesInfo, call).(*types.Func)
	if !ok || fn.Pkg() == nil {
		return false
	}
	switch fn.Pkg().Path() {
	case "fmt":
		return strings.HasPrefix(fn.Name(), "Sprint")
	case "strings":
		switch fn.Name() {
		case "Join", "Replace", "ReplaceAll", "Repeat", "String":
			return true
		}
	case "bytes":
		return fn.Name() == "String"
	}
	return false
}
