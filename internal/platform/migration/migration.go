// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package migration holds the engine-neutral rules of the migration runner of
// ADR-0015 section 4: reading the embedded migration files and their header,
// the checksum, the compatibility floor and the decision whether a binary may
// run against a schema. The adapters apply the files; this package decides.
// It is a platform package: it imports no area or adapter and no database.
package migration

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Kinds a migration header may declare (orcon-db MG-6).
const (
	KindAdditive = "additive"
	KindExpand   = "expand"
	KindContract = "contract"
	KindRebuild  = "rebuild"
	KindData     = "data"
)

// FloorSelf is the header value meaning "the floor is the release that ships
// this migration" (ADR-0015 section 4).
const FloorSelf = "self"

// Migration is one embedded migration file.
type Migration struct {
	// Version is the number NNNN of the file name.
	Version int
	// Name is the file name without the directory, for example
	// "0001_create_instance_tables.sql".
	Name string
	// SQL is the file content, LF-normalized.
	SQL string
	// Checksum is the lowercase hexadecimal SHA-256 of the LF-normalized file,
	// the value listed in migrations.sum and stored in schema_migration.
	Checksum string
	// Kind is the value of "-- orcon:kind".
	Kind string
	// Floor is the value of "-- orcon:compatibility-floor": "self" or a
	// semantic version.
	Floor string
	// NoTransaction is true when the header carries "-- orcon:no-transaction"
	// (PostgreSQL only: one CREATE INDEX CONCURRENTLY).
	NoTransaction bool
}

// Applied is a row of schema_migration.
type Applied struct {
	Version       int
	Checksum      string
	BinaryVersion string
	Floor         string
}

var fileRE = regexp.MustCompile(`^(\d{4})_([a-z][a-z0-9]*(?:_[a-z0-9]+)*)\.sql$`)

// Checksum returns the SHA-256 of data after replacing CRLF with LF.
func Checksum(data []byte) string {
	sum := sha256.Sum256(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
	return hex.EncodeToString(sum[:])
}

// Load reads the migrations of dir in fsys. Files that are not migrations
// (migrations.sum, testdata) are ignored. Numbers must be contiguous from 1.
func Load(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations %s: %w", dir, err)
	}
	var out []Migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		m := fileRE.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migration %s: name does not match NNNN_<snake_description>.sql", e.Name())
		}
		raw, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", e.Name(), err)
		}
		n, _ := strconv.Atoi(m[1]) // four digits matched by the pattern
		mig := Migration{Version: n, Name: e.Name(), Checksum: Checksum(raw)}
		mig.SQL = string(bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n")))
		if err := parseHeader(&mig); err != nil {
			return nil, fmt.Errorf("migration %s: %w", e.Name(), err)
		}
		out = append(out, mig)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	for i, m := range out {
		if m.Version != i+1 {
			return nil, fmt.Errorf("migration numbers must be contiguous from 0001: expected %04d, found %04d", i+1, m.Version)
		}
	}
	return out, nil
}

func parseHeader(m *Migration) error {
	sc := bufio.NewScanner(strings.NewReader(m.SQL))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "--") {
			break
		}
		rest, ok := strings.CutPrefix(line, "-- orcon:")
		if !ok {
			continue
		}
		key, value, _ := strings.Cut(rest, " ")
		value = strings.TrimSpace(value)
		switch key {
		case "kind":
			m.Kind = value
		case "compatibility-floor":
			m.Floor = value
		case "no-transaction":
			m.NoTransaction = true
		case "contracts":
			// Read by tools/dbcheck; the runner has no use for it.
		default:
			return fmt.Errorf("unknown header key %q", key)
		}
	}
	switch m.Kind {
	case KindAdditive, KindExpand, KindContract, KindRebuild, KindData:
	default:
		return fmt.Errorf("header kind %q is not one of additive, expand, contract, rebuild, data", m.Kind)
	}
	if m.Floor != FloorSelf {
		if _, err := ParseVersion(m.Floor); err != nil {
			return fmt.Errorf("header compatibility-floor: %w", err)
		}
	}
	return nil
}

// RecordedFloor returns the floor to store in schema_migration for m applied
// by a binary of the given version: a declared floor is stored as written and
// "self" becomes the version of the binary that ships the migration.
func (m Migration) RecordedFloor(binaryVersion string) string {
	if m.Floor == FloorSelf {
		return binaryVersion
	}
	return m.Floor
}

// Version is a parsed semantic version.
type Version struct {
	Major, Minor, Patch uint64
	// Pre holds the dot-separated pre-release identifiers, empty for a release.
	Pre []string
	raw string
}

// String returns the text the version was parsed from.
func (v Version) String() string { return v.raw }

var versionRE = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

// ParseVersion reads a semantic version, with an optional leading "v" and
// build metadata (ignored for ordering).
func ParseVersion(s string) (Version, error) {
	m := versionRE.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("%q is not a semantic version (MAJOR.MINOR.PATCH)", s)
	}
	v := Version{raw: s}
	for i, dst := range []*uint64{&v.Major, &v.Minor, &v.Patch} {
		n, err := strconv.ParseUint(m[i+1], 10, 64)
		if err != nil {
			return Version{}, fmt.Errorf("%q: %w", s, err)
		}
		*dst = n
	}
	if m[4] != "" {
		v.Pre = strings.Split(m[4], ".")
	}
	return v, nil
}

// Compare orders two versions by semantic versioning 2.0.0 precedence: -1 when
// v is older than w, 0 when equal, 1 when newer.
func (v Version) Compare(w Version) int {
	for _, p := range [][2]uint64{{v.Major, w.Major}, {v.Minor, w.Minor}, {v.Patch, w.Patch}} {
		if c := cmpUint(p[0], p[1]); c != 0 {
			return c
		}
	}
	switch {
	case len(v.Pre) == 0 && len(w.Pre) == 0:
		return 0
	case len(v.Pre) == 0:
		return 1
	case len(w.Pre) == 0:
		return -1
	}
	for i := 0; i < len(v.Pre) && i < len(w.Pre); i++ {
		if c := comparePre(v.Pre[i], w.Pre[i]); c != 0 {
			return c
		}
	}
	return cmpUint(uint64(len(v.Pre)), uint64(len(w.Pre)))
}

func cmpUint(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func comparePre(a, b string) int {
	an, aerr := strconv.ParseUint(a, 10, 64)
	bn, berr := strconv.ParseUint(b, 10, 64)
	switch {
	case aerr == nil && berr == nil:
		return cmpUint(an, bn)
	case aerr == nil:
		return -1
	case berr == nil:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

// Sentinel errors of [Check]; the returned errors wrap them.
var (
	// ErrBinaryTooOld means the binary is older than the effective floor of the
	// schema (ADR-0015 section 4): it refuses to start.
	ErrBinaryTooOld = errors.New("binary is older than the schema compatibility floor")
	// ErrChecksumMismatch means an applied migration differs from the embedded
	// file of the same number.
	ErrChecksumMismatch = errors.New("applied migration differs from the embedded file")
	// ErrInvalidHistory means schema_migration holds a floor or a numbering the
	// runner cannot interpret.
	ErrInvalidHistory = errors.New("schema_migration history is invalid")
)

// FloorError is the refusal of a binary below the effective floor. Its
// message names the binary version, the schema version and the minimum binary
// version required.
type FloorError struct {
	BinaryVersion string
	SchemaVersion int
	Required      string
}

func (e *FloorError) Error() string {
	return fmt.Sprintf("this binary (version %s) is older than the database schema allows: the schema is at migration %04d and requires binary version %s or newer; upgrade the binary to %s or later",
		e.BinaryVersion, e.SchemaVersion, e.Required, e.Required)
}

// Unwrap lets errors.Is match [ErrBinaryTooOld].
func (e *FloorError) Unwrap() error { return ErrBinaryTooOld }

// ChecksumError names the migration whose applied checksum differs.
type ChecksumError struct {
	Version  int
	Name     string
	Applied  string
	Embedded string
}

func (e *ChecksumError) Error() string {
	return fmt.Sprintf("migration %s was applied with checksum %s but this binary embeds %s: an applied migration is never edited", e.Name, e.Applied, e.Embedded)
}

// Unwrap lets errors.Is match [ErrChecksumMismatch].
func (e *ChecksumError) Unwrap() error { return ErrChecksumMismatch }

// Plan is the outcome of [Check].
type Plan struct {
	// Pending lists the embedded migrations not yet applied, in order.
	Pending []Migration
	// SchemaVersion is the newest applied migration number (0 for an empty database).
	SchemaVersion int
	// EffectiveFloor is the maximum compatibility floor over the applied
	// migrations; the empty string for an empty database.
	EffectiveFloor string
}

// Check decides whether a binary of the given version may start against the
// schema described by applied, and which embedded migrations remain. It
// refuses when the binary is below the effective floor and when an applied
// migration that this binary also embeds has a different checksum. Applied
// migrations newer than the binary are allowed: a binary older than the schema
// but at or above the floor keeps running (rolling upgrade).
func Check(binaryVersion string, applied []Applied, embedded []Migration) (Plan, error) {
	bv, err := ParseVersion(binaryVersion)
	if err != nil {
		return Plan{}, fmt.Errorf("binary version: %w", err)
	}
	byVersion := make(map[int]Migration, len(embedded))
	for _, m := range embedded {
		byVersion[m.Version] = m
	}
	var plan Plan
	var floor Version
	for _, a := range applied {
		if a.Version > plan.SchemaVersion {
			plan.SchemaVersion = a.Version
		}
		fv, err := ParseVersion(a.Floor)
		if err != nil {
			return Plan{}, fmt.Errorf("migration %04d: %w: floor: %w", a.Version, ErrInvalidHistory, err)
		}
		if plan.EffectiveFloor == "" || fv.Compare(floor) > 0 {
			floor, plan.EffectiveFloor = fv, a.Floor
		}
		if m, known := byVersion[a.Version]; known && m.Checksum != a.Checksum {
			return Plan{}, &ChecksumError{Version: a.Version, Name: m.Name, Applied: a.Checksum, Embedded: m.Checksum}
		}
	}
	if plan.EffectiveFloor != "" && bv.Compare(floor) < 0 {
		return Plan{}, &FloorError{BinaryVersion: binaryVersion, SchemaVersion: plan.SchemaVersion, Required: plan.EffectiveFloor}
	}
	seen := make(map[int]bool, len(applied))
	for _, a := range applied {
		seen[a.Version] = true
	}
	for i := 1; i <= plan.SchemaVersion; i++ {
		if !seen[i] {
			return Plan{}, fmt.Errorf("migration %04d is missing from schema_migration (newest is %04d): %w", i, plan.SchemaVersion, ErrInvalidHistory)
		}
	}
	for _, m := range embedded {
		if m.Version > plan.SchemaVersion {
			plan.Pending = append(plan.Pending, m)
		}
	}
	return plan, nil
}
