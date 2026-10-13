// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// ClientEnvNames are the environment variables reserved for the command-line
// client (ADR 0058) and for the location of the configuration file. No server
// key may produce one of them, as a value or as a file variant (R-CONF-04).
func ClientEnvNames() []string {
	return []string{
		"ORCON_URL",
		"ORCON_TOKEN",
		"ORCON_CONTEXT",
		"ORCON_CLIENT_ID",
		"ORCON_CLIENT_SECRET",
		"ORCON_FEDERATED_TOKEN_FILE",
		"ORCON_CONFIG",
	}
}

// ReservedFlags are command-line flags that belong to the commands, not to a key.
func ReservedFlags() []string {
	return []string{"config", "probe", "o", "help", "h", "out"}
}

// Schema is the set of server keys. Build one with [NewSchema], or use
// [Default] for the keys of the product.
type Schema struct {
	keys   []*Key
	byPath map[string]*Key
	// byFile maps the YAML path of a file variant to its secret key.
	byFile map[string]*Key
	// groups holds every proper prefix of a key path.
	groups map[string]struct{}
}

// NewSchema validates the keys and indexes them. It reports every problem it
// finds, so a bad declaration is fixed in one pass. The order of the keys is
// the order of resolution: a computed default may read only earlier keys.
func NewSchema(keys []Key) (*Schema, error) {
	s := &Schema{
		keys:   make([]*Key, 0, len(keys)),
		byPath: make(map[string]*Key, len(keys)),
		byFile: make(map[string]*Key),
		groups: make(map[string]struct{}),
	}
	var problems []error
	names := map[string]string{} // environment variable or flag -> owner
	claim := func(kind, name, owner string) {
		id := kind + " " + name
		if prev, taken := names[id]; taken {
			problems = append(problems, fmt.Errorf("%s %s is produced by both %s and %s", kind, name, prev, owner))
			return
		}
		names[id] = owner
	}
	for _, name := range ClientEnvNames() {
		names["environment variable "+name] = "the command-line client"
	}
	for _, name := range ReservedFlags() {
		names["flag --"+name] = "the command line"
	}

	for i := range keys {
		k := &keys[i]
		problems = append(problems, s.add(k)...)
		if k.Secret {
			claim("environment variable", k.EnvFileName(), k.Path)
			claim("flag", "--"+k.FlagFileName(), k.Path)
		} else {
			claim("flag", "--"+k.FlagName(), k.Path)
		}
		claim("environment variable", k.envName(), k.Path)
	}
	for path := range s.byPath {
		if _, clash := s.groups[path]; clash {
			problems = append(problems, fmt.Errorf("key %s is both a value and a section of other keys", path))
		}
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("invalid configuration schema: %w", errors.Join(problems...))
	}
	return s, nil
}

func (s *Schema) add(k *Key) []error {
	var problems []error
	if k.Path == "" || strings.HasPrefix(k.Path, ".") || strings.HasSuffix(k.Path, ".") || strings.Contains(k.Path, "..") {
		return []error{fmt.Errorf("key %q has a malformed path", k.Path)}
	}
	if _, dup := s.byPath[k.Path]; dup {
		return []error{fmt.Errorf("key %s is declared twice", k.Path)}
	}
	if k.Description.EN == "" || k.Description.ES == "" {
		problems = append(problems, fmt.Errorf("key %s needs a description in English and Spanish", k.Path))
	}
	if k.Kind == KindEnum && len(k.Enum) == 0 {
		problems = append(problems, fmt.Errorf("key %s is an enum without values", k.Path))
	}
	if k.Secret && k.Default != "" {
		problems = append(problems, fmt.Errorf("secret key %s must not have a default", k.Path))
	}
	if k.Secret && (k.Kind == KindBool || k.Kind == KindEnum) {
		problems = append(problems, fmt.Errorf("secret key %s must be free text, a list or a map", k.Path))
	}
	if k.Default != "" && k.DefaultFunc == nil {
		if _, err := parseValue(k, k.Default); err != nil {
			problems = append(problems, fmt.Errorf("default of key %s is invalid: %w", k.Path, err))
		}
	}
	s.byPath[k.Path] = k
	s.keys = append(s.keys, k)
	if k.Secret {
		s.byFile[k.FileKey()] = k
	}
	segs := strings.Split(k.Path, ".")
	for i := 1; i < len(segs); i++ {
		s.groups[strings.Join(segs[:i], ".")] = struct{}{}
	}
	return problems
}

// Keys returns the keys in resolution order. The slice is a copy; the keys are shared and must not be changed.
func (s *Schema) Keys() []*Key { return slices.Clone(s.keys) }

// Key returns the key declared at path.
func (s *Schema) Key(path string) (*Key, bool) {
	k, ok := s.byPath[path]
	return k, ok
}

// IsLeaf reports whether a YAML path holds a value: a key, or the file variant of a secret key.
// A decoder uses it to know where a mapping stops being a section.
func (s *Schema) IsLeaf(path string) bool {
	if _, ok := s.byPath[path]; ok {
		return true
	}
	_, ok := s.byFile[path]
	return ok
}

// IsSection reports whether path is a proper prefix of some key.
func (s *Schema) IsSection(path string) bool {
	_, ok := s.groups[path]
	return ok
}

// EnvNames returns every environment variable the schema reads: values and file variants.
func (s *Schema) EnvNames() []string {
	var out []string
	for _, k := range s.keys {
		if k.Secret {
			out = append(out, k.EnvFileName())
		}
		out = append(out, k.envName())
	}
	slices.Sort(out)
	return out
}

var (
	defaultOnce   sync.Once
	defaultSchema *Schema
)

// Default returns the schema of the product: every key of the server. The
// declaration is validated once; an invalid declaration is a programmer bug
// that the tests of this package catch.
func Default() *Schema {
	defaultOnce.Do(func() {
		s, err := NewSchema(productKeys())
		if err != nil {
			panic(err)
		}
		defaultSchema = s
	})
	return defaultSchema
}
