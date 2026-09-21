package feature_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	fhttp "github.com/arandu-io/framework/http"
	swagger "github.com/hyz-is/arandu-swagger"
)

func TestThemeServesCustomCSSAndAppliesDarkPalette(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	cfg := enabledConfig()
	cfg.Theme = swagger.Theme{
		DarkMode:        true,
		PrimaryColor:    "#5eead4",
		BackgroundColor: "#0a0a0a",
		CustomCSS:       ".perata-custom { color: #5eead4; }",
	}
	module := newModule(t, cfg)
	module.Routes(router.ForModule(module.Name()))

	css := request(router, "/docs/theme.css")
	if css.Code != http.StatusOK {
		t.Fatalf("GET /docs/theme.css answered %d", css.Code)
	}
	assertHeader(t, css, "Content-Type", "text/css; charset=utf-8")
	assertHeader(t, css, "Cache-Control", "no-store")

	body := css.Body.String()
	for _, expected := range []string{
		"--swagger-primary: #5eead4;",
		"--swagger-bg: #0a0a0a;",
		"--swagger-card: #121214;",
		".perata-custom { color: #5eead4; }",
		".arandu-swagger-topbar",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("theme.css missing expected fragment %q", expected)
		}
	}

	page := request(router, "/docs")
	if page.Code != http.StatusOK {
		t.Fatalf("GET /docs answered %d", page.Code)
	}
	if !strings.Contains(page.Body.String(), `<link rel="stylesheet" href="/docs/theme.css">`) {
		t.Errorf("/docs does not link theme.css: %s", page.Body.String())
	}
}

func TestThemeRendersLogoAndFaviconInTopbar(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	cfg := enabledConfig()
	cfg.Theme = swagger.Theme{
		DarkMode: true,
		Favicon:  "/favicon.ico",
		Logo: &swagger.ThemeLogo{
			URL:    "/assets/brand/perata-icon.svg",
			Href:   "/workspaces",
			Alt:    "Perata Brand",
			Target: "_blank",
		},
	}
	module := newModule(t, cfg)
	module.Routes(router.ForModule(module.Name()))

	page := request(router, "/docs")
	if page.Code != http.StatusOK {
		t.Fatalf("GET /docs answered %d", page.Code)
	}

	html := page.Body.String()
	for _, fragment := range []string{
		`<link rel="icon" href="/favicon.ico">`,
		`<header class="arandu-swagger-topbar">`,
		`href="/workspaces"`,
		`target="_blank"`,
		`src="/assets/brand/perata-icon.svg"`,
		`alt="Perata Brand"`,
	} {
		if !strings.Contains(html, fragment) {
			t.Errorf("/docs missing expected branding fragment %q", fragment)
		}
	}
}

func TestHTMXLifecycleAndBoostAttribute(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	cfg := enabledConfig()
	cfg.Theme = swagger.Theme{
		HTMX: swagger.HTMXConfig{
			Enabled: true,
			Boost:   true,
		},
	}
	module := newModule(t, cfg)
	module.Routes(router.ForModule(module.Name()))

	page := request(router, "/docs")
	if page.Code != http.StatusOK {
		t.Fatalf("GET /docs answered %d", page.Code)
	}
	if !strings.Contains(page.Body.String(), `<div id="swagger-ui" hx-boost="false"></div>`) {
		t.Errorf("/docs missing hx-boost attribute on #swagger-ui: %s", page.Body.String())
	}

	initScript := request(router, "/docs/swagger-initializer.js")
	if initScript.Code != http.StatusOK {
		t.Fatalf("GET /docs/swagger-initializer.js answered %d", initScript.Code)
	}

	js := initScript.Body.String()
	for _, expected := range []string{
		`data-swagger-initialized`,
		`document.addEventListener("htmx:load"`,
		`document.addEventListener("htmx:afterSettle"`,
	} {
		if !strings.Contains(js, expected) {
			t.Errorf("swagger-initializer.js missing HTMX fragment %q", expected)
		}
	}
}

func TestOpenAPISchemaDialectDefaultsToBaseDialectAndCanBeCustomized(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	defaultCfg := enabledConfig()
	defaultModule := newModule(t, defaultCfg)
	defaultModule.Routes(router.ForModule(defaultModule.Name()))

	defaultDoc, err := defaultModule.Generate()
	if err != nil {
		t.Fatalf("default Generate failed: %v", err)
	}
	if defaultDoc.JSONSchemaDialect != "https://spec.openapis.org/oas/3.1/dialect/base" {
		t.Errorf("default JSONSchemaDialect = %q, want OpenAPI 3.1 base dialect", defaultDoc.JSONSchemaDialect)
	}

	customCfg := enabledConfig()
	customCfg.JSONSchemaDialect = "https://json-schema.org/draft/2020-12/schema"
	customModule := newModule(t, customCfg)
	customRouter := fhttp.NewRouter()
	customModule.Routes(customRouter.ForModule(customModule.Name()))

	customDoc, err := customModule.Generate()
	if err != nil {
		t.Fatalf("custom Generate failed: %v", err)
	}
	if customDoc.JSONSchemaDialect != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("custom JSONSchemaDialect = %q, want 2020-12 dialect", customDoc.JSONSchemaDialect)
	}

	specResp := request(router, "/docs/openapi.json")
	if specResp.Code != http.StatusOK {
		t.Fatalf("GET /docs/openapi.json answered %d", specResp.Code)
	}
	var rawDoc map[string]any
	if err := json.Unmarshal(specResp.Body.Bytes(), &rawDoc); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if rawDoc["jsonSchemaDialect"] != "https://spec.openapis.org/oas/3.1/dialect/base" {
		t.Errorf("raw jsonSchemaDialect = %v, want OpenAPI 3.1 base dialect", rawDoc["jsonSchemaDialect"])
	}
}

func TestThemeEndpointsCollisionValidation(t *testing.T) {
	t.Parallel()

	conflictCSS := enabledConfig()
	conflictCSS.SpecPath = "/docs/theme.css"
	if _, err := swagger.New(conflictCSS); err == nil || !strings.Contains(err.Error(), "theme stylesheet endpoint") {
		t.Fatalf("New with SpecPath=/docs/theme.css error = %v, want conflict error", err)
	}

	conflictJS := enabledConfig()
	conflictJS.SpecPath = "/docs/theme.js"
	if _, err := swagger.New(conflictJS); err == nil || !strings.Contains(err.Error(), "theme script endpoint") {
		t.Fatalf("New with SpecPath=/docs/theme.js error = %v, want conflict error", err)
	}

	blankLogo := enabledConfig()
	blankLogo.Theme.Logo = &swagger.ThemeLogo{URL: "   "}
	if _, err := swagger.New(blankLogo); err == nil || !strings.Contains(err.Error(), "Theme.Logo.URL must not be blank") {
		t.Fatalf("New with blank Logo.URL error = %v, want blank error", err)
	}
}
