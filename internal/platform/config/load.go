// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"
	"slices"
	"strings"
)

// maxFileSize bounds the configuration file and every secret file. Both are
// small by nature; the bound keeps a wrong path (a log, a device) from being read whole.
const maxFileSize = 1 << 20

// Severity ranks an issue.
type Severity int

// The severities of an issue.
const (
	// SeverityWarning does not stop the server from starting.
	SeverityWarning Severity = iota
	// SeverityError makes the configuration invalid.
	SeverityError
)

// String returns ERROR or WARN, the labels of orcon config check.
func (s Severity) String() string {
	if s == SeverityError {
		return "ERROR"
	}
	return "WARN"
}

// Issue is one problem found in the configuration. Its message never contains a secret value.
type Issue struct {
	// Severity is error or warning.
	Severity Severity
	// Key is the key path the issue is about, or the name of an environment variable or file when no key applies.
	Key string
	// Message says what is wrong and how to fix it.
	Message string
}

// SourceKind says where a value came from.
type SourceKind int

// The places a value can come from.
const (
	// SourceDefault is the built-in default.
	SourceDefault SourceKind = iota
	// SourceFlag is a command-line flag.
	SourceFlag
	// SourceEnv is an ORCON_* environment variable.
	SourceEnv
	// SourceFile is the YAML configuration file.
	SourceFile
	// SourceSecretFile is a file that holds a secret; Via says which setting named it.
	SourceSecretFile
)

// Source is the origin of the effective value of a key (R-CONF-07).
type Source struct {
	// Kind is the place the value came from.
	Kind SourceKind
	// Env is the variable name when Kind is SourceEnv.
	Env string
	// Path is the file path when Kind is SourceFile or SourceSecretFile.
	Path string
	// Line is the line in the configuration file when Kind is SourceFile.
	Line int
	// Via is the setting that named the secret file when Kind is SourceSecretFile.
	Via *Source
}

// String renders the source as flag, env:<NAME>, file:<path>:<line> or default.
// A secret read from a file shows the file and, in parentheses, the setting that named it.
func (s Source) String() string {
	switch s.Kind {
	case SourceDefault:
		return "default"
	case SourceFlag:
		return "flag"
	case SourceEnv:
		return "env:" + s.Env
	case SourceFile:
		if s.Line > 0 {
			return fmt.Sprintf("file:%s:%d", s.Path, s.Line)
		}
		return "file:" + s.Path
	case SourceSecretFile:
		if s.Via == nil {
			return "file:" + s.Path
		}
		return "file:" + s.Path + " (" + s.Via.String() + ")"
	default:
		return "unknown"
	}
}

// Entry is the effective setting of one key.
type Entry struct {
	// Key is the declaration.
	Key *Key
	// Value is the resolved value. A secret prints as [REDACTED].
	Value Value
	// Source says where Value came from.
	Source Source
	// Invalid is true when the value failed validation; Value is then the default or empty.
	Invalid bool
}

// Config is the resolved configuration of a process.
type Config struct {
	schema  *Schema
	entries map[string]*Entry
	file    string
	// inlineSecrets lists the secret keys whose value is written in the configuration file itself.
	inlineSecrets []string
	fileMode      fs.FileMode
	platform      Platform
}

// Schema returns the schema the configuration was resolved against.
func (c *Config) Schema() *Schema { return c.schema }

// File returns the path of the configuration file, or "" when none was given.
func (c *Config) File() string { return c.file }

// Entries returns every key, in schema order.
func (c *Config) Entries() []Entry {
	out := make([]Entry, 0, len(c.schema.keys))
	for _, k := range c.schema.keys {
		out = append(out, *c.entries[k.Path])
	}
	return out
}

// Value returns the resolved value of a key. An unknown path yields the zero Value.
func (c *Config) Value(path string) Value {
	if e, ok := c.entries[path]; ok {
		return e.Value
	}
	return Value{}
}

// Source returns where the value of a key came from.
func (c *Config) Source(path string) Source {
	if e, ok := c.entries[path]; ok {
		return e.Source
	}
	return Source{}
}

// Role returns the configured role.
func (c *Config) Role() Role { return Role(c.Value("role").Text()) }

// Secret returns the value of a secret key.
func (c *Config) Secret(path string) Secret { return c.Value(path).Secret() }

// Result is the outcome of [Schema.Load]: a configuration and everything that is wrong with it.
type Result struct {
	// Config is always set. When there are errors it holds a best-effort view for diagnostics only.
	Config *Config
	// Issues lists errors and warnings, in a stable order.
	Issues []Issue
}

// HasErrors reports whether any issue is an error.
func (r *Result) HasErrors() bool {
	return slices.ContainsFunc(r.Issues, func(i Issue) bool { return i.Severity == SeverityError })
}

// ValidationError is returned by [Result.Err].
type ValidationError struct {
	// Issues are the errors, without the warnings.
	Issues []Issue
}

// Error lists the errors, one per line.
func (e *ValidationError) Error() string {
	var b strings.Builder
	b.WriteString("invalid configuration")
	for _, i := range e.Issues {
		b.WriteString("\n  ")
		b.WriteString(i.Key)
		b.WriteString(": ")
		b.WriteString(i.Message)
	}
	return b.String()
}

// Err returns a [*ValidationError] when there is at least one error, and nil otherwise.
func (r *Result) Err() error {
	var errs []Issue
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			errs = append(errs, i)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{Issues: errs}
}

// FileSystem is the part of the file system the loader reads.
type FileSystem interface {
	// ReadFile returns the content of the named file, at most limit+1 bytes.
	ReadFile(name string, limit int64) ([]byte, error)
	// Stat returns the information of the named file.
	Stat(name string) (fs.FileInfo, error)
}

// OSFileSystem reads the real file system.
type OSFileSystem struct{}

// ReadFile reads at most limit+1 bytes, so the caller can tell that a file is too large.
func (OSFileSystem) ReadFile(name string, limit int64) ([]byte, error) {
	f, err := os.Open(name) //nolint:gosec // G304: the operator names the file on purpose
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	defer func() { _ = f.Close() }() // read-only file: nothing is lost if Close fails
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return data, nil
}

// Stat returns the information of the named file.
func (OSFileSystem) Stat(name string) (fs.FileInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", name, err)
	}
	return info, nil
}

// Input is everything Load reads. Nothing is taken from the process: a test, the
// command-line client and the server each pass what they have.
type Input struct {
	// Flags are the values parsed from the command line. Nil means no flags.
	Flags *FlagValues
	// Environ is the environment in os.Environ form (NAME=value).
	Environ []string
	// FS reads the configuration file and the secret files. Nil means the real file system.
	FS FileSystem
	// Decode parses the YAML configuration file. It may be nil when no file is expected.
	Decode FileDecoder
	// GOOS names the operating system, as runtime.GOOS does. Empty means the running one.
	GOOS string
}

// Load resolves every key: flag, then environment, then configuration file,
// then default, key by key (R-CONF-01). It reads files but opens no network
// connection and no database.
func (s *Schema) Load(in Input) *Result {
	l := newLoader(s, in)
	l.run()
	return &Result{Config: l.cfg, Issues: l.sortedIssues()}
}

type loader struct {
	schema  *Schema
	in      Input
	env     map[string]string
	cfg     *Config
	issues  []Issue
	fileEnt map[string][]FileEntry // by key path or file variant path
	used    map[string]bool
}

func newLoader(s *Schema, in Input) *loader {
	if in.FS == nil {
		in.FS = OSFileSystem{}
	}
	if in.GOOS == "" {
		in.GOOS = runtime.GOOS
	}
	env := make(map[string]string, len(in.Environ))
	for _, kv := range in.Environ {
		if name, value, ok := strings.Cut(kv, "="); ok {
			env[name] = value
		}
	}
	l := &loader{schema: s, in: in, env: env, used: map[string]bool{}}
	l.cfg = &Config{
		schema:   s,
		entries:  make(map[string]*Entry, len(s.keys)),
		platform: Platform{GOOS: in.GOOS, Getenv: func(name string) string { return env[name] }},
	}
	return l
}

func (l *loader) errorf(key, format string, args ...any) {
	l.issues = append(l.issues, Issue{Severity: SeverityError, Key: key, Message: fmt.Sprintf(format, args...)})
}

func (l *loader) warnf(key, format string, args ...any) {
	l.issues = append(l.issues, Issue{Severity: SeverityWarning, Key: key, Message: fmt.Sprintf(format, args...)})
}

func (l *loader) run() {
	l.loadFile()
	for _, k := range l.schema.keys {
		l.resolve(k)
	}
	l.reportUnknownFileEntries()
	l.reportUnknownEnv()
	l.crossValidate()
}

func (l *loader) sortedIssues() []Issue {
	out := slices.Clone(l.issues)
	// Errors first, then warnings; stable, so the order of discovery (schema order) holds inside a class.
	slices.SortStableFunc(out, func(a, b Issue) int { return int(b.Severity) - int(a.Severity) })
	return out
}

// configPath returns the configuration file path from --config or ORCON_CONFIG.
func (l *loader) configPath() (path string, src Source) {
	if l.in.Flags != nil && l.in.Flags.Config != "" {
		return l.in.Flags.Config, Source{Kind: SourceFlag}
	}
	if v := l.env["ORCON_CONFIG"]; v != "" {
		return v, Source{Kind: SourceEnv, Env: "ORCON_CONFIG"}
	}
	return "", Source{}
}

func (l *loader) loadFile() {
	l.fileEnt = map[string][]FileEntry{}
	path, _ := l.configPath()
	if path == "" {
		return
	}
	l.cfg.file = path
	data, err := l.in.FS.ReadFile(path, maxFileSize)
	if err != nil {
		l.errorf("config", "cannot read the configuration file %s: %s", path, reason(err))
		return
	}
	if len(data) > maxFileSize {
		l.errorf("config", "the configuration file %s is larger than %d bytes", path, maxFileSize)
		return
	}
	if info, err := l.in.FS.Stat(path); err == nil {
		l.cfg.fileMode = info.Mode()
	}
	if l.in.Decode == nil {
		l.errorf("config", "this build cannot read configuration files")
		return
	}
	entries, err := l.in.Decode(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), l.schema.IsLeaf)
	if err != nil {
		l.errorf("config", "%s: %v", path, err)
	}
	for _, e := range entries {
		l.fileEnt[e.Path] = append(l.fileEnt[e.Path], e)
	}
}

// layer is a place a key can be set at.
type layer int

const (
	layerFlag layer = iota
	layerEnv
	layerFile
)

// setting is the input a layer offers for one key: a value or the path of a file.
type setting struct {
	raw    raw
	src    Source
	isFile bool
	// filePath is the secret file when isFile is true.
	filePath string
}

// raw is an unparsed value: text, or the items of a YAML sequence or mapping.
type raw struct {
	text   string
	list   []string
	pairs  map[string]string
	shaped bool // true when list or pairs carry the value
}

func (l *loader) resolve(k *Key) {
	e := &Entry{Key: k, Source: Source{Kind: SourceDefault}}
	l.cfg.entries[k.Path] = e
	// A file entry that a higher layer overrides is still a known key.
	l.used[k.Path] = true
	if k.Secret {
		l.used[k.FileKey()] = true
	}
	for _, ly := range []layer{layerFlag, layerEnv, layerFile} {
		value, file := l.at(k, ly)
		switch {
		case value != nil && file != nil:
			l.errorf(k.Path, "set twice at the same level (%s and %s); set only one", value.src, file.src)
			e.Invalid = true
			return
		case value != nil:
			l.apply(e, *value)
			return
		case file != nil:
			l.apply(e, *file)
			return
		}
	}
	l.applyDefault(e)
}

func (l *loader) at(k *Key, ly layer) (value, file *setting) {
	switch ly {
	case layerFlag:
		return l.atFlag(k)
	case layerEnv:
		return l.atEnv(k)
	case layerFile:
		return l.atFile(k)
	default:
		return nil, nil
	}
}

func (l *loader) atFlag(k *Key) (value, file *setting) {
	fv := l.in.Flags
	if fv == nil {
		return nil, nil
	}
	if text, ok := fv.Values[k.Path]; ok && !k.Secret {
		value = &setting{raw: raw{text: text}, src: Source{Kind: SourceFlag}}
	}
	if path, ok := fv.Files[k.Path]; ok && k.Secret {
		file = &setting{isFile: true, filePath: path, src: Source{Kind: SourceFlag}}
	}
	return value, file
}

func (l *loader) atEnv(k *Key) (value, file *setting) {
	if text := l.env[k.envName()]; text != "" {
		value = &setting{raw: raw{text: text}, src: Source{Kind: SourceEnv, Env: k.envName()}}
		l.used[k.envName()] = true
	}
	if k.Secret {
		if path := l.env[k.EnvFileName()]; path != "" {
			file = &setting{isFile: true, filePath: path, src: Source{Kind: SourceEnv, Env: k.EnvFileName()}}
			l.used[k.EnvFileName()] = true
		}
	}
	return value, file
}

func (l *loader) atFile(k *Key) (value, file *setting) {
	if es := l.fileEnt[k.Path]; len(es) > 0 {
		l.used[k.Path] = true
		if e := es[0]; e.Kind != EntryNull {
			value = &setting{raw: fileRaw(e), src: Source{Kind: SourceFile, Path: l.cfg.file, Line: e.Line}}
		}
	}
	if k.Secret {
		if es := l.fileEnt[k.FileKey()]; len(es) > 0 {
			l.used[k.FileKey()] = true
			if e := es[0]; e.Kind != EntryNull {
				file = &setting{
					isFile: true, filePath: e.Scalar,
					src: Source{Kind: SourceFile, Path: l.cfg.file, Line: e.Line},
				}
				if e.Kind != EntryScalar {
					l.errorf(k.FileKey(), "must be the path of a file")
					file = nil
				}
			}
		}
	}
	return value, file
}

func fileRaw(e FileEntry) raw {
	switch e.Kind {
	case EntryList:
		return raw{list: e.List, shaped: true}
	case EntryMap:
		return raw{pairs: e.Map, shaped: true}
	case EntryScalar, EntryNull:
		return raw{text: e.Scalar}
	}
	return raw{text: e.Scalar}
}

func (l *loader) apply(e *Entry, s setting) {
	k := e.Key
	e.Source = s.src
	text := s.raw.text
	if s.isFile {
		content, ok := l.readSecretFile(k, s)
		if !ok {
			e.Invalid = true
			return
		}
		text = content
		via := s.src
		e.Source = Source{Kind: SourceSecretFile, Path: s.filePath, Via: &via}
		s.raw = raw{text: text}
	} else if k.Secret && s.src.Kind == SourceFile {
		l.cfg.inlineSecrets = append(l.cfg.inlineSecrets, k.Path)
	}
	var v Value
	var err error
	if s.raw.shaped {
		v, err = l.parseShaped(k, s.raw)
	} else {
		v, err = parseValue(k, text)
	}
	if err != nil {
		l.errorf(k.Path, "%s: %v", describeSource(e.Source), err)
		e.Invalid = true
		return
	}
	e.Value = v
}

func describeSource(s Source) string {
	return "invalid value from " + s.String()
}

func (l *loader) parseShaped(k *Key, r raw) (Value, error) {
	switch k.Kind {
	case KindList:
		if r.pairs != nil {
			return Value{}, errors.New("must be a list, not a mapping")
		}
		return parseItems(k, r.list, nil)
	case KindMap:
		if r.list != nil {
			return Value{}, errors.New("must be a mapping, not a list")
		}
		return parseItems(k, nil, r.pairs)
	case KindString, KindInt, KindFloat, KindBool, KindDuration, KindEnum, KindPath, KindURL, KindAddress, KindBytes:
		return Value{}, errors.New("must be a single value, not a list or a mapping")
	}
	return Value{}, errors.New("unsupported value")
}

func (l *loader) applyDefault(e *Entry) {
	k := e.Key
	text := k.Default
	if k.DefaultFunc != nil {
		get := func(path string) string {
			dep, ok := l.cfg.entries[path]
			if !ok {
				l.errorf(k.Path, "internal: the default reads %s before it is resolved", path)
				return ""
			}
			return dep.Value.plain()
		}
		var err error
		text, err = k.DefaultFunc(l.cfg.platform, get)
		if err != nil {
			l.errorf(k.Path, "no default is available: %v", err)
			e.Invalid = true
			return
		}
	}
	v, err := parseValue(k, text)
	if err != nil {
		l.errorf(k.Path, "internal: the default is invalid: %v", err)
		e.Invalid = true
		return
	}
	e.Value = v
}

func (l *loader) readSecretFile(k *Key, s setting) (string, bool) {
	path := s.filePath
	if path == "" {
		l.errorf(k.Path, "the file path from %s is empty", s.src)
		return "", false
	}
	data, err := l.in.FS.ReadFile(path, maxFileSize)
	if err != nil {
		l.errorf(k.Path, "cannot read the secret file %s named by %s: %s", path, s.src, reason(err))
		return "", false
	}
	if len(data) > maxFileSize {
		l.errorf(k.Path, "the secret file %s is larger than %d bytes", path, maxFileSize)
		return "", false
	}
	if l.in.GOOS != "windows" {
		if info, err := l.in.FS.Stat(path); err == nil && othersCanRead(info.Mode()) {
			l.warnf(k.Path, "the secret file %s is readable by other users; restrict it to the user that runs Orcon (chmod 0400)", path)
		}
	}
	return trimOneNewline(string(data)), true
}

// reason returns the cause of a file error without repeating the path, which the message already names.
func reason(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

// othersCanRead reports whether the "other" permission bits allow reading.
func othersCanRead(mode fs.FileMode) bool { return mode.Perm()&0o004 != 0 }

// trimOneNewline removes a single trailing line break (R-CONF-06).
func trimOneNewline(s string) string {
	if t, ok := strings.CutSuffix(s, "\r\n"); ok {
		return t
	}
	return strings.TrimSuffix(s, "\n")
}

func (l *loader) reportUnknownFileEntries() {
	paths := make([]string, 0, len(l.fileEnt))
	for p := range l.fileEnt {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		if l.used[p] {
			continue
		}
		for _, e := range l.fileEnt[p] {
			where := fmt.Sprintf("%s:%d", l.cfg.file, e.Line)
			if l.schema.IsSection(p) {
				l.errorf(p, "%s: %q is a section of keys, not a value; set the keys inside it", where, p)
				continue
			}
			l.errorf(p, "%s: unknown key %q", where, p)
		}
	}
}

// reportUnknownEnv warns about ORCON_* variables no key and no client setting uses. It is a warning because the client shares the prefix.
func (l *loader) reportUnknownEnv() {
	known := map[string]bool{}
	for _, n := range l.schema.EnvNames() {
		known[n] = true
	}
	for _, n := range ClientEnvNames() {
		known[n] = true
	}
	var unknown []string
	for name := range l.env {
		if strings.HasPrefix(name, EnvPrefix) && !known[name] {
			unknown = append(unknown, name)
		}
	}
	slices.Sort(unknown)
	for _, name := range unknown {
		l.warnf(name, "unknown environment variable; it is ignored")
	}
}
