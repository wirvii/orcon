// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
	"github.com/dop251/goja/token"
	"github.com/dop251/goja/unistring"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// The fast path (ADR-0018 section 2) evaluates a grammar subset of JavaScript
// in Go: literals, member and index access on the item data, arithmetic,
// comparison, logical and conditional operators, with the coercion rules of
// the language. An expression is classified once, when it is compiled. Every
// step that would need behavior the evaluator does not reproduce exactly (an
// object where a primitive is needed, a property that lives on a prototype, a
// read of undefined) returns [errBail], and the sandbox evaluates the
// expression instead, so the fast path never answers differently from the
// interpreter. Differential tests keep the two in step.

// errBail tells the caller to evaluate the expression in the sandbox.
var errBail = errors.New("expr: fast path cannot evaluate this expression")

type kind uint8

const (
	kUndef kind = iota
	kNull
	kBool
	kNum
	kStr
	kComp // an array or an object, kept as the JSON value
)

// val is a JavaScript value restricted to what JSON data can hold.
type val struct {
	k kind
	b bool
	n float64
	s string
	c jsonvalue.Value
}

var (
	undefVal = val{k: kUndef}
	nullVal  = val{k: kNull}
)

func boolVal(b bool) val            { return val{k: kBool, b: b} }
func numVal(n float64) val          { return val{k: kNum, n: n} }
func strVal(s string) val           { return val{k: kStr, s: s} }
func compVal(c jsonvalue.Value) val { return val{k: kComp, c: c} }

// fromJSON converts a JSON value. A number that does not fit a float64 bails
// out: JSON.parse would give Infinity for it, and the sandbox decides.
func fromJSON(v jsonvalue.Value) (val, error) {
	switch v.Kind() {
	case jsonvalue.KindInvalid:
		return undefVal, nil
	case jsonvalue.KindNull:
		return nullVal, nil
	case jsonvalue.KindBool:
		b, _ := v.Bool()
		return boolVal(b), nil
	case jsonvalue.KindNumber:
		f, ok := v.Float64()
		if !ok {
			return val{}, errBail
		}
		return numVal(f), nil
	case jsonvalue.KindString:
		s, _ := v.Str()
		return strVal(s), nil
	case jsonvalue.KindArray, jsonvalue.KindObject:
		return compVal(v), nil
	default:
		return val{}, errBail
	}
}

// result converts a val to the JSON value JSON.stringify would produce:
// undefined has no JSON form, NaN and the infinities become null.
func (v val) result() (Result, error) {
	switch v.k {
	case kUndef:
		return Result{Value: jsonvalue.Null(), Undefined: true}, nil
	case kNull:
		return Result{Value: jsonvalue.Null()}, nil
	case kBool:
		return Result{Value: jsonvalue.Bool(v.b)}, nil
	case kNum:
		if math.IsNaN(v.n) || math.IsInf(v.n, 0) {
			return Result{Value: jsonvalue.Null()}, nil
		}
		n, err := jsonvalue.Number(jsonvalue.FormatNumber(v.n))
		if err != nil {
			return Result{}, err
		}
		return Result{Value: n}, nil
	case kStr:
		return Result{Value: jsonvalue.String(v.s)}, nil
	case kComp:
		return Result{Value: v.c}, nil
	default:
		return Result{}, errBail
	}
}

// ---- coercions ------------------------------------------------------------

func numberText(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	return jsonvalue.FormatNumber(f)
}

// toStr is ToString for primitives; objects and arrays bail out because their
// string form calls into the language (join, toString).
func toStr(v val) (string, error) {
	switch v.k {
	case kUndef:
		return "undefined", nil
	case kNull:
		return "null", nil
	case kBool:
		return strconv.FormatBool(v.b), nil
	case kNum:
		return numberText(v.n), nil
	case kStr:
		return v.s, nil
	case kComp:
		return "", errBail
	}
	return "", errBail
}

func toNum(v val) (float64, error) {
	switch v.k {
	case kUndef:
		return math.NaN(), nil
	case kNull:
		return 0, nil
	case kBool:
		if v.b {
			return 1, nil
		}
		return 0, nil
	case kNum:
		return v.n, nil
	case kStr:
		return stringToNumber(v.s), nil
	case kComp:
		return 0, errBail
	}
	return 0, errBail
}

func truthy(v val) bool {
	switch v.k {
	case kUndef, kNull:
		return false
	case kBool:
		return v.b
	case kNum:
		return v.n != 0 && !math.IsNaN(v.n)
	case kStr:
		return v.s != ""
	case kComp:
		return true
	}
	return false
}

func typeOf(v val) string {
	switch v.k {
	case kUndef:
		return "undefined"
	case kBool:
		return "boolean"
	case kNum:
		return "number"
	case kStr:
		return "string"
	case kNull, kComp:
		return "object" // null, arrays and objects
	}
	return "object"
}

// isJSSpace reports the characters StringToNumber trims: WhiteSpace and
// LineTerminator of ECMA-262.
func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// stringToNumber is the StringToNumber of ECMA-262: trimmed, empty is 0,
// decimal literals with optional sign, Infinity, and 0x/0o/0b integers.
func stringToNumber(s string) float64 {
	s = strings.TrimFunc(s, isJSSpace)
	if s == "" {
		return 0
	}
	if len(s) > 2 && s[0] == '0' {
		base := 0
		switch s[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			var f float64
			for _, c := range s[2:] {
				d := digitValue(c)
				if d < 0 || d >= base {
					return math.NaN()
				}
				f = f*float64(base) + float64(d)
			}
			return f
		}
	}
	body := s
	sign := 1.0
	if body[0] == '+' || body[0] == '-' {
		if body[0] == '-' {
			sign = -1
		}
		body = body[1:]
	}
	if body == "Infinity" {
		return sign * math.Inf(1)
	}
	if !decimalLiteral(body) {
		return math.NaN()
	}
	f, err := strconv.ParseFloat(body, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return math.NaN()
	}
	return sign * f
}

func digitValue(c rune) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// decimalLiteral reports whether s matches StrUnsignedDecimalLiteral:
// digits with an optional fraction or a leading fraction, and an optional
// exponent.
func decimalLiteral(s string) bool {
	i, digits := 0, 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		exp := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			exp++
		}
		if exp == 0 {
			return false
		}
	}
	return i == len(s)
}

// ---- operators ------------------------------------------------------------

func add(a, b val) (val, error) {
	if a.k == kComp || b.k == kComp {
		return val{}, errBail
	}
	if a.k == kStr || b.k == kStr {
		x, _ := toStr(a)
		y, _ := toStr(b)
		return strVal(x + y), nil
	}
	x, _ := toNum(a)
	y, _ := toNum(b)
	return numVal(x + y), nil
}

func arith(op token.Token, a, b val) (val, error) {
	x, err := toNum(a)
	if err != nil {
		return val{}, err
	}
	y, err := toNum(b)
	if err != nil {
		return val{}, err
	}
	switch op { //nolint:exhaustive // the operators of JavaScript are an open set; the rest is not on the fast path
	case token.MINUS:
		return numVal(x - y), nil
	case token.MULTIPLY:
		return numVal(x * y), nil
	case token.SLASH:
		return numVal(x / y), nil
	case token.REMAINDER:
		return numVal(math.Mod(x, y)), nil
	default:
		return val{}, errBail
	}
}

func strictEq(a, b val) (bool, error) {
	if a.k != b.k {
		return false, nil
	}
	switch a.k {
	case kUndef, kNull:
		return true, nil
	case kBool:
		return a.b == b.b, nil
	case kNum:
		return a.n == b.n, nil
	case kStr:
		return a.s == b.s, nil
	case kComp:
		return false, errBail // two objects compare by identity
	}
	return false, errBail
}

func looseEq(a, b val) (bool, error) {
	if a.k == kComp || b.k == kComp {
		if a.k == kComp && b.k == kComp {
			return false, errBail
		}
		other := a
		if a.k == kComp {
			other = b
		}
		if other.k == kUndef || other.k == kNull {
			return false, nil
		}
		return false, errBail // object against a primitive converts the object
	}
	if a.k == b.k {
		return strictEq(a, b)
	}
	nullish := func(v val) bool { return v.k == kUndef || v.k == kNull }
	if nullish(a) || nullish(b) {
		return nullish(a) && nullish(b), nil
	}
	x, _ := toNum(a)
	y, _ := toNum(b)
	return x == y, nil
}

// less implements the comparison a < b (IsLessThan); the result is false
// when either side is NaN.
func less(a, b val) (bool, error) {
	if a.k == kComp || b.k == kComp {
		return false, errBail
	}
	if a.k == kStr && b.k == kStr {
		return compareStrings(a.s, b.s) < 0, nil
	}
	x, _ := toNum(a)
	y, _ := toNum(b)
	return x < y, nil
}

func compareStrings(a, b string) int {
	if isASCII(a) && isASCII(b) {
		return strings.Compare(a, b)
	}
	return jsonvalue.CompareUTF16(a, b)
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func compare(op token.Token, a, b val) (bool, error) {
	if a.k == kComp || b.k == kComp {
		return false, errBail
	}
	nan := func(v val) bool { x, _ := toNum(v); return math.IsNaN(x) }
	bothStrings := a.k == kStr && b.k == kStr
	if !bothStrings && (nan(a) || nan(b)) {
		return false, nil
	}
	switch op { //nolint:exhaustive // the operators of JavaScript are an open set; the rest is not on the fast path
	case token.LESS:
		return less(a, b)
	case token.GREATER:
		return less(b, a)
	case token.LESS_OR_EQUAL:
		r, err := less(b, a)
		return !r, err
	case token.GREATER_OR_EQUAL:
		r, err := less(a, b)
		return !r, err
	default:
		return false, errBail
	}
}

// ---- member access --------------------------------------------------------

// canonicalIndex parses a canonical array index ("0", "17"; no sign, no
// leading zero) below 2^32-1.
func canonicalIndex(key string) (int, bool) {
	if key == "" || len(key) > 10 || (len(key) > 1 && key[0] == '0') {
		return 0, false
	}
	n := 0
	for i := range len(key) {
		if key[i] < '0' || key[i] > '9' {
			return 0, false
		}
		n = n*10 + int(key[i]-'0')
	}
	if n >= math.MaxUint32 {
		return 0, false
	}
	return n, true
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// member reads the property key of v the way the language does, for the cases
// the fast path reproduces exactly.
func member(v val, key string) (val, error) {
	switch v.k {
	case kComp:
		if v.c.Kind() == jsonvalue.KindObject {
			return objectMember(v.c, key)
		}
		return arrayMember(v.c, key)
	case kStr:
		return stringMember(v.s, key)
	case kUndef, kNull, kBool, kNum:
		// Reading a property of undefined or null throws; numbers and booleans are rare.
		return val{}, errBail
	}
	return val{}, errBail
}

func objectMember(o jsonvalue.Value, key string) (val, error) {
	if m, ok := o.Get(key); ok {
		return fromJSON(m)
	}
	if inProto(objectProto, key) {
		return val{}, errBail
	}
	return undefVal, nil
}

func arrayMember(a jsonvalue.Value, key string) (val, error) {
	if key == "length" {
		return numVal(float64(a.Len())), nil
	}
	if i, ok := canonicalIndex(key); ok {
		if item, ok := a.At(i); ok {
			return fromJSON(item)
		}
		return undefVal, nil
	}
	if inProto(arrayProto, key) || inProto(objectProto, key) {
		return val{}, errBail
	}
	return undefVal, nil
}

func stringMember(str, key string) (val, error) {
	if key == "length" {
		return numVal(float64(utf16Len(str))), nil
	}
	if i, ok := canonicalIndex(key); ok {
		units := utf16.Encode([]rune(str))
		if i >= len(units) {
			return undefVal, nil
		}
		if r := rune(units[i]); !utf16.IsSurrogate(r) {
			return strVal(string(r)), nil
		}
		return val{}, errBail
	}
	if inProto(stringProto, key) || inProto(objectProto, key) {
		return val{}, errBail
	}
	return undefVal, nil
}

// ---- compilation ------------------------------------------------------------

type fastFn func(*Env) (val, error)

// fastProgram is an expression compiled for the fast path.
type fastProgram struct {
	eval fastFn
}

// compileFast classifies code. It returns nil when the expression is outside
// the fast-path grammar (or does not parse); the sandbox then evaluates it.
func compileFast(code string) *fastProgram {
	prog, err := parser.ParseFile(nil, "", "(\n"+trimCode(code)+"\n)", 0)
	if err != nil || len(prog.Body) != 1 {
		return nil
	}
	st, ok := prog.Body[0].(*ast.ExpressionStatement)
	if !ok {
		return nil
	}
	fn, ok := compileExpr(st.Expression)
	if !ok {
		return nil
	}
	return &fastProgram{eval: fn}
}

func identName(e ast.Expression) (string, bool) {
	if id, ok := e.(*ast.Identifier); ok {
		return id.Name.String(), true
	}
	return "", false
}

// dotOf matches <left>.name and returns left and name.
func dotOf(e ast.Expression) (ast.Expression, string, bool) { //nolint:ireturn // AST nodes are an interface family
	if d, ok := e.(*ast.DotExpression); ok {
		return d.Left, d.Identifier.Name.String(), true
	}
	return nil, "", false
}

func isIdent(e ast.Expression, name string) bool {
	n, ok := identName(e)
	return ok && n == name
}

// emptyCallOf matches <callee>() and returns the callee.
func emptyCallOf(e ast.Expression) (ast.Expression, bool) { //nolint:ireturn // AST nodes are an interface family
	c, ok := e.(*ast.CallExpression)
	if !ok || len(c.ArgumentList) != 0 {
		return nil, false
	}
	return c.Callee, true
}

// nodeCallOf matches $("Name") and returns the literal name.
func nodeCallOf(e ast.Expression) (string, bool) {
	c, ok := e.(*ast.CallExpression)
	if !ok || len(c.ArgumentList) != 1 || !isIdent(c.Callee, "$") {
		return "", false
	}
	return stringLiteral(c.ArgumentList[0])
}

// itemSource matches the expressions that denote one item of the input or of
// another node ($input.item, $input.first(), $("X").item, $("X").first(),
// $node["X"]) and returns a function that finds it, or reports a bail.
func itemSource(e ast.Expression) (func(*Env) (Item, error), bool) {
	// $input.item
	if left, name, ok := dotOf(e); ok && name == "item" && isIdent(left, "$input") {
		return func(env *Env) (Item, error) { return env.current(), nil }, true
	}
	// $input.first() / $input.last()
	if callee, ok := emptyCallOf(e); ok {
		if left, name, ok := dotOf(callee); ok && (name == "first" || name == "last") {
			if isIdent(left, "$input") {
				return func(env *Env) (Item, error) { return pick(env.Input, name == "last") }, true
			}
			if node, ok := nodeCallOf(left); ok {
				return func(env *Env) (Item, error) {
					out, err := lookup(env, node)
					if err != nil {
						return Item{}, err
					}
					return pick(out.Items, name == "last")
				}, true
			}
		}
	}
	// $("X").item
	if left, name, ok := dotOf(e); ok && name == "item" {
		if node, ok := nodeCallOf(left); ok {
			return func(env *Env) (Item, error) { return paired(env, node) }, true
		}
	}
	// $node["X"]
	if b, ok := e.(*ast.BracketExpression); ok && isIdent(b.Left, "$node") {
		if node, ok := stringLiteral(b.Member); ok {
			return func(env *Env) (Item, error) { return paired(env, node) }, true
		}
	}
	return nil, false
}

func pick(items []Item, last bool) (Item, error) {
	if len(items) == 0 {
		return Item{}, errBail // .first() of nothing is undefined and its .json throws
	}
	if last {
		return items[len(items)-1], nil
	}
	return items[0], nil
}

func lookup(env *Env, name string) (NodeOutput, error) {
	if env.Nodes == nil {
		return NodeOutput{}, errBail
	}
	out, ok := env.Nodes.Node(name)
	if !ok || !out.Executed {
		return NodeOutput{}, errBail // the sandbox raises the documented error
	}
	return out, nil
}

func paired(env *Env, name string) (Item, error) {
	out, err := lookup(env, name)
	if err != nil {
		return Item{}, err
	}
	if out.Paired == nil {
		return Item{}, errBail
	}
	it, ok := out.Paired(env.ItemIndex)
	if !ok {
		return Item{}, errBail
	}
	return it, nil
}

func jsonOf(v jsonvalue.Value) (val, error) {
	if v.IsZero() {
		return fromJSON(emptyObject)
	}
	return fromJSON(v)
}

var emptyObject = func() jsonvalue.Value {
	o, err := jsonvalue.Object()
	if err != nil {
		panic(err) // an empty object cannot have duplicate keys
	}
	return o
}()

// hasLoneSurrogate reports whether a string literal holds an unpaired UTF-16
// surrogate, which has no UTF-8 form.
func hasLoneSurrogate(s unistring.String) bool {
	units := s.AsUtf16()
	if len(units) < 2 {
		return false
	}
	units = units[1:] // the first unit is the marker of the representation
	for i := 0; i < len(units); i++ {
		switch u := rune(units[i]); {
		case utf16.IsSurrogate(u) && u < 0xdc00 && i+1 < len(units) && units[i+1] >= 0xdc00 && units[i+1] < 0xe000:
			i++ // a valid pair
		case utf16.IsSurrogate(u):
			return true
		}
	}
	return false
}

func constFn(v val) fastFn { return func(*Env) (val, error) { return v, nil } }

func compileExpr(e ast.Expression) (fastFn, bool) {
	switch n := e.(type) {
	case *ast.NumberLiteral:
		switch x := n.Value.(type) {
		case int64:
			return constFn(numVal(float64(x))), true
		case float64:
			return constFn(numVal(x)), true
		}
		return nil, false
	case *ast.StringLiteral:
		s := n.Value.String()
		if !utf8.ValidString(s) || hasLoneSurrogate(n.Value) {
			return nil, false // JavaScript keeps a lone surrogate; Go text cannot
		}
		return constFn(strVal(s)), true
	case *ast.BooleanLiteral:
		return constFn(boolVal(n.Value)), true
	case *ast.NullLiteral:
		return constFn(nullVal), true
	case *ast.Identifier:
		return compileIdent(n.Name.String())
	case *ast.DotExpression:
		return compileDot(n)
	case *ast.BracketExpression:
		return compileBracket(n)
	case *ast.UnaryExpression:
		return compileUnary(n)
	case *ast.BinaryExpression:
		return compileBinary(n)
	case *ast.ConditionalExpression:
		test, ok1 := compileExpr(n.Test)
		yes, ok2 := compileExpr(n.Consequent)
		no, ok3 := compileExpr(n.Alternate)
		if !ok1 || !ok2 || !ok3 {
			return nil, false
		}
		return func(env *Env) (val, error) {
			t, err := test(env)
			if err != nil {
				return val{}, err
			}
			if truthy(t) {
				return yes(env)
			}
			return no(env)
		}, true
	}
	return nil, false
}

func compileIdent(name string) (fastFn, bool) {
	switch name {
	case "$json":
		return func(env *Env) (val, error) { return jsonOf(env.current().JSON) }, true
	case "$itemIndex", "$position":
		return func(env *Env) (val, error) { return numVal(float64(env.ItemIndex)), nil }, true
	case "$runIndex":
		return func(env *Env) (val, error) { return numVal(float64(env.RunIndex)), nil }, true
	case "undefined":
		return constFn(undefVal), true
	case "NaN":
		return constFn(numVal(math.NaN())), true
	case "Infinity":
		return constFn(numVal(math.Inf(1))), true
	}
	return nil, false
}

func compileDot(n *ast.DotExpression) (fastFn, bool) {
	name := n.Identifier.Name.String()
	// <item source>.json
	if name == "json" {
		if src, ok := itemSource(n.Left); ok {
			return func(env *Env) (val, error) {
				it, err := src(env)
				if err != nil {
					return val{}, err
				}
				return jsonOf(it.JSON)
			}, true
		}
	}
	// $("X").isExecuted
	if name == "isExecuted" {
		if node, ok := nodeCallOf(n.Left); ok {
			return func(env *Env) (val, error) {
				if env.Nodes == nil {
					return val{}, errBail
				}
				out, ok := env.Nodes.Node(node)
				if !ok {
					return val{}, errBail
				}
				return boolVal(out.Executed), nil
			}, true
		}
	}
	left, ok := compileExpr(n.Left)
	if !ok {
		return nil, false
	}
	return func(env *Env) (val, error) {
		base, err := left(env)
		if err != nil {
			return val{}, err
		}
		return member(base, name)
	}, true
}

func compileBracket(n *ast.BracketExpression) (fastFn, bool) {
	if _, ok := itemSource(n); ok {
		return nil, false // $node["X"] alone is an object, not data
	}
	left, ok := compileExpr(n.Left)
	if !ok {
		return nil, false
	}
	key, ok := compileExpr(n.Member)
	if !ok {
		return nil, false
	}
	return func(env *Env) (val, error) {
		base, err := left(env)
		if err != nil {
			return val{}, err
		}
		k, err := key(env)
		if err != nil {
			return val{}, err
		}
		name, err := toStr(k)
		if err != nil {
			return val{}, err
		}
		return member(base, name)
	}, true
}

func compileUnary(n *ast.UnaryExpression) (fastFn, bool) {
	if n.Postfix {
		return nil, false
	}
	operand, ok := compileExpr(n.Operand)
	if !ok {
		return nil, false
	}
	switch n.Operator { //nolint:exhaustive // the operators of JavaScript are an open set; the rest is not on the fast path
	case token.NOT:
		return func(env *Env) (val, error) {
			v, err := operand(env)
			return boolVal(!truthy(v)), err
		}, true
	case token.MINUS, token.PLUS:
		neg := n.Operator == token.MINUS
		return func(env *Env) (val, error) {
			v, err := operand(env)
			if err != nil {
				return val{}, err
			}
			f, err := toNum(v)
			if err != nil {
				return val{}, err
			}
			if neg {
				f = -f
			}
			return numVal(f), nil
		}, true
	case token.TYPEOF:
		if id, isIdent := n.Operand.(*ast.Identifier); isIdent {
			if _, known := compileIdent(id.Name.String()); !known {
				return nil, false // typeof of an undeclared name does not throw
			}
		}
		return func(env *Env) (val, error) {
			v, err := operand(env)
			if err != nil {
				return val{}, err
			}
			return strVal(typeOf(v)), nil
		}, true
	}
	return nil, false
}

func compileBinary(n *ast.BinaryExpression) (fastFn, bool) {
	left, ok := compileExpr(n.Left)
	if !ok {
		return nil, false
	}
	right, ok := compileExpr(n.Right)
	if !ok {
		return nil, false
	}
	op := n.Operator
	switch op { //nolint:exhaustive // the operators of JavaScript are an open set; the rest is not on the fast path
	case token.LOGICAL_AND, token.LOGICAL_OR, token.COALESCE:
		return func(env *Env) (val, error) {
			a, err := left(env)
			if err != nil {
				return val{}, err
			}
			switch op { //nolint:exhaustive // only the three logical operators reach this closure
			case token.LOGICAL_AND:
				if !truthy(a) {
					return a, nil
				}
			case token.LOGICAL_OR:
				if truthy(a) {
					return a, nil
				}
			default:
				if a.k != kUndef && a.k != kNull {
					return a, nil
				}
			}
			return right(env)
		}, true
	}
	var apply func(a, b val) (val, error)
	switch op { //nolint:exhaustive // the operators of JavaScript are an open set; the rest is not on the fast path
	case token.PLUS:
		apply = add
	case token.MINUS, token.MULTIPLY, token.SLASH, token.REMAINDER:
		apply = func(a, b val) (val, error) { return arith(op, a, b) }
	case token.STRICT_EQUAL, token.STRICT_NOT_EQUAL:
		apply = func(a, b val) (val, error) {
			eq, err := strictEq(a, b)
			return boolVal(eq == (op == token.STRICT_EQUAL)), err
		}
	case token.EQUAL, token.NOT_EQUAL:
		apply = func(a, b val) (val, error) {
			eq, err := looseEq(a, b)
			return boolVal(eq == (op == token.EQUAL)), err
		}
	case token.LESS, token.GREATER, token.LESS_OR_EQUAL, token.GREATER_OR_EQUAL:
		apply = func(a, b val) (val, error) {
			r, err := compare(op, a, b)
			return boolVal(r), err
		}
	default:
		return nil, false
	}
	return func(env *Env) (val, error) {
		a, err := left(env)
		if err != nil {
			return val{}, err
		}
		b, err := right(env)
		if err != nil {
			return val{}, err
		}
		return apply(a, b)
	}, true
}
