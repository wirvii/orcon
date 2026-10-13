// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

// FileEntryKind is the YAML shape of a value in the configuration file.
type FileEntryKind int

// The shapes a configuration file value can have.
const (
	// EntryScalar is a single value.
	EntryScalar FileEntryKind = iota
	// EntryList is a sequence of scalars.
	EntryList
	// EntryMap is a mapping of scalars.
	EntryMap
	// EntryNull is an empty value (a key with nothing after the colon); it counts as not set.
	EntryNull
)

// FileEntry is one value read from the configuration file. A decoder flattens
// the document: nested sections become dotted paths.
type FileEntry struct {
	// Path is the dotted path of the value.
	Path string
	// Line is the 1-based line of the key in the file.
	Line int
	// Kind is the shape of the value.
	Kind FileEntryKind
	// Scalar is the text of a scalar value.
	Scalar string
	// List holds the items of a sequence.
	List []string
	// Map holds the members of a mapping.
	Map map[string]string
}

// FileDecoder parses the YAML configuration file. It lives in
// internal/adapter/codec/configyaml and is passed in by the composition root, so this
// package does not depend on a YAML library (SPEC-008 AL-7). The function isLeaf tells the decoder
// which paths hold a value, so a mapping at such a path is a map value and not a section.
type FileDecoder func(data []byte, isLeaf func(path string) bool) ([]FileEntry, error)

// FlagValues holds what the command line set. It is filled by flag.FlagSet.Parse
// after [Schema.BindFlags].
type FlagValues struct {
	// Config is the value of --config.
	Config string
	// Values holds the text of every flag that carries a value, by key path.
	Values map[string]string
	// Files holds the path given to every --<key>-file flag, by key path.
	Files map[string]string
}

// ErrValueAsFlag is returned when a secret value is given on the command line.
var ErrValueAsFlag = errors.New("a secret value cannot be passed as a flag")

type flagVar struct {
	dst     map[string]string
	path    string
	boolean bool
}

func (f *flagVar) String() string { return "" }

func (f *flagVar) Set(v string) error {
	f.dst[f.path] = v
	return nil
}

// IsBoolFlag lets --flag stand for --flag=true.
func (f *flagVar) IsBoolFlag() bool { return f.boolean }

// BindFlags registers a flag for every key on fs and returns the holder that
// Parse fills. A secret key gets only a --<key>-file flag: its value is never a
// flag (R-CONF-05). The command flag --config is registered too.
func (s *Schema) BindFlags(fs *flag.FlagSet) *FlagValues {
	fv := &FlagValues{Values: map[string]string{}, Files: map[string]string{}}
	fs.StringVar(&fv.Config, "config", "", "path of the YAML configuration file (or ORCON_CONFIG)")
	for _, k := range s.keys {
		desc := k.Description.EN
		if k.Secret {
			fs.Var(&flagVar{dst: fv.Files, path: k.Path}, k.FlagFileName(), "path of a file holding the secret: "+desc)
			continue
		}
		fs.Var(&flagVar{dst: fv.Values, path: k.Path, boolean: k.Kind == KindBool}, k.FlagName(), desc)
	}
	return fv
}

// ParseFlags parses args into fs, which BindFlags prepared. Before parsing it
// rejects a secret key given by value, so the secret never reaches an error message.
func (s *Schema) ParseFlags(fs *flag.FlagSet, args []string) error {
	if err := s.CheckSecretFlags(args); err != nil {
		return err
	}
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	return nil
}

// CheckSecretFlags returns [ErrValueAsFlag] when args name the value flag of a
// secret key. The error names the flag and the file flag to use, never the value.
func (s *Schema) CheckSecretFlags(args []string) error {
	secret := map[string]*Key{}
	for _, k := range s.keys {
		if k.Secret {
			secret[k.FlagName()] = k
		}
	}
	for _, arg := range args {
		if arg == "--" {
			return nil
		}
		name, ok := strings.CutPrefix(arg, "--")
		if !ok {
			name, ok = strings.CutPrefix(arg, "-")
		}
		if !ok {
			continue
		}
		name, _, _ = strings.Cut(name, "=")
		if k, isSecret := secret[name]; isSecret {
			return fmt.Errorf("--%s: %w (it would show in the process list); use --%s with the path of a file", name, ErrValueAsFlag, k.FlagFileName())
		}
	}
	return nil
}
