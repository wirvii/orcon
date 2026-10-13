// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package jsonvalue

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// ErrNotEncodable marks a value that has no JSON text: the absent value, or
// (for canonical form) a number outside the IEEE 754 double range.
var ErrNotEncodable = errors.New("jsonvalue: value cannot be encoded")

// Marshal returns the compact JSON text of v. Object members keep their
// insertion order and numbers keep their text, so Parse followed by Marshal
// does not change the data.
func Marshal(v Value) ([]byte, error) {
	var b strings.Builder
	if err := write(&b, v, false); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// Canonical returns the JSON Canonicalization Scheme text of v (RFC 8785):
// object members sorted by UTF-16 code units, no whitespace, and numbers in
// the ECMAScript form of their IEEE 754 double value.
func Canonical(v Value) ([]byte, error) {
	var b strings.Builder
	if err := write(&b, v, true); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func write(b *strings.Builder, v Value, canonical bool) error {
	switch v.kind {
	case KindNull:
		b.WriteString("null")
	case KindBool:
		b.WriteString(strconv.FormatBool(v.flag))
	case KindNumber:
		return writeNumber(b, v.text, canonical)
	case KindString:
		return writeString(b, v.text)
	case KindArray:
		return writeArray(b, v.items, canonical)
	case KindObject:
		return writeObject(b, v.members, canonical)
	case KindInvalid:
		return fmt.Errorf("%w: absent value", ErrNotEncodable)
	default:
		return fmt.Errorf("%w: kind %d", ErrNotEncodable, v.kind)
	}
	return nil
}

func writeNumber(b *strings.Builder, text string, canonical bool) error {
	if !canonical {
		b.WriteString(text)
		return nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(f, 0) {
		return fmt.Errorf("%w: number %q is outside the double range", ErrNotEncodable, text)
	}
	if !RoundTrips(text) {
		return fmt.Errorf("%w: number %q does not round-trip through a double (integers beyond ±2^53-1 and digits beyond double precision would collide)", ErrNotEncodable, text)
	}
	b.WriteString(formatES6(f))
	return nil
}

func writeArray(b *strings.Builder, items []Value, canonical bool) error {
	b.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			b.WriteByte(',')
		}
		if err := write(b, item, canonical); err != nil {
			return err
		}
	}
	b.WriteByte(']')
	return nil
}

func writeObject(b *strings.Builder, members []Member, canonical bool) error {
	if canonical {
		members = slices.Clone(members)
		slices.SortFunc(members, func(x, y Member) int { return compareUTF16(x.Key, y.Key) })
	}
	b.WriteByte('{')
	for i, m := range members {
		if i > 0 {
			b.WriteByte(',')
		}
		if err := writeString(b, m.Key); err != nil {
			return err
		}
		b.WriteByte(':')
		if err := write(b, m.Value, canonical); err != nil {
			return err
		}
	}
	b.WriteByte('}')
	return nil
}

// compareUTF16 orders strings by their UTF-16 code units, as RFC 8785
// section 3.2.3 requires for object keys.
func compareUTF16(a, b string) int {
	return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(b)))
}

const hexDigits = "0123456789abcdef"

// writeString writes s as a JSON string using the minimal escaping that RFC
// 8785 section 3.2.2.2 prescribes. A string that is not well-formed UTF-8 has
// no JSON text: replacing it would make two different strings share one text.
func writeString(b *strings.Builder, s string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("%w: string is not valid UTF-8", ErrNotEncodable)
	}
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20:
			b.WriteString(`\u00`)
			b.WriteByte(hexDigits[r>>4])
			b.WriteByte(hexDigits[r&0xf])
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return nil
}

// formatES6 formats f as ECMAScript Number.prototype.toString does, which is
// the number form RFC 8785 section 3.2.2.3 requires.
func formatES6(f float64) string {
	if f == 0 {
		return "0" // also folds negative zero
	}
	sign := ""
	if f < 0 {
		sign = "-"
		f = -f
	}
	// 'e' with precision -1 yields the shortest digits that round-trip.
	exp := strconv.FormatFloat(f, 'e', -1, 64) // d.ddde±XX
	mantissa, expText, _ := strings.Cut(exp, "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	e, _ := strconv.Atoi(expText) // always valid: produced by FormatFloat
	k := len(digits)
	n := e + 1 // position of the decimal point relative to digits
	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits
	}
	expSign := "+"
	if n-1 < 0 {
		expSign = "-"
	}
	absExp := strconv.Itoa(max(n-1, 1-n))
	if k == 1 {
		return sign + digits + "e" + expSign + absExp
	}
	return sign + digits[:1] + "." + digits[1:] + "e" + expSign + absExp
}
