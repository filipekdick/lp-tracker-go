// Package web embeds the dashboard's static assets so the server ships as a
// single self-contained binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var assets embed.FS

// Assets returns the embedded frontend filesystem rooted at the asset folder,
// so "/" serves index.html.
func Assets() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	return sub
}
