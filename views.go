package swagger

import (
	"embed"
	"strings"

	"github.com/arandu-io/framework/foundation"
	"github.com/arandu-io/hesape/view"
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
	"resources/views/docs/topbar.kyse.go",
	"resources/views/docs/container.kyse.go",
	"resources/views/docs/header.kyse.go",
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
	view.Page
	Version            string
	SpecPath           string
	UIPath             string
	Locale             string
	Favicon            string
	BackURL            string
	BackText           string
	BackTarget         string
	DisableThemeToggle bool
	LogoURL            string
	LogoAlt            string
	LogoHref           string
	LogoTarget         string
}

// FaviconOrDefault returns the favicon path, defaulting to /favicon.ico.
func (d SwaggerViewData) FaviconOrDefault() string {
	if d.Favicon != "" {
		return d.Favicon
	}
	return "/favicon.ico"
}

// HomeURLOrDefault returns the home URL, preferring LogoHref, then HomeURL, then /.
func (d SwaggerViewData) HomeURLOrDefault() string {
	if d.LogoHref != "" {
		return d.LogoHref
	}
	if d.HomeURL != "" {
		return d.HomeURL
	}
	return "/"
}

// BackURLOrDefault returns the URL to return to the site, defaulting to HomeURLOrDefault.
func (d SwaggerViewData) BackURLOrDefault() string {
	if d.BackURL != "" {
		return d.BackURL
	}
	return d.HomeURLOrDefault()
}

// BackTextOrDefault returns the back link label localized according to Locale.
func (d SwaggerViewData) BackTextOrDefault() string {
	if d.BackText != "" {
		return d.BackText
	}
	if d.Locale == "pt-BR" || strings.HasPrefix(d.Locale, "pt") {
		return "Voltar para o site"
	}
	return "Back to site"
}

// LogoURLOrDefault returns the logo URL, defaulting to /favicon.svg.
func (d SwaggerViewData) LogoURLOrDefault() string {
	if d.LogoURL != "" {
		return d.LogoURL
	}
	return "/favicon.svg"
}

// BrandOrDefault returns the brand name, preferring LogoAlt, then AppName, then Peráta.
func (d SwaggerViewData) BrandOrDefault() string {
	if d.LogoAlt != "" {
		return d.LogoAlt
	}
	if d.AppName != "" {
		return d.AppName
	}
	return "Peráta"
}

// Compile-time check that SwaggerViewData satisfies the view.Layout interface.
var _ view.Layout = SwaggerViewData{}
