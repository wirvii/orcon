// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package reference serves the API reference site at /api/reference.
//
// The site is built from the contract by "make api-reference" (Scalar API
// Reference, script copied without a CDN) and embedded here by "make build",
// which copies dist/api-reference into the dist directory of this package. The
// directory holds only a .gitkeep in the repository, so the package builds
// without the site and then serves a placeholder page. Every binary that serves the API serves the reference of
// exactly the API it runs (ADR-0054 section 7, ADR-0080 section 10).
package reference

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
)

//go:embed all:dist
var embedded embed.FS

// Site returns the embedded reference site rooted at its index.html.
func Site() (fs.FS, error) {
	site, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, fmt.Errorf("open the embedded reference site: %w", err)
	}
	return site, nil
}

// MountPath is where the site lives; the redirect and the handler assume it.
const MountPath = "/api/reference/"

// ContentSecurityPolicy is the policy of every response of the site (ADR-0080
// section 10): same-origin only, styles inline because the reference injects
// them at run time, no framing, no forms, no plugins.
const ContentSecurityPolicy = "default-src 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; base-uri 'self'; form-action 'none'; frame-ancestors 'none'; object-src 'none'"

// Redirect sends /api/reference to /api/reference/ and keeps the query.
func Redirect(w http.ResponseWriter, r *http.Request) {
	target := url.URL{Path: MountPath, RawQuery: r.URL.RawQuery}
	setSecurityHeaders(w)
	http.Redirect(w, r, target.String(), http.StatusMovedPermanently)
}

func setSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Security-Policy", ContentSecurityPolicy)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
}

type entry struct {
	etag string
}

// Handler serves site under /api/reference/. It needs no authentication, like
// the OpenAPI document. Files carry a strong ETag computed once, so a client
// revalidates cheaply.
func Handler(site fs.FS) http.Handler {
	entries := map[string]entry{}
	_ = fs.WalkDir(site, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		f, err := site.Open(name)
		if err != nil {
			return nil
		}
		defer func() { _ = f.Close() }()
		sum := sha256.New()
		if _, err := io.Copy(sum, f); err == nil {
			entries[name] = entry{etag: `"` + hex.EncodeToString(sum.Sum(nil))[:32] + `"`}
		}
		return nil
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		rel := strings.TrimPrefix(r.URL.Path, MountPath)
		name := path.Clean("/" + rel)[1:]
		if name == "" || strings.HasSuffix(rel, "/") {
			name = path.Join(name, "index.html")
		}
		e, ok := entries[name]
		if !ok && name == "index.html" {
			servePlaceholder(w, r)
			return
		}
		if !ok {
			problem.Write(w, r, problem.NotFound)
			return
		}
		f, err := site.Open(name)
		if err != nil {
			problem.Write(w, r, problem.NotFound)
			return
		}
		defer func() { _ = f.Close() }()
		seeker, ok := f.(io.ReadSeeker)
		if !ok {
			problem.Write(w, r, problem.InternalError)
			return
		}
		w.Header().Set("ETag", e.etag)
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", contentType(name))
		http.ServeContent(w, r, name, time.Time{}, seeker)
	})
}

func contentType(name string) string {
	switch path.Ext(name) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json"
	case ".txt":
		return "text/plain; charset=utf-8"
	}
	if t := mime.TypeByExtension(path.Ext(name)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// placeholderPage is what a binary built without the site serves at the root.
const placeholderPage = `<!doctype html>
<html lang="en">
  <head><meta charset="utf-8"><title>Orcon API reference</title></head>
  <body>
    <h1>Orcon API reference</h1>
    <p>This build does not include the reference site. Run <code>make build</code> to generate it from the API contract and embed it.</p>
  </body>
</html>
`

func servePlaceholder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", time.Time{}, strings.NewReader(placeholderPage))
}
