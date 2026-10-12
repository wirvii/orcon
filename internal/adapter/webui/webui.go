package webui

import (
 "embed"
 "io/fs"
)

//go:embed dist
var embedded embed.FS

// Assets returns the embedded web UI rooted at the dist directory.
func Assets() (fs.FS, error) { return fs.Sub(embedded, "dist") }
