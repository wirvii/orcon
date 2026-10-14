// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// maxMagnitude bounds every number in a brand vector. Brand art lives in a box
// a few hundred units wide; a number beyond a million is a mistake or an attack
// on whatever renders the file.
const maxMagnitude = 1e6

// Text limits for <title> and <desc>.
const (
	maxTextRunes = 200
	maxTextWord  = 32
)

// number is one SVG number: optional sign, digits with an optional fraction and
// an optional exponent of at most two digits.
const number = `[-+]?(?:[0-9]+\.?[0-9]*|\.[0-9]+)(?:[eE][-+]?[0-9]{1,2})?`

const ident = `[A-Za-z][A-Za-z0-9_-]{0,31}`

var (
	idValue      = regexp.MustCompile(`^` + ident + `$`)
	localRef     = regexp.MustCompile(`^#` + ident + `$`)
	urlRef       = regexp.MustCompile(`^url\(#(` + ident + `)\)$`)
	colorValue   = regexp.MustCompile(`^(none|currentColor|#[0-9A-Fa-f]{3,8}|[A-Za-z]{3,20})$`)
	lengthValue  = regexp.MustCompile(`^(` + number + `)(px|%)?$`)
	numberOnly   = regexp.MustCompile(`^(` + number + `)$`)
	numberList   = regexp.MustCompile(`^` + number + `(?:[ ,]+` + number + `)*$`)
	numberAnchor = regexp.MustCompile(`^` + number)
	numberAny    = regexp.MustCompile(number)
	aspectRatio  = regexp.MustCompile(`^(none|x(Min|Mid|Max)Y(Min|Mid|Max))( (meet|slice))?$`)
	versionValue = regexp.MustCompile(`^[0-9]\.[0-9]$`)
	transformArg = regexp.MustCompile(`^ *(?:(?:matrix|translate|scale|rotate|skewX|skewY) *\( *` + number + `(?:[ ,]+` + number + `)* *\) *,? *)+$`)
)

// enumValues are the attributes whose value is one of a short list.
var enumValues = map[string][]string{
	"fill-rule":        {"nonzero", "evenodd"},
	"clip-rule":        {"nonzero", "evenodd"},
	"stroke-linecap":   {"butt", "round", "square"},
	"stroke-linejoin":  {"miter", "round", "bevel"},
	"gradientUnits":    {"userSpaceOnUse", "objectBoundingBox"},
	"clipPathUnits":    {"userSpaceOnUse", "objectBoundingBox"},
	"maskUnits":        {"userSpaceOnUse", "objectBoundingBox"},
	"maskContentUnits": {"userSpaceOnUse", "objectBoundingBox"},
	"spreadMethod":     {"pad", "reflect", "repeat"},
}

// lengthAttrs hold one number with an optional px or % unit.
var lengthAttrs = map[string]bool{
	"x": true, "y": true, "width": true, "height": true, "rx": true, "ry": true,
	"cx": true, "cy": true, "r": true, "fx": true, "fy": true,
	"x1": true, "y1": true, "x2": true, "y2": true,
	"stroke-width": true, "stroke-dashoffset": true, "offset": true,
}

// numberAttrs hold exactly one plain number.
var numberAttrs = map[string]bool{
	"opacity": true, "fill-opacity": true, "stroke-opacity": true,
	"stop-opacity": true, "stroke-miterlimit": true,
}

// checkValue validates one attribute value and returns the problem, or "".
// Nothing is trimmed: the value must already be in canonical form, so unusual
// white space cannot hide anything from the check. A backslash is never allowed
// and a parenthesis only inside a transform list or exactly as url(#id).
func checkValue(name, value string) string {
	if strings.Contains(value, `\`) {
		return "contains a backslash"
	}
	switch name {
	case "id":
		if !idValue.MatchString(value) {
			return fmt.Sprintf("id %q is not a safe identifier ([A-Za-z][A-Za-z0-9_-]{0,31})", shorten(value))
		}
	case "href":
		if !localRef.MatchString(value) {
			return fmt.Sprintf("reference %q is not a local #fragment (no external resources)", shorten(value))
		}
	case "fill", "stroke":
		if !colorValue.MatchString(value) && !urlRef.MatchString(value) {
			return fmt.Sprintf("value %q is not a plain paint or a local url(#id)", shorten(value))
		}
	case "stop-color":
		if !colorValue.MatchString(value) {
			return fmt.Sprintf("value %q is not a plain color", shorten(value))
		}
	case "clip-path", "mask":
		if value != "none" && !urlRef.MatchString(value) {
			return fmt.Sprintf("value %q is not none or a local url(#id)", shorten(value))
		}
	case "transform", "gradientTransform":
		if !transformArg.MatchString(value) {
			return fmt.Sprintf("value %q is not a plain transform list", shorten(value))
		}
		return boundedNumbers(value)
	case "d":
		return checkPathData(value)
	case "points":
		return checkPoints(value)
	case "viewBox":
		return checkViewBox(value)
	case "stroke-dasharray":
		if value == "none" {
			return ""
		}
		if !numberList.MatchString(value) {
			return fmt.Sprintf("value %q is not a list of numbers", shorten(value))
		}
		return boundedNumbers(value)
	case "preserveAspectRatio":
		if !aspectRatio.MatchString(value) {
			return fmt.Sprintf("value %q is not a preserveAspectRatio", shorten(value))
		}
	case "version":
		if !versionValue.MatchString(value) {
			return fmt.Sprintf("value %q is not a version number", shorten(value))
		}
	default:
		return checkPlainValue(name, value)
	}
	return ""
}

// checkPlainValue validates the attributes with a simple shape: an enum, a
// length or a number.
func checkPlainValue(name, value string) string {
	if allowed, ok := enumValues[name]; ok {
		for _, a := range allowed {
			if value == a {
				return ""
			}
		}
		return fmt.Sprintf("value %q is not one of %s", shorten(value), strings.Join(allowed, ", "))
	}
	switch {
	case lengthAttrs[name]:
		if !lengthValue.MatchString(value) {
			return fmt.Sprintf("value %q is not a number with an optional px or %% unit", shorten(value))
		}
	case numberAttrs[name]:
		if !numberOnly.MatchString(value) {
			return fmt.Sprintf("value %q is not a number", shorten(value))
		}
	default:
		return fmt.Sprintf("value %q has no validator for attribute %s", shorten(value), name)
	}
	return boundedNumbers(value)
}

// boundedNumbers checks that every number in s is finite and within
// maxMagnitude.
func boundedNumbers(s string) string {
	for _, n := range numberAny.FindAllString(s, -1) {
		if problem := boundedNumber(n); problem != "" {
			return problem
		}
	}
	return ""
}

func boundedNumber(n string) string {
	f, err := strconv.ParseFloat(n, 64)
	if err != nil || f > maxMagnitude || f < -maxMagnitude {
		return fmt.Sprintf("number %q is beyond +-%g", shorten(n), maxMagnitude)
	}
	return ""
}

// pathArity is the number of numbers each path command takes. Arcs are
// special-cased because their flags may be written without separators.
var pathArity = map[byte]int{'m': 2, 'l': 2, 'h': 1, 'v': 1, 'c': 6, 's': 4, 'q': 4, 't': 2, 'a': 7, 'z': 0}

// checkPathData validates the d attribute against the path grammar: a move
// first, then commands and the exact number of numbers each takes. Nothing
// else, in particular no stray letters, may appear.
func checkPathData(d string) string {
	var cmd byte
	count := 0
	closeCommand := func() string {
		if cmd == 0 {
			return ""
		}
		arity := pathArity[cmd|0x20]
		switch {
		case arity == 0 && count != 0:
			return fmt.Sprintf("command %c takes no numbers, found %d", cmd, count)
		case arity > 0 && count == 0:
			return fmt.Sprintf("command %c has no numbers", cmd)
		case arity > 0 && cmd|0x20 != 'a' && count%arity != 0:
			return fmt.Sprintf("command %c has %d numbers, want a multiple of %d", cmd, count, arity)
		}
		return ""
	}
	for i := 0; i < len(d); {
		ch := d[i]
		switch {
		case ch == ' ' || ch == ',' || ch == '\t' || ch == '\n' || ch == '\r':
			i++
		case ch < 0x80 && strings.IndexByte("MmLlHhVvCcSsQqTtAaZz", ch) >= 0:
			if problem := closeCommand(); problem != "" {
				return "path data: " + problem
			}
			if cmd == 0 && ch|0x20 != 'm' {
				return fmt.Sprintf("path data must start with a move command, found %q", ch)
			}
			cmd, count = ch, 0
			i++
		default:
			loc := numberAnchor.FindStringIndex(d[i:])
			if loc == nil || cmd == 0 {
				return fmt.Sprintf("path data: unexpected %q at offset %d", shorten(d[i:]), i)
			}
			if problem := boundedNumber(d[i : i+loc[1]]); problem != "" {
				return "path data: " + problem
			}
			count++
			i += loc[1]
		}
	}
	if cmd == 0 {
		return "path data is empty"
	}
	if problem := closeCommand(); problem != "" {
		return "path data: " + problem
	}
	return ""
}

// checkPoints validates a points list: an even count of bounded numbers.
func checkPoints(s string) string {
	if !numberList.MatchString(s) {
		return fmt.Sprintf("points %q are not a list of numbers", shorten(s))
	}
	if n := len(numberAny.FindAllString(s, -1)); n%2 != 0 {
		return fmt.Sprintf("points hold %d numbers, want pairs", n)
	}
	return boundedNumbers(s)
}

// checkViewBox validates four numbers with a positive width and height.
func checkViewBox(s string) string {
	if !numberList.MatchString(s) {
		return fmt.Sprintf("viewBox %q is not four numbers", shorten(s))
	}
	nums := numberAny.FindAllString(s, -1)
	if len(nums) != 4 {
		return fmt.Sprintf("viewBox %q is not four numbers", shorten(s))
	}
	if problem := boundedNumbers(s); problem != "" {
		return problem
	}
	for _, n := range nums[2:] {
		if f, _ := strconv.ParseFloat(n, 64); f <= 0 {
			return "viewBox width and height must be positive"
		}
	}
	return ""
}

// checkText validates the character data of <title> and <desc>: short, plain
// words, no URLs and no long unbroken runs that could carry an encoded payload.
func checkText(text string) string {
	text = strings.Trim(text, " \t\r\n")
	if n := len([]rune(text)); n > maxTextRunes {
		return fmt.Sprintf("text is %d characters, over the %d limit", n, maxTextRunes)
	}
	for _, r := range text {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune(" .,:()'-_", r) {
			return fmt.Sprintf("text has the character %q; only letters, digits, spaces and . , : ( ) ' - _ are allowed", r)
		}
	}
	for _, word := range strings.Fields(text) {
		if len([]rune(word)) > maxTextWord {
			return fmt.Sprintf("text has a %d character unbroken run, over %d: it could hide an encoded payload", len([]rune(word)), maxTextWord)
		}
	}
	return ""
}
