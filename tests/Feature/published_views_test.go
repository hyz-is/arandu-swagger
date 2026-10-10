package feature_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/view"
	swagger "github.com/hyz-is/arandu-swagger"

	// The published views, compiled by `aru view:build`. Importing the package
	// registers them, so a router given a renderer draws the page through them
	// exactly as an application that published them does.
	_ "github.com/hyz-is/arandu-swagger/tests/Feature/compiledviews/docs"
)

// compiledViewSources is the SHA-256 of each published view as it was when the
// code under compiledviews/docs was compiled from it.
//
// The views are templates, and a template is only tested by rendering it, which
// needs the compiler that lives in the aru command. That code is therefore
// compiled once and committed, and this record is what keeps it honest: a view
// edited without compiling it again fails TestCompiledViewsMatchThePublishedSources
// instead of leaving every rendering test below asserting on the old markup.
//
// To compile them again, with aru v0.69.3 or later, in any Arandu application
// whose go.mod resolves this module to the working copy:
//
//	cp resources/publish/docs/*.kyse.go <app>/resources/views/docs/
//	(cd <app> && aru view:build)
//	cp <app>/storage/framework/views/docs/*.go tests/Feature/compiledviews/docs/
//	shasum -a 256 resources/publish/docs/*.kyse.go
//
// and write the new sums here.
var compiledViewSources = map[string]string{
	"container.kyse.go": "b13a78e0732c3f396cb6dfaac565504deb5199fd55dac51f5fffd44b7c987977",
	"header.kyse.go":    "051ace7a339119039fceac6d5cd333283713659acab581a44282bccb7f8ca052",
	"swagger.kyse.go":   "3d88937ceab17fc78d6acd7f95df93ff10c6fad777522e20aace4108ea6a60a3",
	"topbar.kyse.go":    "19772ae7729b6e1d534030055ce5a25011842a0b993fd9b9d5db4795d78e584f",
}

func TestCompiledViewsMatchThePublishedSources(t *testing.T) {
	t.Parallel()

	module := newModule(t, enabledConfig())
	seen := map[string]bool{}
	for _, publication := range module.Publishes() {
		err := fs.WalkDir(publication.Files, publication.From, func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			source, err := fs.ReadFile(publication.Files, name)
			if err != nil {
				return err
			}
			base := path.Base(name)
			seen[base] = true
			sum := sha256.Sum256(source)
			want, recorded := compiledViewSources[base]
			if !recorded {
				t.Errorf("%s is published but has no compiled copy under compiledviews/docs", name)
				return nil
			}
			if got := hex.EncodeToString(sum[:]); got != want {
				t.Errorf("%s changed since compiledviews/docs was compiled from it (sha256 %s, compiled from %s); compile the views again as compiledViewSources describes", name, got, want)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk published views: %v", err)
		}
	}
	for base := range compiledViewSources {
		if !seen[base] {
			t.Errorf("compiledViewSources records %s, which is no longer published", base)
		}
	}
}

// mountRendered mounts the module on a router that renders views, so the UI
// route draws the published docs.swagger view instead of the embedded page.
func mountRendered(t *testing.T, config swagger.Config) *fhttp.Router {
	t.Helper()
	router := fhttp.NewRouter().WithRenderer(view.NewRenderer())
	module := newModule(t, config)
	module.Routes(router.ForModule(module.Name()))
	return router
}

// renderedPage returns the markup the published view draws at path, failing
// when the embedded page answered instead.
func renderedPage(t *testing.T, router *fhttp.Router, target string) string {
	t.Helper()
	response := request(router, target)
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s answered %d: %s", target, response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, `class="arandu-swagger-container"`) {
		t.Fatalf("GET %s was not drawn by the published view:\n%s", target, body)
	}
	return body
}

var spaces = regexp.MustCompile(`\s+`)

// openingTag returns the opening tag of the first element whose class
// attribute is exactly class, with its whitespace collapsed.
func openingTag(t *testing.T, body, class string) string {
	t.Helper()
	marker := strings.Index(body, `class="`+class+`"`)
	if marker < 0 {
		t.Fatalf("no element with class %q in:\n%s", class, body)
	}
	start := strings.LastIndex(body[:marker], "<")
	end := strings.Index(body[marker:], ">")
	if start < 0 || end < 0 {
		t.Fatalf("the element with class %q is not a complete tag in:\n%s", class, body)
	}
	return spaces.ReplaceAllString(body[start:marker+end+1], " ")
}

func assertNoTemplateSyntax(t *testing.T, body string) {
	t.Helper()
	for _, token := range []string{"@if", "@endif", "{{", "}}"} {
		if strings.Contains(body, token) {
			t.Errorf("the rendered page carries template syntax %q:\n%s", token, body)
		}
	}
}

func TestPublishedTopbarWritesTheLinkTargetsItIsGiven(t *testing.T) {
	t.Parallel()

	cfg := enabledConfig()
	cfg.Theme.Logo = &swagger.ThemeLogo{URL: "/brand/acme.svg", Href: "/home", Alt: "Acme", Target: "_blank"}
	cfg.Theme.BackTarget = "_top"
	body := renderedPage(t, mountRendered(t, cfg), "/docs")

	assertNoTemplateSyntax(t, body)
	if got, want := openingTag(t, body, "arandu-swagger-logo-link"), `<a href="/home" class="arandu-swagger-logo-link" target="_blank" >`; got != want {
		t.Errorf("logo link = %s, want %s", got, want)
	}
	if got, want := openingTag(t, body, "arandu-swagger-back-link"), `<a href="/home" class="arandu-swagger-back-link" target="_top" >`; got != want {
		t.Errorf("back link = %s, want %s", got, want)
	}
}

func TestPublishedTopbarWritesNoTargetWhenNoneIsGiven(t *testing.T) {
	t.Parallel()

	body := renderedPage(t, mountRendered(t, enabledConfig()), "/docs")

	assertNoTemplateSyntax(t, body)
	if got, want := openingTag(t, body, "arandu-swagger-logo-link"), `<a href="/" class="arandu-swagger-logo-link" >`; got != want {
		t.Errorf("logo link = %s, want %s", got, want)
	}
	// New fills BackTarget with _self when it is blank, so the back link always
	// names one.
	if got, want := openingTag(t, body, "arandu-swagger-back-link"), `<a href="/" class="arandu-swagger-back-link" target="_self" >`; got != want {
		t.Errorf("back link = %s, want %s", got, want)
	}
}

// documentedPolicy returns the content security policy docs/assets-csp.md
// states, which is the one the UI route promises to send.
func documentedPolicy(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	doc, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "docs", "assets-csp.md"))
	if err != nil {
		t.Fatalf("read docs/assets-csp.md: %v", err)
	}
	for _, line := range strings.Split(string(doc), "\n") {
		if strings.HasPrefix(line, "default-src ") {
			return line
		}
	}
	t.Fatal("docs/assets-csp.md states no policy")
	return ""
}

func TestUIRouteSendsTheDocumentedHeadersFromTheViewAndTheEmbeddedPage(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		"Content-Security-Policy": documentedPolicy(t),
		"Referrer-Policy":         "no-referrer",
		"Cache-Control":           "no-store",
		"X-Content-Type-Options":  "nosniff",
		"Content-Type":            "text/html; charset=utf-8",
	}

	viewRouter := mountRendered(t, enabledConfig())
	renderedPage(t, viewRouter, "/docs")
	embeddedRouter, _ := mount(t, enabledConfig())

	for name, router := range map[string]*fhttp.Router{"the docs.swagger view": viewRouter, "the embedded page": embeddedRouter} {
		response := request(router, "/docs")
		if response.Code != http.StatusOK {
			t.Fatalf("%s answered %d", name, response.Code)
		}
		for header, value := range want {
			if got := response.Header().Get(header); got != value {
				t.Errorf("%s: %s = %q, want %q", name, header, got, value)
			}
		}
	}
}

var inlineHandler = regexp.MustCompile(`(?i)\son[a-z]+\s*=`)

func TestPublishedViewsRunNoScriptThePolicyRefuses(t *testing.T) {
	t.Parallel()

	data := swagger.SwaggerViewData{UIPath: "/docs", SpecPath: "/docs/openapi.json", Locale: "en", BackURL: "/"}
	data.Title = "Example API"
	pages := map[string]string{"docs.swagger": renderedPage(t, mountRendered(t, enabledConfig()), "/docs")}
	pages["docs.header"] = renderView(t, "docs.header", data)

	for name, body := range pages {
		if match := inlineHandler.FindString(body); match != "" {
			t.Errorf("%s writes an inline event handler (%q), which script-src 'self' refuses:\n%s", name, match, body)
		}
		if strings.Contains(body, "<script>") || strings.Contains(body, "<style") {
			t.Errorf("%s writes an inline script or style block:\n%s", name, body)
		}
	}
	if !strings.Contains(pages["docs.header"], "data-arandu-swagger-authorize") {
		t.Errorf("the header's Authorize button carries no data-arandu-swagger-authorize:\n%s", pages["docs.header"])
	}

	router, _ := mount(t, enabledConfig())
	initializer := request(router, "/docs/swagger-initializer.js").Body.String()
	if !strings.Contains(initializer, `document.querySelectorAll("[data-arandu-swagger-authorize]")`) {
		t.Errorf("the initializer does not bind the header's Authorize button:\n%s", initializer)
	}
}

// renderView draws one published view with data, as a layout that includes it
// would.
func renderView(t *testing.T, name string, data swagger.SwaggerViewData) string {
	t.Helper()
	response := httptest.NewRecorder()
	if err := view.NewRenderer().Render(context.Background(), response, http.StatusOK, name, data); err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
	return response.Body.String()
}
