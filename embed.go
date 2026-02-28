package mapwatch

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var embeddedStatic embed.FS

// StaticFiles is the embedded filesystem containing all static assets.
// Use http.FS(StaticFiles) to serve it, or fs.ReadFile to access individual files.
var StaticFiles fs.FS = embeddedStatic

// StaticHTTPFS returns an http.FileSystem rooted at the static/ subdirectory.
func StaticHTTPFS() http.FileSystem {
	sub, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		panic("embed: " + err.Error())
	}
	return http.FS(sub)
}
