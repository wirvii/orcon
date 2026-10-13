// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package webui

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	indexName = "index.html"

	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheRevalidate = "no-cache"
)

// reservedPrefixes are the URL prefixes the single-page fallback never answers
// with index.html: a missing resource under one of them is a plain 404, so an
// API typo or a missing hashed asset is never masked by the application shell.
// Other specs that add pages served by Go append their prefix here.
var reservedPrefixes = []string{"/api", "/assets", "/healthz"}

// contentTypes pins the types the web build emits so the result does not
// depend on the host's MIME database.
var contentTypes = map[string]string{
	".html":        "text/html; charset=utf-8",
	".js":          "text/javascript; charset=utf-8",
	".mjs":         "text/javascript; charset=utf-8",
	".css":         "text/css; charset=utf-8",
	".json":        "application/json",
	".map":         "application/json",
	".webmanifest": "application/manifest+json",
	".svg":         "image/svg+xml",
	".txt":         "text/plain; charset=utf-8",
	".wasm":        "application/wasm",
	".woff2":       "font/woff2",
	".woff":        "font/woff",
	".ico":         "image/x-icon",
	".png":         "image/png",
	".jpg":         "image/jpeg",
	".jpeg":        "image/jpeg",
	".webp":        "image/webp",
	".avif":        "image/avif",
}

// Handler serves the built web app from assets.
//
// GET and HEAD for a file that exists are answered with that file. A path that
// does not exist, is not reserved (see reservedPrefixes) and whose last segment
// has no extension is a client-side route and receives index.html with
// "Cache-Control: no-cache". Everything under assets/ is content-hashed by the
// build and is served with immutable caching. Other methods outside /api/ get
// 405; a missing resource under a reserved prefix, or a missing file with an
// extension, gets 404 (problem+json under /api/).
//
// Files are sent pre-compressed when "<name>.br" or "<name>.gz" sits beside
// them and the client accepts that encoding; otherwise compressible files are
// gzipped once on first use and the result is cached for the life of the
// handler.
func Handler(assets fs.FS) http.Handler {
	return &spa{assets: assets, gz: make(map[string][]byte)}
}

type spa struct {
	assets fs.FS

	mu sync.Mutex
	gz map[string][]byte
}

func (s *spa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := path.Clean("/" + r.URL.Path)

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if isUnder(p, "/api") {
			writeNotFound(w, p)
			return
		}
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(p, "/")
	if name == "" {
		name = indexName
	}
	if s.isFile(name) && !isUnder(p, "/healthz") && !isUnder(p, "/api") {
		s.serve(w, r, name, cachePolicy(name))
		return
	}
	if isReserved(p) || path.Ext(name) != "" || !s.isFile(indexName) {
		writeNotFound(w, p)
		return
	}
	// A client-side route: hand the request to the application shell.
	s.serve(w, r, indexName, cacheRevalidate)
}

func isUnder(p, prefix string) bool {
	return p == prefix || strings.HasPrefix(p, prefix+"/")
}

func isReserved(p string) bool {
	for _, prefix := range reservedPrefixes {
		if isUnder(p, prefix) {
			return true
		}
	}
	return false
}

// cachePolicy returns the Cache-Control value of an existing file: build
// output under assets/ carries a content hash and never changes.
func cachePolicy(name string) string {
	if strings.HasPrefix(name, "assets/") {
		return cacheImmutable
	}
	return cacheRevalidate
}

func (s *spa) isFile(name string) bool {
	if !fs.ValidPath(name) {
		return false
	}
	info, err := fs.Stat(s.assets, name)
	return err == nil && !info.IsDir()
}

func (s *spa) serve(w http.ResponseWriter, r *http.Request, name, cache string) {
	ctype := contentType(name)
	body, encoding, ok := s.variant(name, ctype, r.Header.Get("Accept-Encoding"))
	if !ok {
		writeNotFound(w, "/"+name)
		return
	}
	h := w.Header()
	h.Set("Content-Type", ctype)
	h.Set("Cache-Control", cache)
	h.Set("X-Content-Type-Options", "nosniff")
	if compressible(ctype) {
		h.Add("Vary", "Accept-Encoding")
	}
	if encoding != "" {
		h.Set("Content-Encoding", encoding)
	}
	sum := sha256.Sum256(body)
	h.Set("ETag", `"`+hex.EncodeToString(sum[:8])+encoding+`"`)
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
}

// variant picks the bytes and Content-Encoding to send for name.
func (s *spa) variant(name, ctype, acceptEncoding string) ([]byte, string, bool) {
	if compressible(ctype) {
		if accepts(acceptEncoding, "br") {
			if b, err := fs.ReadFile(s.assets, name+".br"); err == nil {
				return b, "br", true
			}
		}
		if accepts(acceptEncoding, "gzip") {
			if b, err := fs.ReadFile(s.assets, name+".gz"); err == nil {
				return b, "gzip", true
			}
			if b, ok := s.gzipped(name); ok {
				return b, "gzip", true
			}
		}
	}
	b, err := fs.ReadFile(s.assets, name)
	return b, "", err == nil
}

// gzipped returns the cached gzip form of name, compressing it on first use.
func (s *spa) gzipped(name string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.gz[name]; ok {
		return b, true
	}
	raw, err := fs.ReadFile(s.assets, name)
	if err != nil {
		return nil, false
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, false
	}
	if _, err := zw.Write(raw); err != nil {
		return nil, false
	}
	if err := zw.Close(); err != nil {
		return nil, false
	}
	s.gz[name] = buf.Bytes()
	return s.gz[name], true
}

func contentType(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if t, ok := contentTypes[ext]; ok {
		return t
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}

// compressible reports whether a content type benefits from compression;
// fonts and images are already compressed formats.
func compressible(ctype string) bool {
	for _, marker := range []string{"text/", "javascript", "json", "xml", "svg", "wasm"} {
		if strings.Contains(ctype, marker) {
			return true
		}
	}
	return false
}

// accepts reports whether an Accept-Encoding header allows the coding.
func accepts(header, coding string) bool {
	for _, part := range strings.Split(header, ",") {
		token, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(token), coding) {
			continue
		}
		params = strings.ReplaceAll(strings.ToLower(params), " ", "")
		return params != "q=0" && params != "q=0.0" && params != "q=0.00" && params != "q=0.000"
	}
	return false
}

func writeNotFound(w http.ResponseWriter, p string) {
	if isUnder(p, "/api") {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"type":"about:blank","title":"Not Found","status":404}`)
		return
	}
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}
