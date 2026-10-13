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
	"strconv"
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
	return &spa{assets: assets, variants: make(map[variantKey]*variant)}
}

// variantKey identifies one servable form of a file.
type variantKey struct {
	name     string
	encoding string // "", "br" or "gzip"
}

// variant is the bytes sent for a variantKey and their validator.
type variant struct {
	body []byte
	etag string
}

type spa struct {
	assets fs.FS

	mu       sync.RWMutex
	variants map[variantKey]*variant
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
	encoding := ""
	if compressible(ctype) {
		encoding = s.negotiate(name, r.Header.Values("Accept-Encoding"))
	}
	v, ok := s.lookup(variantKey{name: name, encoding: encoding})
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
	h.Set("ETag", v.etag)
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(v.body))
}

// negotiate picks the Content-Encoding to send for name: brotli when a
// pre-compressed .br sits beside the file, then gzip (pre-compressed or made on
// the fly), otherwise identity.
func (s *spa) negotiate(name string, acceptEncoding []string) string {
	if accepts(acceptEncoding, "br") && s.isFile(name+".br") {
		return "br"
	}
	if accepts(acceptEncoding, "gzip") {
		return "gzip"
	}
	return ""
}

// lookup returns the cached variant, building and caching it on first use.
// The mutex is never held while reading or compressing: concurrent first
// requests may build the same variant, and the first one stored wins.
func (s *spa) lookup(key variantKey) (*variant, bool) {
	s.mu.RLock()
	v, ok := s.variants[key]
	s.mu.RUnlock()
	if ok {
		return v, true
	}
	body, ok := s.build(key)
	if !ok {
		return nil, false
	}
	sum := sha256.Sum256(body)
	built := &variant{body: body, etag: `"` + hex.EncodeToString(sum[:8]) + key.encoding + `"`}

	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.variants[key]; ok {
		return existing, true
	}
	s.variants[key] = built
	return built, true
}

func (s *spa) build(key variantKey) ([]byte, bool) {
	switch key.encoding {
	case "br":
		b, err := fs.ReadFile(s.assets, key.name+".br")
		return b, err == nil
	case "gzip":
		if b, err := fs.ReadFile(s.assets, key.name+".gz"); err == nil {
			return b, true
		}
		raw, err := fs.ReadFile(s.assets, key.name)
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
		return buf.Bytes(), true
	default:
		b, err := fs.ReadFile(s.assets, key.name)
		return b, err == nil
	}
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

// accepts reports whether the Accept-Encoding header lines allow the coding.
// An explicit entry decides; otherwise a "*" entry does; a q value of zero
// refuses the coding.
func accepts(lines []string, coding string) bool {
	explicit, wildcard := -1, -1 // -1: not mentioned, 0: refused, 1: allowed
	for _, line := range lines {
		for _, part := range strings.Split(line, ",") {
			token, params, _ := strings.Cut(strings.TrimSpace(part), ";")
			token = strings.TrimSpace(token)
			allowed := 1
			if qualityIsZero(params) {
				allowed = 0
			}
			switch {
			case strings.EqualFold(token, coding):
				explicit = allowed
			case token == "*":
				wildcard = allowed
			}
		}
	}
	if explicit >= 0 {
		return explicit == 1
	}
	return wildcard == 1
}

// qualityIsZero reports whether a parameter list carries q=0 (any spelling).
func qualityIsZero(params string) bool {
	for _, p := range strings.Split(params, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(p), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "q") {
			continue
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return err == nil && f == 0
	}
	return false
}

func writeNotFound(w http.ResponseWriter, p string) {
	if isUnder(p, "/api") {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"type":"about:blank","title":"Not Found","status":404}`)
		return
	}
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}
