// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package nosecretlog reports sensitive values that flow into log records or
// error messages: arguments of log/slog calls, of (*slog.Logger) methods and
// of fmt.Errorf whose source is a struct field tagged `orcon:"sensitive"` or
// an identifier whose name looks like a secret, unless the value has a type
// from internal/platform/redact (whose String, LogValue and MarshalJSON
// return [REDACTED]).
package nosecretlog

import (
	"go/ast"
	"go/types"
	"reflect"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the nosecretlog analyzer.
var Analyzer = &analysis.Analyzer{
	Name:     "nosecretlog",
	Doc:      "reports secrets passed to slog calls or fmt.Errorf; wrap them in an internal/platform/redact type",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// secretName matches identifiers that name secrets.
var secretName = regexp.MustCompile(`(?i)(password|passwd|secret|token|api_?key|authorization|cookie|private_?key|credential|session_?key)`)

// redactPkgSuffix identifies the redact package by import path suffix so the
// analyzer works for any module path and for analysistest fixtures.
const redactPkgSuffix = "internal/platform/redact"

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint:errcheck,forcetypeassert // the Requires clause guarantees the type
	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		call := n.(*ast.CallExpr) //nolint:errcheck,forcetypeassert // filtered by the node types above
		name, ok := loggingCall(pass, call)
		if !ok {
			return
		}
		for _, arg := range call.Args {
			checkArg(pass, arg, name)
		}
	})
	return nil, nil //nolint:nilnil // analyzers without a result return nil
}

// loggingCall reports whether call is a log/slog function, a method of
// *slog.Logger, or fmt.Errorf, returning a printable name for messages.
func loggingCall(pass *analysis.Pass, call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	obj := pass.TypesInfo.Uses[sel.Sel]
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil {
		return "", false
	}
	switch fn.Pkg().Path() {
	case "log/slog":
		return "slog." + fn.Name(), true
	case "fmt":
		if fn.Name() == "Errorf" {
			return "fmt.Errorf", true
		}
	}
	return "", false
}

// checkArg walks one argument expression and reports sensitive sources.
func checkArg(pass *analysis.Pass, expr ast.Expr, callName string) {
	if isRedacted(pass.TypesInfo.TypeOf(expr)) {
		return
	}
	switch e := expr.(type) {
	case *ast.Ident:
		if v, ok := pass.TypesInfo.Uses[e].(*types.Var); ok && secretName.MatchString(v.Name()) {
			report(pass, e, e.Name, callName)
		}
	case *ast.SelectorExpr:
		checkSelector(pass, e, callName)
	case *ast.CallExpr:
		// A nested slog call (slog.String("k", v)) is analyzed on its own by
		// the inspector; other calls (conversions, helpers) are inspected here
		// through their arguments.
		if _, isLog := loggingCall(pass, e); isLog {
			return
		}
		for _, a := range e.Args {
			checkArg(pass, a, callName)
		}
	case *ast.ParenExpr:
		checkArg(pass, e.X, callName)
	case *ast.StarExpr:
		checkArg(pass, e.X, callName)
	case *ast.UnaryExpr:
		checkArg(pass, e.X, callName)
	case *ast.BinaryExpr:
		checkArg(pass, e.X, callName)
		checkArg(pass, e.Y, callName)
	case *ast.IndexExpr:
		checkArg(pass, e.X, callName)
	}
}

func checkSelector(pass *analysis.Pass, sel *ast.SelectorExpr, callName string) {
	selection := pass.TypesInfo.Selections[sel]
	switch {
	case selection != nil && selection.Kind() == types.FieldVal:
		field, _ := selection.Obj().(*types.Var)
		if field != nil && (hasSensitiveTag(structTag(selection.Recv(), field)) || secretName.MatchString(field.Name())) {
			report(pass, sel, exprString(sel), callName)
			return
		}
		checkArg(pass, sel.X, callName)
	case selection == nil:
		// Package-qualified variable, e.g. config.Token.
		if v, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Var); ok && secretName.MatchString(v.Name()) {
			report(pass, sel, exprString(sel), callName)
		}
	}
}

// structTag finds the tag of field within the struct underlying recv.
func structTag(recv types.Type, field *types.Var) string {
	for {
		ptr, ok := recv.(*types.Pointer)
		if !ok {
			break
		}
		recv = ptr.Elem()
	}
	st, ok := recv.Underlying().(*types.Struct)
	if !ok {
		return findPromotedTag(recv, field)
	}
	for i := range st.NumFields() {
		if st.Field(i) == field {
			return st.Tag(i)
		}
	}
	return findPromotedTag(recv, field)
}

// findPromotedTag looks for field in embedded structs.
func findPromotedTag(recv types.Type, field *types.Var) string {
	st, ok := recv.Underlying().(*types.Struct)
	if !ok {
		return ""
	}
	for i := range st.NumFields() {
		if st.Field(i) == field {
			return st.Tag(i)
		}
		if st.Field(i).Embedded() {
			if tag := structTag(st.Field(i).Type(), field); tag != "" {
				return tag
			}
		}
	}
	return ""
}

func hasSensitiveTag(tag string) bool {
	for _, part := range strings.Split(reflect.StructTag(tag).Get("orcon"), ",") {
		if strings.TrimSpace(part) == "sensitive" {
			return true
		}
	}
	return false
}

// isRedacted reports whether t is a named type declared in the redact package.
func isRedacted(t types.Type) bool {
	if t == nil {
		return false
	}
	for {
		ptr, ok := t.(*types.Pointer)
		if !ok {
			break
		}
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return strings.HasSuffix(named.Obj().Pkg().Path(), redactPkgSuffix)
}

func report(pass *analysis.Pass, at ast.Node, what, callName string) {
	pass.Reportf(at.Pos(), "sensitive value %s passed to %s; wrap it in an internal/platform/redact type or drop it", what, callName)
}

func exprString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprString(x.X) + "." + x.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(x.X)
	case *ast.ParenExpr:
		return exprString(x.X)
	default:
		return "value"
	}
}
