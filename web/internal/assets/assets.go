// Package assets holds the built front end (web/ui, built into dist/ by pnpm build). Before a build, dist/ holds
// only .gitkeep and the server says so.
package assets

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS is the front end with index.html at its root.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
