// Package ui provides the self-hosted Swagger UI distribution assets.
package ui

import (
	"embed"
	"sort"
)

// Version is the vendored Swagger UI distribution version.
const Version = "5.32.14"

const assetBase = "assets/" + Version + "/"

// Asset is one embedded Swagger UI file and its response metadata.
type Asset struct {
	// Name is the allowlisted path relative to the documentation UI.
	Name string
	// Path is the embedded source path used to load the asset at build time.
	Path string
	// ContentType is the response media type for the asset.
	ContentType string
	// Bytes is the immutable embedded asset payload.
	Bytes []byte
}

//go:embed assets/5.32.14/*
var embedded embed.FS

var contentTypes = map[string]string{
	"LICENSE":                          "text/plain; charset=utf-8",
	"NOTICE":                           "text/plain; charset=utf-8",
	"swagger-ui-bundle.js":             "text/javascript; charset=utf-8",
	"swagger-ui-bundle.js.LICENSE.txt": "text/plain; charset=utf-8",
	"swagger-ui.css":                   "text/css; charset=utf-8",
}

// Path returns the versioned relative URL path for an allowed asset name.
func Path(name string) (string, bool) {
	if _, ok := contentTypes[name]; !ok {
		return "", false
	}
	return assetBase + name, true
}

// Paths returns the sorted allowlist of versioned relative asset paths.
func Paths() []string {
	paths := make([]string, 0, len(contentTypes))
	for name := range contentTypes {
		path, _ := Path(name)
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// Lookup reads an asset only when path exactly matches the versioned allowlist.
func Lookup(path string) (Asset, bool) {
	for name, contentType := range contentTypes {
		expected, _ := Path(name)
		if path != expected {
			continue
		}

		data, err := embedded.ReadFile(expected)
		if err != nil {
			return Asset{}, false
		}
		return Asset{
			Name:        name,
			Path:        expected,
			ContentType: contentType,
			Bytes:       data,
		}, true
	}
	return Asset{}, false
}
