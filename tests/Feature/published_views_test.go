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
	fmiddleware "github.com/arandu-io/framework/http/middleware"
	"github.com/arandu-io/hesape/view"
	swagger "github.com/hyz-is/arandu-swagger"

	// The published views, compiled by `aru view:build`. Importing the package
	// registers them, so a router given a renderer draws the page through them
	// exactly as an application that published them does.
	_ "github.com/hyz-is/arandu-swagger/tests/Feature/internal/compiledviews/docs"
)

// compiledViewSources is the SHA-256 of each published view as it was when the
// code under internal/compiledviews/docs was compiled from it.
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
//	cp <app>/storage/framework/views/docs/*.go tests/Feature/internal/compiledviews/docs/
//	shasum -a 256 resources/publish/docs/*.kyse.go
//
// and write the new sums here.
var compiledViewSources = map[string]string{
	"container.kyse.go": "b13a78e0732c3f396cb6dfaac565504deb5199fd55dac51f5fffd44b7c987977",
	"header.kyse.go":    "eb6c99d28bd22bebac86df016de9bad8b398af953af05d8b38a6d6ac5e8f9ea9",
	"swagger.kyse.go":   "3d88937ceab17fc78d6acd7f95df93ff10c6fad777522e20aace4108ea6a60a3",
	"topbar.kyse.go":    "b94ad1044d0fb43ee78a6f67e43fbf57657fa211d5acbe083fcac261a25726f5",
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
				t.Errorf("%s is published but has no compiled copy under internal/compiledviews/docs", name)
				return nil
			}
			if got := hex.EncodeToString(sum[:]); got != want {
				t.Errorf("%s changed since internal/compiledviews/docs was compiled from it (sha256 %s, compiled from %s); compile the views again as compiledViewSources describes", name, got, want)
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

// assetsCSPDoc returns docs/assets-csp.md.
func assetsCSPDoc(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	doc, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "docs", "assets-csp.md"))
	if err != nil {
		t.Fatalf("read docs/assets-csp.md: %v", err)
	}
	return string(doc)
}

// documentedPolicies returns the two content security policies
// docs/assets-csp.md states, in the order it states them: the one the
// docs.swagger view answers with, then the one the embedded page answers with.
func documentedPolicies(t *testing.T) (viewPolicy, embeddedPolicy string) {
	t.Helper()
	var policies []string
	for _, line := range strings.Split(assetsCSPDoc(t), "\n") {
		if strings.HasPrefix(line, "default-src ") {
			policies = append(policies, line)
		}
	}
	if len(policies) != 2 {
		t.Fatalf("docs/assets-csp.md states %d policies, want the view's and the embedded page's: %q", len(policies), policies)
	}
	return policies[0], policies[1]
}

// directives splits a content security policy into its directives, keyed by
// name, failing on a directive named twice.
func directives(t *testing.T, policy string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, part := range strings.Split(policy, ";") {
		name, value, _ := strings.Cut(strings.TrimSpace(part), " ")
		if _, dup := out[name]; dup {
			t.Fatalf("policy %q names %s twice", policy, name)
		}
		out[name] = value
	}
	return out
}

func TestUIRouteSendsTheDocumentedHeadersFromTheViewAndTheEmbeddedPage(t *testing.T) {
	t.Parallel()

	viewPolicy, embeddedPolicy := documentedPolicies(t)
	shared := map[string]string{
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
		"Content-Type":           "text/html; charset=utf-8",
	}

	viewRouter := mountRendered(t, enabledConfig())
	renderedPage(t, viewRouter, "/docs")
	embeddedRouter, _ := mount(t, enabledConfig())

	for _, tc := range []struct {
		name   string
		router *fhttp.Router
		policy string
	}{
		{"the docs.swagger view", viewRouter, viewPolicy},
		{"the embedded page", embeddedRouter, embeddedPolicy},
	} {
		response := request(tc.router, "/docs")
		if response.Code != http.StatusOK {
			t.Fatalf("%s answered %d", tc.name, response.Code)
		}
		if got := response.Header().Get("Content-Security-Policy"); got != tc.policy {
			t.Errorf("%s: Content-Security-Policy = %q, want %q", tc.name, got, tc.policy)
		}
		for header, value := range shared {
			if got := response.Header().Get(header); got != value {
				t.Errorf("%s: %s = %q, want %q", tc.name, header, got, value)
			}
		}
	}
}

func TestViewPolicyAdmitsOnlyFormsToItsOwnOriginAndChangesNothingElse(t *testing.T) {
	t.Parallel()

	view := directives(t, request(mountRendered(t, enabledConfig()), "/docs").Header().Get("Content-Security-Policy"))
	embeddedRouter, _ := mount(t, enabledConfig())
	embedded := directives(t, request(embeddedRouter, "/docs").Header().Get("Content-Security-Policy"))

	if got := view["form-action"]; got != "'self'" {
		t.Errorf("the view's form-action = %q, want 'self': a form in the application's layout submits to the application", got)
	}
	if got := embedded["form-action"]; got != "'none'" {
		t.Errorf("the embedded page's form-action = %q, want 'none': that page draws no form a browser submits", got)
	}
	if len(view) != len(embedded) {
		t.Errorf("the view's policy has %d directives and the embedded page's %d:\nview:     %v\nembedded: %v", len(view), len(embedded), view, embedded)
	}
	for name, value := range embedded {
		if name == "form-action" {
			continue
		}
		if got := view[name]; got != value {
			t.Errorf("the view's %s = %q, want the embedded page's %q", name, got, value)
		}
	}
}

func TestUIRouteReplacesThePolicyAMiddlewareSetBeforeIt(t *testing.T) {
	t.Parallel()

	// The claim docs/assets-csp.md makes, word for word with its line breaks
	// folded, so the document cannot go back to promising that a middleware
	// replaces the page's policy while this test still passes.
	claim := "The UI route sets these four headers itself, so a value an application middleware set for any of them before calling the route is replaced."
	if doc := spaces.ReplaceAllString(assetsCSPDoc(t), " "); !strings.Contains(doc, claim) {
		t.Errorf("docs/assets-csp.md no longer says %q", claim)
	}

	// The default security headers an application runs on every route, which
	// write a policy, a referrer policy and nosniff of their own before the
	// handler runs.
	config := enabledConfig()
	config.UIMiddleware = []swagger.Middleware{fmiddleware.SecurityHeaders(false)}
	viewPolicy, embeddedPolicy := documentedPolicies(t)
	embeddedRouter, _ := mount(t, config)

	for _, tc := range []struct {
		name   string
		router *fhttp.Router
		policy string
	}{
		{"the docs.swagger view", mountRendered(t, config), viewPolicy},
		{"the embedded page", embeddedRouter, embeddedPolicy},
	} {
		response := request(tc.router, "/docs")
		if response.Code != http.StatusOK {
			t.Fatalf("%s answered %d", tc.name, response.Code)
		}
		if got := response.Header().Get("Content-Security-Policy"); got != tc.policy {
			t.Errorf("%s: Content-Security-Policy = %q, want the route's own %q", tc.name, got, tc.policy)
		}
		if got := response.Header().Values("Content-Security-Policy"); len(got) != 1 {
			t.Errorf("%s: %d Content-Security-Policy headers, want one: %q", tc.name, len(got), got)
		}
		if got := response.Header().Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("%s: Referrer-Policy = %q, want the route's no-referrer", tc.name, got)
		}
		// The headers the route does not set reach the browser as the
		// middleware wrote them.
		if got := response.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options = %q, want the middleware's DENY", tc.name, got)
		}
		if got := response.Header().Get("Permissions-Policy"); got == "" {
			t.Errorf("%s: the middleware's Permissions-Policy did not reach the answer", tc.name)
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

// headerFor renders the published header with the data the UI route hands
// the docs.swagger view under config.
func headerFor(t *testing.T, config swagger.Config) string {
	t.Helper()
	router, _ := mount(t, config)
	rendered, _ := requestViewData(t, router, "/docs")
	return renderView(t, "docs.header", rendered.Data)
}

func TestPublishedHeaderFollowsTheLocale(t *testing.T) {
	t.Parallel()

	portuguese := enabledConfig()
	portuguese.Locale = "pt-BR"
	overridden := enabledConfig()
	overridden.Translations = map[string]string{"Home": "Dashboard", "Authorize": "Sign in"}

	for _, tc := range []struct {
		name   string
		config swagger.Config
		want   []string
		refuse []string
	}{
		{
			name:   "default English",
			config: enabledConfig(),
			want:   []string{"<span>Home</span>", "<span>API documentation</span>", ">Developer integration</p>", "<span>Authorize</span>"},
			refuse: []string{"Workspace", "Documentação", "INTEGRAÇÃO", "Integração", "Autorizar"},
		},
		{
			name:   "pt-BR",
			config: portuguese,
			want:   []string{"<span>Início</span>", "<span>Documentação da API</span>", ">Integração e desenvolvedor</p>", "<span>Autorizar</span>"},
			refuse: []string{"Workspace", "<span>Home</span>", "API documentation", "<span>Authorize</span>"},
		},
		{
			name:   "Config.Translations",
			config: overridden,
			want:   []string{"<span>Dashboard</span>", "<span>Sign in</span>", "<span>API documentation</span>"},
			refuse: []string{"Workspace", "<span>Home</span>", "<span>Authorize</span>"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := headerFor(t, tc.config)
			assertNoTemplateSyntax(t, body)
			for _, fragment := range tc.want {
				if !strings.Contains(body, fragment) {
					t.Errorf("the header does not draw %q:\n%s", fragment, body)
				}
			}
			for _, fragment := range tc.refuse {
				if strings.Contains(body, fragment) {
					t.Errorf("the header draws %q:\n%s", fragment, body)
				}
			}
		})
	}
}

func TestThemeToggleLabelFollowsTheLocaleOnBothPages(t *testing.T) {
	t.Parallel()

	portuguese := enabledConfig()
	portuguese.Locale = "pt-BR"
	for _, tc := range []struct {
		name   string
		config swagger.Config
		label  string
	}{
		{"default English", enabledConfig(), "Toggle theme"},
		{"pt-BR", portuguese, "Alternar tema"},
	} {
		want := `aria-label="` + tc.label + `" title="` + tc.label + `"`
		viewBody := renderedPage(t, mountRendered(t, tc.config), "/docs")
		if got := openingTag(t, viewBody, "arandu-swagger-theme-toggle"); !strings.Contains(got, want) {
			t.Errorf("%s: the view's toggle = %s, want %s", tc.name, got, want)
		}
		embedded, _ := mount(t, tc.config)
		embeddedBody := request(embedded, "/docs").Body.String()
		if got := openingTag(t, embeddedBody, "arandu-swagger-theme-toggle"); !strings.Contains(got, want) {
			t.Errorf("%s: the embedded page's toggle = %s, want %s", tc.name, got, want)
		}
	}
}
