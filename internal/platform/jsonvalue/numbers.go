// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package jsonvalue

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// MaxSafeInteger is the largest integer every IEEE 754 double represents
// exactly, 2^53-1. Integers beyond it are not interoperable (RFC 7493, I-JSON)
// and would collide in the canonical form.
const MaxSafeInteger = 1<<53 - 1

// ExactNumber reports whether the number is safe to hash and to exchange: it
// [RoundTrips], and an integer written without a fraction or exponent also lies
// within ±[MaxSafeInteger] (RFC 7493, I-JSON). Documents are read with this
// rule so no two different numbers can share a content hash.
func ExactNumber(text string) bool {
	if !RoundTrips(text) {
		return false
	}
	if !strings.ContainsAny(text, ".eE") {
		n, err := strconv.ParseInt(text, 10, 64)
		return err == nil && n >= -MaxSafeInteger && n <= MaxSafeInteger
	}
	return true
}

// RoundTrips reports whether the number text survives the canonical form
// unchanged: its exact decimal value equals the value of the canonical text of
// its IEEE 754 double. A number that does not, such as 9007199254740993 or
// 1.00000000000000001, would share its canonical text with a different number.
func RoundTrips(text string) bool {
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(f, 0) {
		return false
	}
	if f == 0 {
		return mantissaIsZero(text) // otherwise the value underflowed
	}
	a, okA := new(big.Rat).SetString(text)
	b, okB := new(big.Rat).SetString(formatES6(f))
	return okA && okB && a.Cmp(b) == 0
}

func mantissaIsZero(text string) bool {
	mantissa, _, _ := strings.Cut(strings.ToLower(text), "e")
	return strings.Trim(mantissa, "-0.") == ""
}

// NumberIssue locates a number that [ExactNumber] rejects.
type NumberIssue struct {
	// Pointer is the JSON Pointer (RFC 6901) of the number.
	Pointer string
	Text    string
}

// InexactNumbers lists up to limit numbers of v that are not [ExactNumber],
// in document order.
func InexactNumbers(v Value, limit int) []NumberIssue {
	var out []NumberIssue
	var walk func(v Value, pointer string)
	walk = func(v Value, pointer string) {
		if len(out) >= limit {
			return
		}
		switch v.kind {
		case KindNumber:
			if !ExactNumber(v.text) {
				out = append(out, NumberIssue{Pointer: pointer, Text: v.text})
			}
		case KindArray:
			for i, item := range v.items {
				walk(item, pointer+"/"+strconv.Itoa(i))
			}
		case KindObject:
			for _, m := range v.members {
				walk(m.Value, pointer+"/"+escapeToken(m.Key))
			}
		case KindInvalid, KindNull, KindBool, KindString:
		}
	}
	walk(v, "")
	return out
}

func escapeToken(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
