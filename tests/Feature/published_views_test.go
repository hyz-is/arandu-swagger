package feature_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"regexp"
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
	"header.kyse.go":    "9fabc1bae7aff3ffc3e433bfd963164ab26fbdb9ea1495ceccd182e615cfee28",
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
