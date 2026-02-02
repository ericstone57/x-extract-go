package web

import (
	"embed"
	"io/fs"
)

//go:embed templates/* static/*
var content embed.FS

// GetTemplatesFS returns the embedded templates filesystem
func GetTemplatesFS() fs.FS {
	templatesFS, _ := fs.Sub(content, "templates")
	return templatesFS
}

// GetStaticFS returns the embedded static files filesystem
func GetStaticFS() fs.FS {
	staticFS, _ := fs.Sub(content, "static")
	return staticFS
}
