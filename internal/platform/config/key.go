// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strings"
	"unicode"
)

// Kind is the value type of a configuration key.
type Kind int

// The kinds of value a key can hold. Every kind has one textual form, which is
// what a flag, an environment variable and a YAML scalar carry.
const (
	// KindString is free text.
	KindString Kind = iota
	// KindInt is a base-10 integer.
	KindInt
	// KindFloat is a decimal number.
	KindFloat
	// KindBool is true or false.
	KindBool
	// KindDuration is a positive Go duration such as 30s or 24h.
	KindDuration
	// KindEnum is one of a closed set of words.
	KindEnum
	// KindPath is a file system path.
	KindPath
	// KindURL is an absolute URL.
	KindURL
	// KindAddress is a host:port listen address.
	KindAddress
	// KindBytes is a size in bytes, with an optional binary suffix (KiB, MiB, GiB, TiB).
	KindBytes
	// KindList is a list of strings, written comma-separated in text form.
	KindList
	// KindMap is a map of strings, written name=value pairs separated by commas in text form.
	KindMap
)

// String returns the name of the kind as shown in the generated reference.
func (k Kind) String() string {
	switch k {
	case KindString:
		return "string"
	case KindInt:
		return "integer"
	case KindFloat:
		return "number"
	case KindBool:
		return "boolean"
	case KindDuration:
		return "duration"
	case KindEnum:
		return "enum"
	case KindPath:
		return "path"
	case KindURL:
		return "URL"
	case KindAddress:
		return "host:port"
	case KindBytes:
		return "bytes"
	case KindList:
		return "list"
	case KindMap:
		return "map"
	default:
		return "unknown"
	}
}

// Reload says when a change of a key takes effect (R-CONF-09).
type Reload int

// The reload classes of a key.
const (
	// ReloadRestart keys are read once, at start. A reload keeps the running value.
	ReloadRestart Reload = iota
	// ReloadLive keys are read again on SIGHUP.
	ReloadLive
	// ReloadFirstStart keys are consumed only by the first start of a new instance.
	ReloadFirstStart
)

// String returns the name of the reload class as shown in the generated reference.
func (r Reload) String() string {
	switch r {
	case ReloadRestart:
		return "restart"
	case ReloadLive:
		return "live"
	case ReloadFirstStart:
		return "first start"
	default:
		return "unknown"
	}
}

// Text is a description in the two languages of the documentation (decision P3).
type Text struct {
	// EN is the English text.
	EN string
	// ES is the Spanish text.
	ES string
}

// Lookup returns the resolved text value of a key that comes earlier in the
// schema. It is how a computed default depends on another key.
type Lookup func(path string) string

// DefaultFunc computes the default of a key from keys that come earlier in the
// schema and from the platform (R-CONF-02).
type DefaultFunc func(env Platform, get Lookup) (string, error)

// Platform is what computed defaults may ask about the host.
type Platform struct {
	// GOOS is the operating system name, as runtime.GOOS spells it.
	GOOS string
	// Getenv reads a process environment variable.
	Getenv func(name string) string
}

// Key declares one server configuration key. A key is declared exactly once, in
// this package, and everything else (flag, environment variable, YAML member,
// validation, JSON Schema, reference) is derived from it.
type Key struct {
	// Path is the dotted camelCase path of the key, for example store.postgres.maxConnections.
	Path string
	// Kind is the value type.
	Kind Kind
	// Enum lists the allowed words of a KindEnum key.
	Enum []string
	// Min and Max bound KindInt, KindFloat and KindBytes keys when HasRange is true.
	Min, Max float64
	// HasRange turns Min and Max on.
	HasRange bool
	// Default is the textual default. Empty means no default.
	Default string
	// DefaultFunc computes the default when it depends on the host or on other keys. It wins over Default.
	DefaultFunc DefaultFunc
	// DefaultDoc describes a computed default in the reference.
	DefaultDoc Text
	// Secret marks a key whose value is never printed, logged or accepted as a flag value.
	Secret bool
	// Reload says when a change takes effect.
	Reload Reload
	// Roles lists the roles that read the key. Empty means every role.
	Roles []Role
	// Env overrides the derived environment variable name. Only the three names fixed by ADR 0042 and ADR 0039 use it.
	Env string
	// Validate checks a value that already has the right kind. It must never put the value in the error of a secret key.
	Validate func(value string) error
	// Description explains the key to an operator.
	Description Text
}

// EnvPrefix starts every environment variable of the server.
const EnvPrefix = "ORCON_"

// fileSuffix is appended to a secret key to name the key that holds a file path.
const fileSuffix = "File"

// words splits a camelCase path segment into its words.
func words(segment string) []string {
	var out []string
	start := 0
	runes := []rune(segment)
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) && !unicode.IsUpper(runes[i-1]) {
			out = append(out, string(runes[start:i]))
			start = i
		}
	}
	return append(out, string(runes[start:]))
}

func joinWords(path, wordSep, segSep string, transform func(string) string) string {
	segs := strings.Split(path, ".")
	parts := make([]string, 0, len(segs))
	for _, seg := range segs {
		ws := words(seg)
		for i := range ws {
			ws[i] = transform(ws[i])
		}
		parts = append(parts, strings.Join(ws, wordSep))
	}
	return strings.Join(parts, segSep)
}

// EnvName returns the environment variable of a key path: ORCON_ plus the
// segments in UPPER_SNAKE (R-CONF-04).
func EnvName(path string) string {
	return EnvPrefix + joinWords(path, "_", "_", strings.ToUpper)
}

// FlagName returns the command-line flag of a key path, without the leading
// dashes: the segments in kebab-case (R-CONF-05).
func FlagName(path string) string {
	return joinWords(path, "-", "-", strings.ToLower)
}

// FilePath returns the YAML path of the file variant of a secret key: the last
// segment with File appended (R-CONF-06).
func FilePath(path string) string { return path + fileSuffix }

// envName returns the variable that carries the value of the key.
func (k *Key) envName() string {
	if k.Env != "" {
		return k.Env
	}
	return EnvName(k.Path)
}

// EnvName returns the environment variable that carries the value of the key.
func (k *Key) EnvName() string { return k.envName() }

// EnvFileName returns the environment variable that carries the path of a file
// holding the value. Only secret keys have one.
func (k *Key) EnvFileName() string { return k.envName() + "_FILE" }

// FlagName returns the flag that carries the value, without the leading dashes.
// A secret key has no such flag; use FlagFileName.
func (k *Key) FlagName() string { return FlagName(k.Path) }

// FlagFileName returns the flag that carries the path of a file holding the value.
func (k *Key) FlagFileName() string { return FlagName(k.Path) + "-file" }

// FileKey returns the YAML path of the file variant of the key.
func (k *Key) FileKey() string { return FilePath(k.Path) }

// AppliesTo reports whether the key is read by the role.
func (k *Key) AppliesTo(r Role) bool {
	if len(k.Roles) == 0 {
		return true
	}
	for _, have := range k.Roles {
		if have == r {
			return true
		}
	}
	return false
}
