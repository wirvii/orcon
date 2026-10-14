// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// DigestPrefix starts every digest: the digest of a package is
// "sha256:" and the lowercase hexadecimal SHA-256 of the archive bytes.
const DigestPrefix = "sha256:"

// Names of the files of a package (ADR-0029 section 2).
const (
	// ManifestFile holds the orcon/connector@1 manifest.
	ManifestFile = "orcon-package.json"
	// DeclarativeBodyFile holds the body of a declarative connector.
	DeclarativeBodyFile = "connector.json"
	// WasmModuleFile holds the module of a WASM connector.
	WasmModuleFile = "module.wasm"
)

// Limits that keep a hostile archive from exhausting memory. They are far above
// any real connector.
const (
	maxPackageFiles = 4096
	maxFileBytes    = 128 << 20
	maxPackageBytes = 256 << 20
	maxPathLength   = 200
)

// Digest is the identity of a package archive: "sha256:" and 64 lowercase
// hexadecimal digits.
type Digest string

// DigestOf returns the digest of archive bytes.
func DigestOf(archive []byte) Digest {
	sum := sha256.Sum256(archive)
	return Digest(DigestPrefix + hex.EncodeToString(sum[:]))
}

// PackageFile is one file of a package: a slash-separated path relative to the
// package root and its content.
type PackageFile struct {
	Path string
	Data []byte
}

// PackageError is returned when files do not make a valid package.
type PackageError struct {
	Problems []string
}

// Error implements error.
func (e *PackageError) Error() string {
	return "invalid connector package: " + strings.Join(e.Problems, "; ")
}

var (
	// ErrArchive means the bytes are not a canonical package archive: not zstd,
	// not tar, or tar entries that are not in the normalized form that makes one
	// content have one digest.
	ErrArchive = errors.New("not a canonical connector package archive")

	languageFile = regexp.MustCompile(`^i18n/[a-z]{2,3}(-[A-Z]{2})?\.json$`)
	changesFile  = regexp.MustCompile(`^changes/[0-9A-Za-z.+-]+\.\.[0-9A-Za-z.+-]+\.json$`)
	testsFile    = regexp.MustCompile(`^tests/[A-Za-z0-9._-]+\.json$`)
	iconFile     = regexp.MustCompile(`^icons/[A-Za-z0-9._-]+$`)
)

// fixedFiles are the files a package may carry at its root.
var fixedFiles = []string{
	ManifestFile, DeclarativeBodyFile, WasmModuleFile, "README.md", "LICENSE", "NOTICE", "ATTRIBUTION.json", "authoring.json",
}

// requiredFiles are the files every package carries (ADR-0029 section 2, R-CON-03).
var requiredFiles = []string{ManifestFile, "LICENSE", "NOTICE", "i18n/en.json", "i18n/es.json"}

// Package is a validated connector package: its manifest, its files and the
// digest of the archive it was read from or written to.
type Package struct {
	manifest Manifest
	files    map[string][]byte
	archive  []byte
	digest   Digest
}

// Manifest returns the manifest of the package.
func (p *Package) Manifest() *Manifest { return &p.manifest }

// Digest returns the digest of the archive.
func (p *Package) Digest() Digest { return p.digest }

// Archive returns the bytes of the archive. The caller must not change them.
func (p *Package) Archive() []byte { return p.archive }

// File returns the content of one file of the package.
func (p *Package) File(path string) ([]byte, bool) {
	data, ok := p.files[path]
	return data, ok
}

// Paths returns the paths of the files, in archive order.
func (p *Package) Paths() []string {
	paths := make([]string, 0, len(p.files))
	for path := range p.files {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

// BuildPackage validates a set of files as a connector package and writes the
// reproducible archive: a tar archive with entries sorted by path, normalized
// owners, modes and times, compressed with zstd at a fixed level. The same
// files in any order, built at any time on any machine, give the same bytes and
// so the same digest. The digest does not depend on file modes or times of the
// inputs, which the caller does not provide.
//
// The compressor is deterministic for one version of the zstd library and one
// set of options; a digest is always computed over the archive as shipped and
// never from a re-encoding, so installed digests do not change when the library
// does.
func BuildPackage(files []PackageFile, opts Options) (*Package, error) {
	byPath := make(map[string][]byte, len(files))
	var problems []string
	for _, f := range files {
		if _, dup := byPath[f.Path]; dup {
			problems = append(problems, fmt.Sprintf("file %q appears twice", f.Path))
			continue
		}
		byPath[f.Path] = f.Data
	}
	m, more := checkPackageFiles(byPath, opts)
	if problems = append(problems, more...); len(problems) > 0 {
		return nil, &PackageError{Problems: problems}
	}
	archive, err := writeArchive(byPath)
	if err != nil {
		return nil, err
	}
	return &Package{manifest: m, files: byPath, archive: archive, digest: DigestOf(archive)}, nil
}

// OpenPackage reads and validates a package archive. It accepts only the
// canonical form BuildPackage writes, so one content has one digest, and it
// limits how much it expands.
func OpenPackage(archive []byte, opts Options) (*Package, error) {
	files, err := readArchive(archive)
	if err != nil {
		return nil, err
	}
	m, problems := checkPackageFiles(files, opts)
	if len(problems) > 0 {
		return nil, &PackageError{Problems: problems}
	}
	return &Package{manifest: m, files: files, archive: bytes.Clone(archive), digest: DigestOf(archive)}, nil
}

// checkPackageFiles applies the rules of the package layout and returns the
// manifest.
func checkPackageFiles(files map[string][]byte, opts Options) (Manifest, []string) {
	var problems []string
	if len(files) > maxPackageFiles {
		return Manifest{}, []string{fmt.Sprintf("the package has %d files; at most %d are allowed", len(files), maxPackageFiles)}
	}
	var total int
	for path, data := range files {
		if msg := checkPath(path); msg != "" {
			problems = append(problems, msg)
		}
		if len(data) > maxFileBytes {
			problems = append(problems, fmt.Sprintf("file %q is larger than %d bytes", path, maxFileBytes))
		}
		total += len(data)
	}
	if total > maxPackageBytes {
		problems = append(problems, fmt.Sprintf("the package holds more than %d bytes", maxPackageBytes))
	}
	for _, name := range requiredFiles {
		if _, ok := files[name]; !ok {
			problems = append(problems, fmt.Sprintf("required file %q is missing", name))
		}
	}
	if len(problems) > 0 {
		return Manifest{}, problems
	}
	m, err := Parse(files[ManifestFile], opts)
	if err != nil {
		return Manifest{}, []string{fmt.Sprintf("%s: %v", ManifestFile, err)}
	}
	problems = append(problems, checkBody(files, &m)...)
	problems = append(problems, checkCatalogs(files)...)
	return m, problems
}

// checkPath returns why a path cannot be part of a package, or "".
func checkPath(path string) string {
	switch {
	case path == "" || len(path) > maxPathLength || !utf8.ValidString(path):
		return fmt.Sprintf("path %q is empty, too long or not UTF-8", path)
	case strings.HasPrefix(path, "/") || strings.Contains(path, `\`) || strings.Contains(path, "//") || strings.HasSuffix(path, "/"):
		return fmt.Sprintf("path %q must be relative with forward slashes", path)
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "." || seg == ".." || strings.ContainsFunc(seg, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
			return fmt.Sprintf("path %q has a segment that is not allowed", path)
		}
	}
	switch {
	case slices.Contains(fixedFiles, path),
		languageFile.MatchString(path),
		changesFile.MatchString(path),
		testsFile.MatchString(path),
		iconFile.MatchString(path),
		strings.HasPrefix(path, "src/"):
		return ""
	}
	return fmt.Sprintf("file %q is not part of the package layout", path)
}

// checkBody requires the body file the kind needs and rejects the other one.
func checkBody(files map[string][]byte, m *Manifest) []string {
	_, hasDecl := files[DeclarativeBodyFile]
	_, hasWasm := files[WasmModuleFile]
	var problems []string
	switch m.Kind {
	case KindDeclarative:
		if !hasDecl {
			problems = append(problems, "a DECLARATIVE package needs "+DeclarativeBodyFile)
		}
		if hasWasm {
			problems = append(problems, "a DECLARATIVE package has no "+WasmModuleFile)
		}
	case KindWASM:
		if !hasWasm {
			problems = append(problems, "a WASM package needs "+WasmModuleFile)
		}
		if hasDecl {
			problems = append(problems, "a WASM package has no "+DeclarativeBodyFile)
		}
	case KindNative:
		problems = append(problems, "a NATIVE connector is compiled into the binary and has no package")
	case KindLibrary, KindRuntime:
		if hasDecl {
			problems = append(problems, "a "+string(m.Kind)+" package has no "+DeclarativeBodyFile)
		}
	}
	if hasDecl {
		if _, err := jsonvalue.Parse(files[DeclarativeBodyFile]); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", DeclarativeBodyFile, err))
		}
	}
	return problems
}

// checkCatalogs requires every JSON file to be well formed, and the message
// catalogs to be objects.
func checkCatalogs(files map[string][]byte) []string {
	var problems []string
	for _, path := range sortedKeys(files) {
		if !strings.HasSuffix(path, ".json") || path == ManifestFile || path == DeclarativeBodyFile {
			continue
		}
		v, err := jsonvalue.Parse(files[path])
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		if languageFile.MatchString(path) && v.Kind() != jsonvalue.KindObject {
			problems = append(problems, path+": a message catalog is a JSON object of message keys")
		}
	}
	return problems
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// zeroTime is the modification time of every entry.
var zeroTime = time.Unix(0, 0)

func writeArchive(files map[string][]byte) ([]byte, error) {
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for _, path := range sortedKeys(files) {
		data := files[path]
		hdr := &tar.Header{
			Typeflag: tar.TypeReg, Name: path, Size: int64(len(data)), Mode: 0o644, ModTime: zeroTime, Format: tar.FormatPAX,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("write package entry %q: %w", path, err)
		}
		if _, err := tw.Write(data); err != nil {
			return nil, fmt.Errorf("write package entry %q: %w", path, err)
		}
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("finish package tar: %w", err)
	}
	var out bytes.Buffer
	enc, err := zstd.NewWriter(&out, zstd.WithEncoderLevel(zstd.SpeedDefault), zstd.WithEncoderConcurrency(1), zstd.WithEncoderCRC(true))
	if err != nil {
		return nil, fmt.Errorf("create package compressor: %w", err)
	}
	if _, err := enc.Write(tarBuf.Bytes()); err != nil {
		_ = enc.Close()
		return nil, fmt.Errorf("compress package: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("compress package: %w", err)
	}
	return out.Bytes(), nil
}

func readArchive(archive []byte) (map[string][]byte, error) {
	dec, err := zstd.NewReader(bytes.NewReader(archive), zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(maxPackageBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrArchive, err)
	}
	defer dec.Close()
	tr := tar.NewReader(io.LimitReader(dec, maxPackageBytes+(1<<20)))
	files := map[string][]byte{}
	previous := ""
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			if len(files) == 0 {
				return nil, fmt.Errorf("%w: it has no entries", ErrArchive)
			}
			return files, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrArchive, err)
		}
		if len(files) >= maxPackageFiles {
			return nil, fmt.Errorf("%w: more than %d entries", ErrArchive, maxPackageFiles)
		}
		if msg := canonicalEntry(hdr, previous); msg != "" {
			return nil, fmt.Errorf("%w: %s", ErrArchive, msg)
		}
		if hdr.Size > maxFileBytes {
			return nil, fmt.Errorf("%w: entry %q is larger than %d bytes", ErrArchive, hdr.Name, maxFileBytes)
		}
		data, err := io.ReadAll(io.LimitReader(tr, hdr.Size+1))
		if err != nil || int64(len(data)) != hdr.Size {
			return nil, fmt.Errorf("%w: entry %q is truncated", ErrArchive, hdr.Name)
		}
		files[hdr.Name] = data
		previous = hdr.Name
	}
}

// canonicalEntry returns why a tar entry is not in the normalized form
// BuildPackage writes, or "".
func canonicalEntry(hdr *tar.Header, previous string) string {
	switch {
	case hdr.Typeflag != tar.TypeReg:
		return fmt.Sprintf("entry %q is not a regular file", hdr.Name)
	case hdr.Name <= previous:
		return fmt.Sprintf("entry %q is out of order or repeated; entries are sorted by path", hdr.Name)
	case hdr.Mode != 0o644 || hdr.Uid != 0 || hdr.Gid != 0 || hdr.Uname != "" || hdr.Gname != "" || hdr.ModTime.Unix() != 0:
		return fmt.Sprintf("entry %q has an owner, mode or time; they are normalized to zero and 0644", hdr.Name)
	case len(hdr.PAXRecords) > 0 && !onlyPathRecords(hdr):
		return fmt.Sprintf("entry %q carries extended attributes", hdr.Name)
	}
	return ""
}

func onlyPathRecords(hdr *tar.Header) bool {
	for k := range hdr.PAXRecords {
		if k != "path" {
			return false
		}
	}
	return true
}
