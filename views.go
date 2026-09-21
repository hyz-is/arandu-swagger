package swagger

import (
	"embed"

	"github.com/arandu-io/framework/foundation"
)

// The view sources this package offers to the project that installs it.
//
//go:embed all:resources/publish/*/*.kyse.go
var viewSources embed.FS

const (
	viewRoot   = "resources/publish"
	viewPrefix = "resources/views"
)

var publishedPaths = []string{
	"resources/views/docs/swagger.kyse.go",
}

// Publishes declares the files this package offers, each at the path it takes
// relative to the root of the project.
func (m *Module) Publishes() []foundation.Publication {
	return []foundation.Publication{{
		Tag:   foundation.PublishView,
		Files: viewSources,
		From:  viewRoot,
		To:    viewPrefix,
	}}
}

// PublishedPaths returns the files the module offers for publication.
func PublishedPaths() []string {
	return append([]string(nil), publishedPaths...)
}

// SwaggerViewData is the data payload provided to the Kyse view.
type SwaggerViewData struct {
	Title       string
	Description string
	Version     string
	SpecPath    string
	UIPath      string
	Locale      string
}
