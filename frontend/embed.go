// Package frontend embeds the production browser build. It lives beside dist
// because go:embed cannot reach parent directories, so the server package
// imports the build from here.
package frontend

import (
	"embed"
	"io/fs"
)

// The production build is embedded, so the binary is the whole deployment.
// `all:` keeps the committed `.gitkeep` eligible, which lets `go build` work on
// a fresh checkout before anyone has run `make build-web`; the page route then
// reports the missing build instead of serving a blank document.
//
//go:embed all:dist
var build embed.FS

// Dist is dist rooted at itself, so "index.html" and "assets/…" are exactly
// the paths the built document already references.
var Dist = mustSub(build, "dist")

func mustSub(embedded embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(embedded, dir)
	if err != nil {
		panic(err) // the embed pattern above guarantees the directory exists
	}
	return sub
}
