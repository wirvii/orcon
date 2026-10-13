// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import (
	"slices"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/parser"
)

// FindingKind classifies what [Analyze] could not vouch for.
type FindingKind string

// Kinds of findings.
const (
	// FindingSyntax means the code does not parse.
	FindingSyntax FindingKind = "SYNTAX"
	// FindingUnknownIdentifier means the code uses a free identifier the
	// runtime does not define.
	FindingUnknownIdentifier FindingKind = "UNKNOWN_IDENTIFIER"
	// FindingUnknownMethod means the code calls a method name that neither
	// JavaScript, Luxon nor the n8n-compatible extension methods define.
	FindingUnknownMethod FindingKind = "UNKNOWN_METHOD"
	// FindingDynamicReference means a node reference whose name is computed, so
	// the analysis cannot tell which node it reads.
	FindingDynamicReference FindingKind = "DYNAMIC_NODE_REFERENCE"
	// FindingExecutionVariables means $execution.variables is used other than
	// with a literal name: a computed key, the object as a whole, enumeration
	// or destructuring (ADR-0018 section 1, amendment 10). Publication rejects
	// it (rule V8).
	FindingExecutionVariables FindingKind = "EXECUTION_VARIABLES"
)

// Finding is something about an expression that a reader of the import report
// or a publication rule needs to know (ADR-0018 section 7). When the analysis
// cannot decide, it reports instead of guessing.
type Finding struct {
	Kind FindingKind
	// Name is the identifier or method concerned, empty for syntax findings.
	Name string
	// Offset is the byte position in the analyzed text.
	Offset  int
	Message string
}

// NodeRef is a read of another node's output by name: $('Name'),
// $node["Name"] or $items("Name").
type NodeRef struct {
	Name string
	// Guarded is true when the reference is followed by .isExecuted, the
	// explicit test that the node ran (ADR-0008 section 4.2).
	Guarded bool
	Offset  int
}

// Analysis is the outcome of parsing an expression without running it.
type Analysis struct {
	// Identifiers are the free identifiers of the code, sorted, without
	// repeats: the names it reads from the context or the global scope.
	Identifiers []string
	// NodeRefs are the references to other nodes by literal name.
	NodeRefs []NodeRef
	// Methods are the names of the methods the code calls, sorted, without
	// repeats.
	Methods []string
	// Findings lists everything the analysis could not vouch for.
	Findings []Finding
	// Size is the number of expression nodes in the syntax tree, a measure of
	// the complexity of the code.
	Size int
	// JSONPaths are the property paths of $json the code reads through static
	// member chains: $json.a.b is the path [a b]. When JSONWhole is false the
	// code touches nothing of $json outside them.
	JSONPaths [][]string
	// JSONWhole is set when the code uses $json as a whole (passes it on, lists
	// its keys, reads it by a computed key) or cannot be analyzed, so all of it
	// is needed.
	JSONWhole bool
	// Dynamic is set when the code can build names at run time (eval, the
	// Function constructor, arguments, with), so any context name may be read.
	Dynamic bool
	// Variables are the execution variables the code reads by literal name,
	// $execution.variables.name or $execution.variables["name"], sorted,
	// without repeats. Any other use of $execution.variables is a finding of
	// kind [FindingExecutionVariables].
	Variables []string
}

// Syntax reports whether the code failed to parse.
func (a Analysis) Syntax() bool {
	return slices.ContainsFunc(a.Findings, func(f Finding) bool { return f.Kind == FindingSyntax })
}

// Uses reports whether the code reads the free identifier name.
func (a Analysis) Uses(name string) bool { return slices.Contains(a.Identifiers, name) }

// Supported reports whether the analysis found nothing it could not vouch for.
func (a Analysis) Supported() bool { return len(a.Findings) == 0 }

// Analyze parses the JavaScript of one expression and reports the names it
// uses. It never evaluates anything.
func Analyze(code string) Analysis {
	const prefix = "(\n"
	code = trimCode(code)
	prog, err := parser.ParseFile(nil, "", prefix+code+"\n)", 0)
	if err != nil {
		return Analysis{
			Findings:  []Finding{{Kind: FindingSyntax, Message: firstLine(err.Error())}},
			JSONWhole: true, Dynamic: true,
		}
	}
	w := &walker{base: len(prefix)}
	w.push(nil)
	for _, st := range prog.Body {
		w.stmt(st)
	}
	var a Analysis
	a.Identifiers = slices.Sorted(maps(w.free))
	a.NodeRefs = w.refs
	a.Methods = slices.Sorted(maps(w.methods))
	a.Findings = w.findings
	a.Size = w.size
	a.JSONWhole = w.jsonWhole
	a.JSONPaths = dedupePaths(w.jsonPaths)
	a.Variables = slices.Sorted(maps(w.variables))
	for _, name := range []string{"eval", "Function", "arguments"} {
		if _, ok := w.free[name]; ok {
			a.Dynamic, a.JSONWhole = true, true
		}
	}
	if w.dynamic {
		a.Dynamic, a.JSONWhole = true, true
	}
	for _, id := range a.Identifiers {
		if !knownName(id) {
			a.Findings = append(a.Findings, Finding{
				Kind: FindingUnknownIdentifier, Name: id, Offset: w.free[id],
				Message: "the identifier " + id + " is not defined by the expression runtime",
			})
		}
	}
	for _, m := range a.Methods {
		if !knownMethod(m) {
			a.Findings = append(a.Findings, Finding{
				Kind: FindingUnknownMethod, Name: m, Offset: w.methods[m],
				Message: "the method " + m + "() is not defined by JavaScript, Luxon or the supported extension methods",
			})
		}
	}
	slices.SortFunc(a.Findings, func(x, y Finding) int {
		if x.Offset != y.Offset {
			return x.Offset - y.Offset
		}
		return strings.Compare(x.Name, y.Name)
	})
	return a
}

// AnalyzeTemplate analyzes every code segment of a template string (a value
// that starts with "="), merging the results. Offsets are relative to the
// body of the template. A string that is not a template has no code.
func AnalyzeTemplate(s string) Analysis {
	body, ok := strings.CutPrefix(s, "=")
	if !ok {
		return Analysis{}
	}
	var out Analysis
	ids, methods, variables := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for _, seg := range Segments(body) {
		if !seg.Code {
			continue
		}
		a := Analyze(seg.Text)
		out.Size += a.Size
		out.JSONWhole = out.JSONWhole || a.JSONWhole
		out.Dynamic = out.Dynamic || a.Dynamic
		out.JSONPaths = append(out.JSONPaths, a.JSONPaths...)
		for _, id := range a.Identifiers {
			ids[id] = struct{}{}
		}
		for _, m := range a.Methods {
			methods[m] = struct{}{}
		}
		for _, name := range a.Variables {
			variables[name] = struct{}{}
		}
		for _, r := range a.NodeRefs {
			r.Offset += seg.Offset
			out.NodeRefs = append(out.NodeRefs, r)
		}
		for _, f := range a.Findings {
			f.Offset += seg.Offset
			out.Findings = append(out.Findings, f)
		}
	}
	out.Identifiers = slices.Sorted(maps(ids))
	out.Methods = slices.Sorted(maps(methods))
	out.Variables = slices.Sorted(maps(variables))
	out.JSONPaths = dedupePaths(out.JSONPaths)
	return out
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// maps returns the keys of a set-like map as a sequence.
func maps[V any](m map[string]V) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// walker collects the facts of an Analysis in one pass over the syntax tree.
type walker struct {
	base     int
	scopes   []map[string]struct{}
	free     map[string]int // identifier -> offset of its first use
	methods  map[string]int
	refs     []NodeRef
	findings []Finding
	size     int
	renames  []rename

	jsonPaths [][]string
	jsonWhole bool
	dynamic   bool
	variables map[string]struct{}
}

// rename is a method name to replace in the source text.
type rename struct {
	at       int
	from, to string
}

func (w *walker) offset(idx file.Idx) int { return int(idx) - 1 - w.base }

func (w *walker) push(names map[string]struct{}) {
	if w.free == nil {
		w.free, w.methods = map[string]int{}, map[string]int{}
		w.variables = map[string]struct{}{}
	}
	w.scopes = append(w.scopes, names)
}

func (w *walker) pop() { w.scopes = w.scopes[:len(w.scopes)-1] }

func (w *walker) bound(name string) bool {
	for _, s := range w.scopes {
		if _, ok := s[name]; ok {
			return true
		}
	}
	return false
}

func (w *walker) use(name string, idx file.Idx) {
	if w.bound(name) {
		return
	}
	if name == "$json" {
		w.jsonWhole = true // a bare $json is the whole item
	}
	if _, seen := w.free[name]; !seen {
		w.free[name] = w.offset(idx)
	}
}

// useRoot records the use of $json as the base of a member chain, which reads
// only the path of the chain (see [walker.jsonMember]).
func (w *walker) useRoot(idx file.Idx) {
	if _, seen := w.free["$json"]; !seen {
		w.free["$json"] = w.offset(idx)
	}
}

// jsonMember handles a member chain whose base is the free identifier $json:
// it records the longest static prefix of the chain as a path read, and walks
// the computed keys, if any. It reports whether e was such a chain.
func (w *walker) jsonMember(e ast.Expression) bool {
	var links []ast.Expression // outermost first
	cur := e
	for {
		switch n := cur.(type) {
		case *ast.DotExpression:
			links = append(links, n)
			cur = n.Left
			continue
		case *ast.BracketExpression:
			links = append(links, n)
			cur = n.Left
			continue
		}
		break
	}
	id, ok := cur.(*ast.Identifier)
	if !ok || id.Name.String() != "$json" || w.bound("$json") {
		return false
	}
	w.useRoot(id.Idx)
	slices.Reverse(links)
	path := []string{}
	static := true
	for _, l := range links {
		w.size++
		switch n := l.(type) {
		case *ast.DotExpression:
			if static {
				path = append(path, n.Identifier.Name.String())
			}
		case *ast.BracketExpression:
			key, lit := stringLiteral(n.Member)
			if _, index := canonicalIndex(key); static && lit && !index {
				path = append(path, key)
			} else {
				static = false
				w.expr(n.Member)
			}
		}
	}
	w.jsonPaths = append(w.jsonPaths, path)
	return true
}

// unwrapOptional strips the optional-chaining wrappers of a node.
func unwrapOptional(e ast.Expression) ast.Expression { //nolint:ireturn // returns the AST node it was given, whatever its type
	for {
		switch n := e.(type) {
		case *ast.Optional:
			e = n.Expression
		case *ast.OptionalChain:
			e = n.Expression
		default:
			return e
		}
	}
}

// isExecution reports whether e is the free identifier $execution.
func (w *walker) isExecution(e ast.Expression) bool {
	id, ok := unwrapOptional(e).(*ast.Identifier)
	return ok && string(id.Name) == "$execution" && !w.bound("$execution")
}

// isExecVariables reports whether e is $execution.variables, written with a
// dot or with a literal bracket key.
func (w *walker) isExecVariables(e ast.Expression) bool {
	switch n := unwrapOptional(e).(type) {
	case *ast.DotExpression:
		return n.Identifier.Name == "variables" && w.isExecution(n.Left)
	case *ast.BracketExpression:
		key, lit := stringLiteral(n.Member)
		return lit && key == "variables" && w.isExecution(n.Left)
	}
	return false
}

// variableAccess handles $execution.variables.name and
// $execution.variables["name"]: it records the variable read, or a finding
// when the key is computed. It reports whether e was such an access.
func (w *walker) variableAccess(e ast.Expression) bool {
	var left, member ast.Expression
	var name string
	lit := true
	switch n := e.(type) {
	case *ast.DotExpression:
		left, name = n.Left, n.Identifier.Name.String()
	case *ast.BracketExpression:
		left, member = n.Left, n.Member
		name, lit = stringLiteral(n.Member)
	default:
		return false
	}
	if !w.isExecVariables(left) {
		return false
	}
	w.use("$execution", e.Idx0())
	w.size++
	if lit {
		w.variables[name] = struct{}{}
		return true
	}
	w.findings = append(w.findings, Finding{
		Kind: FindingExecutionVariables, Name: "$execution.variables", Offset: w.offset(e.Idx0()),
		Message: "$execution.variables is read with a computed key; use a literal name, such as $execution.variables.total",
	})
	w.expr(member)
	return true
}

// dedupePaths sorts the paths and drops repeats and the paths that lie inside
// another one.
func dedupePaths(paths [][]string) [][]string {
	slices.SortFunc(paths, slices.Compare[[]string])
	var out [][]string
	for _, p := range paths {
		if len(out) > 0 && hasPrefix(p, out[len(out)-1]) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func hasPrefix(p, prefix []string) bool {
	return len(p) >= len(prefix) && slices.Equal(p[:len(prefix)], prefix)
}

func (w *walker) method(name string, idx file.Idx) {
	if _, seen := w.methods[name]; !seen {
		w.methods[name] = w.offset(idx)
	}
}

// stringLiteral returns the value of a string literal expression.
func stringLiteral(e ast.Expression) (string, bool) {
	if s, ok := e.(*ast.StringLiteral); ok {
		return s.Value.String(), true
	}
	return "", false
}

func (w *walker) call(n *ast.CallExpression) {
	switch callee := n.Callee.(type) {
	case *ast.Identifier:
		if string(callee.Name) == "$" || string(callee.Name) == "$items" {
			w.nodeCall(n, string(callee.Name))
		}
	case *ast.DotExpression:
		name := callee.Identifier.Name.String()
		w.method(name, callee.Identifier.Idx)
		if alias, ok := callAliases[name]; ok {
			w.renames = append(w.renames, rename{at: w.offset(callee.Identifier.Idx), from: name, to: alias})
		}
	case *ast.BracketExpression:
		if name, ok := stringLiteral(callee.Member); ok {
			w.method(name, callee.Member.Idx0())
		}
	}
	switch callee := n.Callee.(type) {
	case *ast.DotExpression:
		w.size++
		w.expr(callee.Left) // the method name is not data
	case *ast.BracketExpression:
		w.size++
		w.expr(callee.Left)
		w.expr(callee.Member)
	default:
		w.expr(n.Callee)
	}
	for _, a := range n.ArgumentList {
		w.expr(a)
	}
}

// nodeCall records $('Name') and $items('Name') references.
func (w *walker) nodeCall(n *ast.CallExpression, fn string) {
	if w.bound(fn) {
		return
	}
	if len(n.ArgumentList) == 0 {
		return // $items() reads the input
	}
	name, ok := stringLiteral(n.ArgumentList[0])
	if !ok {
		w.findings = append(w.findings, Finding{
			Kind: FindingDynamicReference, Name: fn, Offset: w.offset(n.Idx0()),
			Message: "the node read by " + fn + "(...) is computed, so which node it reads cannot be determined",
		})
		return
	}
	w.refs = append(w.refs, NodeRef{Name: name, Offset: w.offset(n.Idx0())})
}

// markGuarded flags the most recent reference when the expression being
// walked is <ref>.isExecuted.
func (w *walker) markGuarded(left ast.Expression) {
	var name string
	switch l := left.(type) {
	case *ast.CallExpression:
		id, ok := l.Callee.(*ast.Identifier)
		if !ok || string(id.Name) != "$" || len(l.ArgumentList) == 0 {
			return
		}
		name, _ = stringLiteral(l.ArgumentList[0])
	case *ast.BracketExpression:
		id, ok := l.Left.(*ast.Identifier)
		if !ok || string(id.Name) != "$node" {
			return
		}
		name, _ = stringLiteral(l.Member)
	default:
		return
	}
	for i := len(w.refs) - 1; i >= 0; i-- {
		if w.refs[i].Name == name {
			w.refs[i].Guarded = true
			return
		}
	}
}

// contextMember handles the member chains that read a part of the context the
// analysis tracks: an execution variable by name, $execution.variables as a
// whole, or a path of $json. It reports whether e was one.
func (w *walker) contextMember(e ast.Expression) bool {
	if w.variableAccess(e) {
		return true
	}
	if w.isExecVariables(e) {
		w.wholeVariables(e)
		return true
	}
	if w.executionMember(e) {
		return true
	}
	if w.globalMember(e) {
		return true
	}
	return w.jsonMember(e)
}

// isGlobalBase reports whether e is globalThis or this, the two ways to reach
// the global object by member access.
func (w *walker) isGlobalBase(e ast.Expression) bool {
	switch n := unwrapOptional(e).(type) {
	case *ast.Identifier:
		return string(n.Name) == "globalThis" && !w.bound("globalThis")
	case *ast.ThisExpression:
		return true
	}
	return false
}

// globalMember handles a member access on globalThis or on this: $execution
// reached through them (by name or by a computed key) would go around the
// checks of $execution, so it is a finding. A literal key that is not
// $execution is an ordinary read.
func (w *walker) globalMember(e ast.Expression) bool {
	var left, member ast.Expression
	var name string
	lit := true
	switch n := e.(type) {
	case *ast.DotExpression:
		left, name = n.Left, n.Identifier.Name.String()
	case *ast.BracketExpression:
		left, member = n.Left, n.Member
		name, lit = stringLiteral(n.Member)
	default:
		return false
	}
	if !w.isGlobalBase(left) {
		return false
	}
	if id, ok := unwrapOptional(left).(*ast.Identifier); ok {
		w.use(string(id.Name), id.Idx)
	}
	w.size++
	if lit && name != "$execution" {
		return true
	}
	w.findings = append(w.findings, Finding{
		Kind: FindingExecutionVariables, Name: "$execution", Offset: w.offset(e.Idx0()),
		Message: "the global object is read with a computed key or for $execution, which could reach $execution around its checks; use $execution directly",
	})
	if !lit {
		w.expr(member)
	}
	return true
}

// executionFields are the members of $execution an expression may read besides
// its variables.
var executionFields = []string{"id", "mode", "resumeUrl", "resumeFormUrl"}

// executionMember handles a member access whose base is $execution itself
// (not its variables): the four plain fields are allowed, anything else is a
// finding because it could reach the variables around the check.
func (w *walker) executionMember(e ast.Expression) bool {
	var left, member ast.Expression
	var name string
	lit := true
	switch n := e.(type) {
	case *ast.DotExpression:
		left, name = n.Left, n.Identifier.Name.String()
	case *ast.BracketExpression:
		left, member = n.Left, n.Member
		name, lit = stringLiteral(n.Member)
	default:
		return false
	}
	if !w.isExecution(left) {
		return false
	}
	w.use("$execution", e.Idx0())
	w.size++
	if lit && slices.Contains(executionFields, name) {
		return true
	}
	w.findings = append(w.findings, Finding{
		Kind: FindingExecutionVariables, Name: "$execution", Offset: w.offset(e.Idx0()),
		Message: "$execution is read other than by .id, .mode, .resumeUrl, .resumeFormUrl or a literal .variables.name; use one of those",
	})
	if !lit {
		w.expr(member)
	}
	return true
}

// wholeVariables records a use of $execution.variables as an object.
func (w *walker) wholeVariables(e ast.Expression) {
	w.use("$execution", e.Idx0())
	w.findings = append(w.findings, Finding{
		Kind: FindingExecutionVariables, Name: "$execution.variables", Offset: w.offset(e.Idx0()),
		Message: "$execution.variables is used as a whole object; read one declared variable by a literal name, such as $execution.variables.total",
	})
}

// identifier reports the identifiers that could reach $execution around the
// checks, and records the use of the name.
func (w *walker) identifier(n *ast.Identifier) {
	if name := string(n.Name); (name == "globalThis" || name == "eval" || name == "Function") && !w.bound(name) {
		// The global object as a value, eval and the Function constructor can
		// build any name at run time, $execution included.
		w.findings = append(w.findings, Finding{
			Kind: FindingExecutionVariables, Name: name, Offset: w.offset(n.Idx),
			Message: name + " can reach $execution around its checks and is not allowed in an expression",
		})
	}
	if string(n.Name) == "$execution" && !w.bound("$execution") {
		// $execution used as a value (aliased, spread, destructured,
		// enumerated or passed on) could reach the variables around the
		// literal-name check.
		w.findings = append(w.findings, Finding{
			Kind: FindingExecutionVariables, Name: "$execution", Offset: w.offset(n.Idx),
			Message: "$execution is used as a whole; read .id, .mode, .resumeUrl, .resumeFormUrl or a literal .variables.name",
		})
	}
	w.use(string(n.Name), n.Idx)
}

// nodeIndex records the node read by $node["X"], or a finding when the name
// is computed.
func (w *walker) nodeIndex(n *ast.BracketExpression) {
	id, ok := n.Left.(*ast.Identifier)
	if !ok || string(id.Name) != "$node" || w.bound("$node") {
		return
	}
	if name, lit := stringLiteral(n.Member); lit {
		w.refs = append(w.refs, NodeRef{Name: name, Offset: w.offset(n.Idx0())})
		return
	}
	w.findings = append(w.findings, Finding{
		Kind: FindingDynamicReference, Name: "$node", Offset: w.offset(n.Idx0()),
		Message: "the node read by $node[...] is computed, so which node it reads cannot be determined",
	})
}

func (w *walker) expr(e ast.Expression) {
	if e != nil {
		w.size++
	}
	switch n := e.(type) {
	case nil:
	case *ast.Identifier:
		w.identifier(n)
	case *ast.CallExpression:
		w.call(n)
	case *ast.DotExpression:
		if w.contextMember(n) {
			return
		}
		if n.Identifier.Name == "isExecuted" {
			defer w.markGuarded(n.Left)
		}
		w.expr(n.Left)
	case *ast.BracketExpression:
		if w.contextMember(n) {
			return
		}
		w.nodeIndex(n)
		w.expr(n.Left)
		w.expr(n.Member)
	case *ast.BinaryExpression:
		w.expr(n.Left)
		w.expr(n.Right)
	case *ast.AssignExpression:
		w.expr(n.Left)
		w.expr(n.Right)
	case *ast.UnaryExpression:
		w.expr(n.Operand)
	case *ast.ConditionalExpression:
		w.expr(n.Test)
		w.expr(n.Consequent)
		w.expr(n.Alternate)
	case *ast.SequenceExpression:
		for _, x := range n.Sequence {
			w.expr(x)
		}
	case *ast.ArrayLiteral:
		for _, x := range n.Value {
			w.expr(x)
		}
	case *ast.ObjectLiteral:
		for _, p := range n.Value {
			w.property(p)
		}
	case *ast.NewExpression:
		w.expr(n.Callee)
		for _, x := range n.ArgumentList {
			w.expr(x)
		}
	case *ast.TemplateLiteral:
		w.expr(n.Tag)
		for _, x := range n.Expressions {
			w.expr(x)
		}
	case *ast.OptionalChain:
		w.expr(n.Expression)
	case *ast.Optional:
		w.expr(n.Expression)
	case *ast.SpreadElement:
		w.expr(n.Expression)
	case *ast.AwaitExpression:
		w.expr(n.Argument)
	case *ast.YieldExpression:
		w.expr(n.Argument)
	case *ast.FunctionLiteral:
		w.function(n.ParameterList, n.Body, n.Name, nil)
	case *ast.ArrowFunctionLiteral:
		switch body := n.Body.(type) {
		case *ast.ExpressionBody:
			w.function(n.ParameterList, nil, nil, body.Expression)
		case *ast.BlockStatement:
			w.function(n.ParameterList, body, nil, nil)
		}
	case *ast.ClassLiteral:
		w.expr(n.SuperClass)
	case *ast.NumberLiteral, *ast.StringLiteral, *ast.BooleanLiteral, *ast.NullLiteral,
		*ast.RegExpLiteral, *ast.ThisExpression, *ast.BadExpression:
	}
}

func (w *walker) property(p ast.Property) {
	switch n := p.(type) {
	case *ast.PropertyShort:
		w.use(string(n.Name.Name), n.Name.Idx)
		w.expr(n.Initializer)
	case *ast.PropertyKeyed:
		if n.Computed {
			w.expr(n.Key)
		}
		w.expr(n.Value)
	case *ast.SpreadElement:
		w.expr(n.Expression)
	}
}

// function walks a function body with its own scope: parameters and every
// name declared anywhere in the body (hoisting is approximated by treating
// the whole body as one scope).
func (w *walker) function(params *ast.ParameterList, body *ast.BlockStatement, name *ast.Identifier, concise ast.Expression) {
	names := map[string]struct{}{}
	if name != nil {
		names[string(name.Name)] = struct{}{}
	}
	if params != nil {
		for _, b := range params.List {
			bindingNames(b.Target, names)
		}
		bindingNames(params.Rest, names)
	}
	if body != nil {
		declaredNames(body, names)
	}
	w.push(names)
	defer w.pop()
	if params != nil {
		for _, b := range params.List {
			w.expr(b.Initializer)
			w.patternDefaults(b.Target)
		}
	}
	if body != nil {
		w.stmt(body)
	}
	w.expr(concise)
}

// patternDefaults walks the default values and computed keys inside a
// destructuring pattern.
func (w *walker) patternDefaults(t ast.Expression) {
	switch n := t.(type) {
	case *ast.ObjectPattern:
		for _, p := range n.Properties {
			switch pp := p.(type) {
			case *ast.PropertyShort:
				w.expr(pp.Initializer)
			case *ast.PropertyKeyed:
				if pp.Computed {
					w.expr(pp.Key)
				}
				w.patternDefaults(pp.Value)
			}
		}
	case *ast.ArrayPattern:
		for _, el := range n.Elements {
			w.patternDefaults(el)
		}
	case *ast.AssignExpression:
		w.patternDefaults(n.Left)
		w.expr(n.Right)
	}
}

// bindingNames adds the names a binding target declares.
func bindingNames(t ast.Expression, into map[string]struct{}) {
	switch n := t.(type) {
	case *ast.Identifier:
		into[string(n.Name)] = struct{}{}
	case *ast.ObjectPattern:
		for _, p := range n.Properties {
			switch pp := p.(type) {
			case *ast.PropertyShort:
				into[string(pp.Name.Name)] = struct{}{}
			case *ast.PropertyKeyed:
				bindingNames(pp.Value, into)
			}
		}
		bindingNames(n.Rest, into)
	case *ast.ArrayPattern:
		for _, el := range n.Elements {
			bindingNames(el, into)
		}
		bindingNames(n.Rest, into)
	case *ast.AssignExpression:
		bindingNames(n.Left, into)
	}
}

// declaredNames adds every name declared by var, let, const, function, class,
// catch or for statements anywhere inside stmt, not entering nested functions.
func declaredNames(s ast.Statement, into map[string]struct{}) {
	switch n := s.(type) {
	case *ast.BlockStatement:
		if n == nil { // a try statement without a finally block
			return
		}
		for _, x := range n.List {
			declaredNames(x, into)
		}
	case *ast.VariableStatement:
		declaredBindings(n.List, into)
	case *ast.LexicalDeclaration:
		declaredBindings(n.List, into)
	case *ast.FunctionDeclaration:
		if n.Function != nil && n.Function.Name != nil {
			into[string(n.Function.Name.Name)] = struct{}{}
		}
	case *ast.ClassDeclaration:
		if n.Class != nil && n.Class.Name != nil {
			into[string(n.Class.Name.Name)] = struct{}{}
		}
	case *ast.IfStatement:
		declaredNames(n.Consequent, into)
		declaredNames(n.Alternate, into)
	case *ast.ForStatement, *ast.ForInStatement, *ast.ForOfStatement, *ast.WhileStatement, *ast.DoWhileStatement:
		declaredInLoop(s, into)
	case *ast.TryStatement:
		declaredNames(n.Body, into)
		if n.Catch != nil {
			bindingNames(n.Catch.Parameter, into)
			declaredNames(n.Catch.Body, into)
		}
		declaredNames(n.Finally, into)
	case *ast.SwitchStatement:
		for _, c := range n.Body {
			for _, x := range c.Consequent {
				declaredNames(x, into)
			}
		}
	case *ast.LabelledStatement:
		declaredNames(n.Statement, into)
	}
}

func declaredBindings(list []*ast.Binding, into map[string]struct{}) {
	for _, b := range list {
		bindingNames(b.Target, into)
	}
}

// declaredInLoop adds the names declared by the head and the body of a loop.
func declaredInLoop(s ast.Statement, into map[string]struct{}) {
	switch n := s.(type) {
	case *ast.ForStatement:
		switch init := n.Initializer.(type) {
		case *ast.ForLoopInitializerVarDeclList:
			declaredBindings(init.List, into)
		case *ast.ForLoopInitializerLexicalDecl:
			declaredBindings(init.LexicalDeclaration.List, into)
		}
		declaredNames(n.Body, into)
	case *ast.ForInStatement:
		forInto(n.Into, into)
		declaredNames(n.Body, into)
	case *ast.ForOfStatement:
		forInto(n.Into, into)
		declaredNames(n.Body, into)
	case *ast.WhileStatement:
		declaredNames(n.Body, into)
	case *ast.DoWhileStatement:
		declaredNames(n.Body, into)
	}
}

func forInto(i ast.ForInto, into map[string]struct{}) {
	switch n := i.(type) {
	case *ast.ForIntoVar:
		bindingNames(n.Binding.Target, into)
	case *ast.ForDeclaration:
		bindingNames(n.Target, into)
	}
}

func (w *walker) stmt(s ast.Statement) {
	switch n := s.(type) {
	case nil:
	case *ast.ExpressionStatement:
		w.expr(n.Expression)
	case *ast.BlockStatement:
		if n == nil { // a try statement without a finally block
			return
		}
		for _, x := range n.List {
			w.stmt(x)
		}
	case *ast.ReturnStatement:
		w.expr(n.Argument)
	case *ast.ThrowStatement:
		w.expr(n.Argument)
	case *ast.IfStatement:
		w.expr(n.Test)
		w.stmt(n.Consequent)
		w.stmt(n.Alternate)
	case *ast.VariableStatement:
		for _, b := range n.List {
			w.expr(b.Initializer)
			w.patternDefaults(b.Target)
		}
	case *ast.LexicalDeclaration:
		for _, b := range n.List {
			w.expr(b.Initializer)
			w.patternDefaults(b.Target)
		}
	case *ast.ForStatement:
		switch init := n.Initializer.(type) {
		case *ast.ForLoopInitializerExpression:
			w.expr(init.Expression)
		case *ast.ForLoopInitializerVarDeclList:
			for _, b := range init.List {
				w.expr(b.Initializer)
			}
		case *ast.ForLoopInitializerLexicalDecl:
			for _, b := range init.LexicalDeclaration.List {
				w.expr(b.Initializer)
			}
		}
		w.expr(n.Test)
		w.expr(n.Update)
		w.stmt(n.Body)
	case *ast.ForInStatement:
		w.expr(n.Source)
		w.stmt(n.Body)
	case *ast.ForOfStatement:
		w.expr(n.Source)
		w.stmt(n.Body)
	case *ast.WhileStatement:
		w.expr(n.Test)
		w.stmt(n.Body)
	case *ast.DoWhileStatement:
		w.expr(n.Test)
		w.stmt(n.Body)
	case *ast.SwitchStatement:
		w.expr(n.Discriminant)
		for _, c := range n.Body {
			w.expr(c.Test)
			for _, x := range c.Consequent {
				w.stmt(x)
			}
		}
	case *ast.TryStatement:
		w.stmt(n.Body)
		if n.Catch != nil {
			w.stmt(n.Catch.Body)
		}
		w.stmt(n.Finally)
	case *ast.LabelledStatement:
		w.stmt(n.Statement)
	case *ast.WithStatement:
		w.dynamic = true // a with block can read any name as a property
		w.expr(n.Object)
		w.stmt(n.Body)
	case *ast.FunctionDeclaration:
		if n.Function != nil {
			w.function(n.Function.ParameterList, n.Function.Body, n.Function.Name, nil)
		}
	}
}
