// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/wirvii/orcon/internal/connector"
)

// referenceFiles are the files of a complete package of the reference manifest.
func referenceFiles(manifest string) []connector.PackageFile {
	return []connector.PackageFile{
		{Path: "orcon-package.json", Data: []byte(manifest)},
		{Path: "connector.json", Data: []byte(`{"format":"orcon/declarative@1","http":{"baseUrl":"https://api.shop.example.com"}}`)},
		{Path: "LICENSE", Data: []byte("Apache License 2.0\n")},
		{Path: "NOTICE", Data: []byte("Acme Shop\n")},
		{Path: "i18n/en.json", Data: []byte(`{"node.order.displayName":"Order"}`)},
		{Path: "i18n/es.json", Data: []byte(`{"node.order.displayName":"Pedido"}`)},
		{Path: "README.md", Data: []byte("# Acme Shop\n")},
		{Path: "icons/icon.svg", Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"/>`)},
		{Path: "tests/order.get.ok.json", Data: []byte(`{"operation":"order.get"}`)},
	}
}

func buildOK(t *testing.T, files []connector.PackageFile) *connector.Package {
	t.Helper()
	p, err := connector.BuildPackage(files, connector.Options{})
	if err != nil {
		t.Fatalf("BuildPackage() failed: %v", err)
	}
	return p
}

// TestPackageReproducible: the same content gives the same archive and digest
// whatever the order the files are given in or the goroutine that builds it,
// and the build reads no clock: every entry carries the zero time (ADR-0029
// section 2, R-CON-03).
func TestPackageReproducible(t *testing.T) {
	t.Parallel()
	want := buildOK(t, referenceFiles(referenceManifest))
	if !strings.HasPrefix(string(want.Digest()), connector.DigestPrefix) || len(want.Digest()) != 71 {
		t.Fatalf("Digest() = %q, want sha256: and 64 hexadecimal digits", want.Digest())
	}
	if got := connector.DigestOf(want.Archive()); got != want.Digest() {
		t.Errorf("DigestOf(Archive()) = %s, want %s", got, want.Digest())
	}

	t.Run("any file order", func(t *testing.T) {
		t.Parallel()
		n := len(referenceFiles(referenceManifest))
		for shift := range n {
			for _, reversed := range []bool{false, true} {
				files := referenceFiles(referenceManifest)
				files = append(files[shift:], files[:shift]...)
				if reversed {
					slices.Reverse(files)
				}
				if got := buildOK(t, files).Digest(); got != want.Digest() {
					t.Fatalf("digest of a rotated build (shift %d, reversed %v) = %s, want %s", shift, reversed, got, want.Digest())
				}
			}
		}
	})

	t.Run("concurrent builds", func(t *testing.T) {
		t.Parallel()
		var wg sync.WaitGroup
		digests := make([]connector.Digest, 8)
		for i := range digests {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p, err := connector.BuildPackage(referenceFiles(referenceManifest), connector.Options{})
				if err == nil {
					digests[i] = p.Digest()
				}
			}()
		}
		wg.Wait()
		for i, d := range digests {
			if d != want.Digest() {
				t.Errorf("concurrent build %d digest = %q, want %s", i, d, want.Digest())
			}
		}
	})

	t.Run("different content, different digest", func(t *testing.T) {
		t.Parallel()
		files := referenceFiles(referenceManifest)
		files[2].Data = []byte("Apache License 2.0 (changed)\n")
		if buildOK(t, files).Digest() == want.Digest() {
			t.Error("changing a file did not change the digest")
		}
	})
}

func TestPackageRoundTrip(t *testing.T) {
	t.Parallel()
	built := buildOK(t, referenceFiles(referenceManifest))
	opened, err := connector.OpenPackage(built.Archive(), connector.Options{})
	if err != nil {
		t.Fatalf("OpenPackage() failed: %v", err)
	}
	if opened.Digest() != built.Digest() {
		t.Errorf("digest after opening = %s, want %s", opened.Digest(), built.Digest())
	}
	if opened.Manifest().ID != "acme.shop" {
		t.Errorf("manifest id = %q", opened.Manifest().ID)
	}
	data, ok := opened.File("i18n/es.json")
	if !ok || string(data) != `{"node.order.displayName":"Pedido"}` {
		t.Errorf("File(i18n/es.json) = %q, %v", data, ok)
	}
	paths := opened.Paths()
	if len(paths) != 9 || paths[0] != "LICENSE" {
		t.Errorf("Paths() = %v", paths)
	}
}

func TestPackageRejectsBadLayouts(t *testing.T) {
	t.Parallel()
	without := func(path string) []connector.PackageFile {
		var out []connector.PackageFile
		for _, f := range referenceFiles(referenceManifest) {
			if f.Path != path {
				out = append(out, f)
			}
		}
		return out
	}
	with := func(extra ...connector.PackageFile) []connector.PackageFile {
		return append(referenceFiles(referenceManifest), extra...)
	}
	tests := []struct {
		name  string
		files []connector.PackageFile
		want  string
	}{
		{"no manifest", without("orcon-package.json"), `required file "orcon-package.json"`},
		{"no license", without("LICENSE"), `required file "LICENSE"`},
		{"no notice", without("NOTICE"), `required file "NOTICE"`},
		{"no English catalog", without("i18n/en.json"), `required file "i18n/en.json"`},
		{"no Spanish catalog", without("i18n/es.json"), `required file "i18n/es.json"`},
		{"no declarative body", without("connector.json"), "needs connector.json"},
		{"a WASM module in a declarative package", with(connector.PackageFile{Path: "module.wasm", Data: []byte{0, 'a', 's', 'm'}}), "no module.wasm"},
		{"a path that climbs out", with(connector.PackageFile{Path: "src/../../etc/passwd", Data: []byte("x")}), "segment that is not allowed"},
		{"an absolute path", with(connector.PackageFile{Path: "/etc/passwd", Data: []byte("x")}), "must be relative"},
		{"a path with a backslash", with(connector.PackageFile{Path: `src\a.go`, Data: []byte("x")}), "must be relative"},
		{"a file outside the layout", with(connector.PackageFile{Path: "run.sh", Data: []byte("x")}), "not part of the package layout"},
		{"a file twice", with(connector.PackageFile{Path: "README.md", Data: []byte("again")}), `"README.md" appears twice`},
		{"a catalog that is not an object", append(without("i18n/en.json"), connector.PackageFile{Path: "i18n/en.json", Data: []byte(`[]`)}), "a JSON object of message keys"},
		{"a JSON file that is not JSON", with(connector.PackageFile{Path: "authoring.json", Data: []byte(`{`)}), "authoring.json"},
		{"an invalid manifest", referenceFiles(strings.Replace(referenceManifest, `"kind": "DECLARATIVE"`, `"kind": "declarative"`, 1)), "UPPER_SNAKE"},
		{"a native connector", referenceFiles(strings.Replace(referenceManifest, `"kind": "DECLARATIVE"`, `"kind": "NATIVE"`, 1)), "no package"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := connector.BuildPackage(tt.files, connector.Options{})
			var pe *connector.PackageError
			if !errors.As(err, &pe) {
				t.Fatalf("BuildPackage() error = %v, want a *PackageError", err)
			}
			if !strings.Contains(pe.Error(), tt.want) {
				t.Errorf("error %q does not mention %q", pe, tt.want)
			}
		})
	}
}

func TestPackageRejectsReservedNamespace(t *testing.T) {
	t.Parallel()
	text := strings.ReplaceAll(strings.ReplaceAll(referenceManifest, `"id": "acme.shop"`, `"id": "orcon.shop"`), `acme.shop.`, `orcon.shop.`)
	if _, err := connector.BuildPackage(referenceFiles(text), connector.Options{}); err == nil {
		t.Error("an archive in the reserved orcon. namespace was accepted")
	}
}

// archive writes a tar with the given entries, compressed with zstd, the way
// a hostile or careless tool might.
func archive(t *testing.T, entries []tar.Header, bodies [][]byte) []byte {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for i := range entries {
		h := entries[i]
		h.Size = int64(len(bodies[i]))
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(bodies[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	enc, err := zstd.NewWriter(&out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enc.Write(tarBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestOpenPackageOnlyAcceptsTheCanonicalForm(t *testing.T) {
	t.Parallel()
	reg := func(name string) tar.Header {
		return tar.Header{Typeflag: tar.TypeReg, Name: name, Mode: 0o644, ModTime: time.Unix(0, 0)}
	}
	tests := []struct {
		name string
		data []byte
	}{
		{"not zstd", []byte("this is not an archive")},
		{"empty", nil},
		{"a non-zero modification time", func() []byte {
			h := reg("LICENSE")
			h.ModTime = time.Unix(1_700_000_000, 0)
			return archive(t, []tar.Header{h}, [][]byte{[]byte("x")})
		}()},
		{"a mode of 0755", func() []byte {
			h := reg("LICENSE")
			h.Mode = 0o755
			return archive(t, []tar.Header{h}, [][]byte{[]byte("x")})
		}()},
		{"an owner", func() []byte {
			h := reg("LICENSE")
			h.Uname = "root"
			return archive(t, []tar.Header{h}, [][]byte{[]byte("x")})
		}()},
		{"entries out of order", archive(t, []tar.Header{reg("NOTICE"), reg("LICENSE")}, [][]byte{[]byte("x"), []byte("y")})},
		{"a symbolic link", func() []byte {
			h := tar.Header{Typeflag: tar.TypeSymlink, Name: "LICENSE", Linkname: "/etc/passwd", Mode: 0o644, ModTime: time.Unix(0, 0)}
			return archive(t, []tar.Header{h}, [][]byte{nil})
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := connector.OpenPackage(tt.data, connector.Options{})
			if !errors.Is(err, connector.ErrArchive) {
				t.Errorf("OpenPackage() error = %v, want ErrArchive", err)
			}
		})
	}
}

func TestOpenPackageLimitsWhatItExpands(t *testing.T) {
	t.Parallel()
	// 300 MiB of zeros compresses to a few kilobytes: a decompression bomb.
	big := make([]byte, 300<<20)
	h := tar.Header{Typeflag: tar.TypeReg, Name: "src/big.bin", Mode: 0o644, ModTime: time.Unix(0, 0)}
	bomb := archive(t, []tar.Header{h}, [][]byte{big})
	if len(bomb) > 1<<20 {
		t.Fatalf("test setup: the bomb is %d bytes, want a small archive", len(bomb))
	}
	if _, err := connector.OpenPackage(bomb, connector.Options{}); err == nil {
		t.Fatal("OpenPackage() expanded a 300 MiB entry")
	}
}

func ExampleDigestOf() {
	fmt.Println(connector.DigestOf([]byte("hello")))
	// Output: sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
}
