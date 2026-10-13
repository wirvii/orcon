// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Redacted is what is printed in place of a secret value.
const Redacted = "[REDACTED]"

// Secret is a secret value held in memory. Its text form is [Redacted] for
// every formatter and logger; the plain text is available only through
// [Secret.Reveal], which a caller uses at the point it hands the value to the
// component that needs it. It stands in for the redact types of
// internal/platform/redact until that package exists.
type Secret struct{ plain string }

// NewSecret wraps a plain value.
func NewSecret(plain string) Secret { return Secret{plain: plain} }

// Reveal returns the plain value.
func (s Secret) Reveal() string { return s.plain }

// IsSet reports whether the secret holds a value.
func (s Secret) IsSet() bool { return s.plain != "" }

// String returns [Redacted].
func (Secret) String() string { return Redacted }

// GoString returns [Redacted], so %#v does not leak the value.
func (Secret) GoString() string { return Redacted }

// MarshalText returns [Redacted], so encoders do not leak the value.
func (Secret) MarshalText() ([]byte, error) { return []byte(Redacted), nil }

// LogValue returns [Redacted] for log/slog.
func (Secret) LogValue() slog.Value { return slog.StringValue(Redacted) }

// Value is the resolved value of a key. The zero Value is "unset".
type Value struct {
	scalar string
	list   []string
	pairs  map[string]string
	secret bool
}

// String returns the scalar text, or the items joined by commas for a list or
// a map. A secret value prints as [Redacted].
func (v Value) String() string {
	if v.secret {
		if v.IsZero() {
			return ""
		}
		return Redacted
	}
	return v.plain()
}

func (v Value) plain() string {
	switch {
	case v.pairs != nil:
		names := slices.Sorted(maps.Keys(v.pairs))
		parts := make([]string, 0, len(names))
		for _, n := range names {
			parts = append(parts, n+"="+v.pairs[n])
		}
		return strings.Join(parts, ",")
	case v.list != nil:
		return strings.Join(v.list, ",")
	default:
		return v.scalar
	}
}

// IsZero reports whether the value is empty.
func (v Value) IsZero() bool { return v.scalar == "" && len(v.list) == 0 && len(v.pairs) == 0 }

// Text returns the scalar text of a non-secret value.
func (v Value) Text() string { return v.plain() }

// Secret returns the value as a [Secret].
func (v Value) Secret() Secret { return NewSecret(v.plain()) }

// Int returns the value as an integer, or 0 when it is unset or not a number.
func (v Value) Int() int {
	n, err := strconv.Atoi(v.scalar)
	if err != nil {
		return 0
	}
	return n
}

// Float returns the value as a number, or 0 when it is unset or not a number.
func (v Value) Float() float64 {
	f, err := strconv.ParseFloat(v.scalar, 64)
	if err != nil {
		return 0
	}
	return f
}

// Bool returns the value as a boolean, or false when it is unset.
func (v Value) Bool() bool {
	b, err := strconv.ParseBool(v.scalar)
	return err == nil && b
}

// Duration returns the value as a duration, or 0 when it is unset.
func (v Value) Duration() time.Duration {
	d, err := time.ParseDuration(v.scalar)
	if err != nil {
		return 0
	}
	return d
}

// Bytes returns the value as a size in bytes, or 0 when it is unset.
func (v Value) Bytes() int64 {
	n, err := parseBytes(v.scalar)
	if err != nil {
		return 0
	}
	return n
}

// List returns a copy of the items of a list value.
func (v Value) List() []string { return slices.Clone(v.list) }

// Map returns a copy of a map value.
func (v Value) Map() map[string]string { return maps.Clone(v.pairs) }

func (v Value) equal(o Value) bool {
	return v.scalar == o.scalar && slices.Equal(v.list, o.list) && maps.Equal(v.pairs, o.pairs)
}

// errEmpty is returned for an empty text where the kind needs a value.
var errEmpty = errors.New("must not be empty")

// parseValue turns the text form of a value into a Value. The error never
// contains the text, so it is safe for secret keys.
func parseValue(k *Key, text string) (Value, error) {
	v := Value{secret: k.Secret}
	if text == "" {
		// An empty text clears a key that has no default. A key with a default must keep a value.
		if k.Default != "" || k.DefaultFunc != nil {
			return Value{}, errEmpty
		}
		switch k.Kind {
		case KindList:
			v.list = []string{}
		case KindMap:
			v.pairs = map[string]string{}
		case KindString, KindInt, KindFloat, KindBool, KindDuration, KindEnum, KindPath, KindURL, KindAddress, KindBytes:
		}
		return v, nil
	}
	switch k.Kind {
	case KindList:
		items := splitList(text)
		for _, item := range items {
			if err := checkScalar(k, item); err != nil {
				return Value{}, fmt.Errorf("item: %w", err)
			}
		}
		v.list = items
		return v, nil
	case KindMap:
		pairs, err := parsePairs(text)
		if err != nil {
			return Value{}, err
		}
		v.pairs = pairs
		return v, nil
	case KindString, KindInt, KindFloat, KindBool, KindDuration, KindEnum, KindPath, KindURL, KindAddress, KindBytes:
		canon, err := canonical(k, text)
		if err != nil {
			return Value{}, err
		}
		v.scalar = canon
		return v, nil
	}
	return Value{}, fmt.Errorf("unsupported kind %s", k.Kind)
}

// parseItems builds a list or map value from items already split by a decoder.
func parseItems(k *Key, list []string, pairs map[string]string) (Value, error) {
	v := Value{secret: k.Secret}
	if k.Kind == KindMap {
		for name := range pairs {
			if name == "" {
				return Value{}, errors.New("member names must not be empty")
			}
		}
		v.pairs = maps.Clone(pairs)
		if v.pairs == nil {
			v.pairs = map[string]string{}
		}
		return v, nil
	}
	for _, item := range list {
		if err := checkScalar(k, item); err != nil {
			return Value{}, fmt.Errorf("item: %w", err)
		}
	}
	v.list = slices.Clone(list)
	if v.list == nil {
		v.list = []string{}
	}
	return v, nil
}

func splitList(text string) []string {
	items := []string{}
	for part := range strings.SplitSeq(text, ",") {
		if part = strings.TrimSpace(part); part != "" {
			items = append(items, part)
		}
	}
	return items
}

func parsePairs(text string) (map[string]string, error) {
	pairs := map[string]string{}
	for part := range strings.FieldsFuncSeq(text, func(r rune) bool { return r == ',' || r == '\n' }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, ok := strings.Cut(part, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, errors.New("must be name=value pairs separated by commas")
		}
		pairs[name] = strings.TrimSpace(value)
	}
	return pairs, nil
}

// checkScalar validates one scalar text against the element rules of a key.
func checkScalar(k *Key, text string) error {
	elem := *k
	if k.Kind == KindList {
		elem.Kind = KindString
	}
	_, err := canonical(&elem, text)
	return err
}

// canonical validates a scalar and returns its canonical text.
func canonical(k *Key, text string) (string, error) {
	var out string
	var err error
	switch k.Kind {
	case KindString, KindPath:
		out, err = noControl(text)
	case KindInt:
		out, err = canonInt(k, text)
	case KindFloat:
		out, err = canonFloat(k, text)
	case KindBool:
		out, err = canonBool(text)
	case KindDuration:
		out, err = canonDuration(text)
	case KindEnum:
		if !slices.Contains(k.Enum, text) {
			return "", fmt.Errorf("must be one of %s", strings.Join(k.Enum, ", "))
		}
		out = text
	case KindURL:
		out, err = canonURL(text)
	case KindAddress:
		out, err = canonAddress(text)
	case KindBytes:
		out, err = canonBytes(k, text)
	case KindList, KindMap:
		return "", errors.New("not a scalar")
	}
	if err != nil {
		return "", err
	}
	if k.Validate != nil {
		if err := k.Validate(out); err != nil {
			return "", err
		}
	}
	return out, nil
}

func noControl(text string) (string, error) {
	for _, r := range text {
		if r < 0x20 && r != '\t' || r == 0x7f {
			return "", errors.New("must not contain control characters")
		}
	}
	return text, nil
}

func inRange(k *Key, f float64) error {
	if k.HasRange && (f < k.Min || f > k.Max) {
		return fmt.Errorf("must be between %s and %s", fmtNum(k.Min), fmtNum(k.Max))
	}
	return nil
}

func fmtNum(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

func canonInt(k *Key, text string) (string, error) {
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return "", errors.New("must be an integer")
	}
	if err := inRange(k, float64(n)); err != nil {
		return "", err
	}
	return strconv.FormatInt(n, 10), nil
}

func canonFloat(k *Key, text string) (string, error) {
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return "", errors.New("must be a number")
	}
	if err := inRange(k, f); err != nil {
		return "", err
	}
	return fmtNum(f), nil
}

func canonBool(text string) (string, error) {
	switch strings.ToLower(text) {
	case "true", "1":
		return "true", nil
	case "false", "0":
		return "false", nil
	default:
		return "", errors.New("must be true or false")
	}
}

func canonDuration(text string) (string, error) {
	d, err := time.ParseDuration(text)
	if err != nil || d <= 0 {
		return "", errors.New("must be a positive duration such as 30s, 5m or 24h")
	}
	return text, nil
}

func canonURL(text string) (string, error) {
	u, err := url.Parse(text)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("must be an absolute URL with a scheme and a host")
	}
	return text, nil
}

func canonAddress(text string) (string, error) {
	_, port, err := net.SplitHostPort(text)
	if err != nil {
		return "", errors.New("must be host:port, for example 127.0.0.1:8080 or :8080")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return "", errors.New("must end with a numeric port from 0 to 65535")
	}
	return text, nil
}

var byteUnits = []struct {
	suffix string
	factor int64
}{
	{"TiB", 1 << 40}, {"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}, {"B", 1},
}

func parseBytes(text string) (int64, error) {
	factor := int64(1)
	digits := text
	for _, u := range byteUnits {
		if strings.HasSuffix(text, u.suffix) {
			factor, digits = u.factor, strings.TrimSuffix(text, u.suffix)
			break
		}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(digits), 10, 64)
	if err != nil || n < 0 || n > math.MaxInt64/factor {
		return 0, errors.New("must be a size in bytes, for example 50GiB")
	}
	return n * factor, nil
}

func canonBytes(k *Key, text string) (string, error) {
	n, err := parseBytes(text)
	if err != nil {
		return "", err
	}
	if err := inRange(k, float64(n)); err != nil {
		return "", err
	}
	return text, nil
}
