// Package web embeds the compiled frontend: the output of "npm run build" in this
// directory (dist/). The directory always holds a .gitkeep, so the embed compiles on a
// fresh checkout; the UI is just absent until the frontend is built.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Files returns the built frontend, rooted at dist/ (so index.html is at the top).
func Files() fs.FS {
	files, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // unreachable: the embed directive above guarantees dist exists
	}
	return files
}
