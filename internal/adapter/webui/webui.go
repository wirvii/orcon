// Package webui embeds the built web editor and exposes it as a file system.
// The build output is copied into dist by "make web".
package webui

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed dist
var embedded embed.FS

// Assets returns the embedded web UI rooted at the dist directory.
func Assets() (fs.FS, error) {
	assets, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, fmt.Errorf("open embedded web ui: %w", err)
	}
	return assets, nil
}
