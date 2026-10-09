package feature_test

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fhttp "github.com/arandu-io/framework/http"
	hhttp "github.com/arandu-io/hesape/http"
	swagger "github.com/hyz-is/arandu-swagger"
)

// productTraces are strings that belong to one application and must never
// reach a page the package draws without being told to.
var productTraces = []string{"Peráta", "Perata", "perata", "/workspaces", "/favicon.svg"}

type renderedView struct {
	View string                  `json:"view"`
	Data swagger.SwaggerViewData `json:"data"`
}

func requestViewData(t *testing.T, router *fhttp.Router, target string) (renderedView, string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Accept", hhttp.ViewDataMediaType)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s as view data answered %d: %s", target, response.Code, response.Body.String())
	}
	var rendered renderedView
	if err := json.Unmarshal(response.Body.Bytes(), &rendered); err != nil {
		t.Fatalf("decode view data: %v\n%s", err, response.Body.String())
	}
	return rendered, response.Body.String()
}

func assertNoProductTrace(t *testing.T, where, body string) {
	t.Helper()
	for _, trace := range productTraces {
		if strings.Contains(body, trace) {
			t.Errorf("%s carries %q without a Theme naming it:\n%s", where, trace, body)
		}
	}
}

func TestUIWithoutThemeNamesNoProductHomeOrLogo(t *testing.T) {
	t.Parallel()

	router, _ := mount(t, enabledConfig())

	rendered, raw := requestViewData(t, router, "/docs")
	if rendered.View != "docs.swagger" {
		t.Fatalf("view = %q, want docs.swagger", rendered.View)
	}
	assertNoProductTrace(t, "the docs.swagger view data", raw)

	data := rendered.Data
	if data.LogoURL != "" || data.LogoURLOrDefault() != "" {
		t.Errorf("logo = %q / %q, want none", data.LogoURL, data.LogoURLOrDefault())
	}
	if data.AppName != "Example API" || data.BrandOrDefault() != "Example API" {
		t.Errorf("brand = %q / %q, want the document title", data.AppName, data.BrandOrDefault())
	}
	if data.HomeURL != "/" || data.HomeURLOrDefault() != "/" {
		t.Errorf("home = %q / %q, want /", data.HomeURL, data.HomeURLOrDefault())
	}

	page := request(router, "/docs")
	if page.Code != http.StatusOK {
		t.Fatalf("GET /docs answered %d", page.Code)
	}
	assertNoProductTrace(t, "the embedded page", page.Body.String())
	if strings.Contains(page.Body.String(), "<img") {
		t.Errorf("the embedded page draws an image nobody configured:\n%s", page.Body.String())
	}
}

func TestConfiguredLogoReachesTheViewUnchanged(t *testing.T) {
	t.Parallel()

	cfg := enabledConfig()
	cfg.Theme.Logo = &swagger.ThemeLogo{
		URL:    "/brand/acme.svg",
		Href:   "/home",
		Alt:    "Acme",
		Target: "_blank",
	}
	router, _ := mount(t, cfg)

	rendered, _ := requestViewData(t, router, "/docs")
	data := rendered.Data
	for _, check := range []struct{ name, got, want string }{
		{"LogoURL", data.LogoURL, "/brand/acme.svg"},
		{"LogoHref", data.LogoHref, "/home"},
		{"LogoAlt", data.LogoAlt, "Acme"},
		{"LogoTarget", data.LogoTarget, "_blank"},
		{"AppName", data.AppName, "Acme"},
		{"HomeURL", data.HomeURL, "/home"},
		{"BackURL", data.BackURL, "/home"},
	} {
		if check.got != check.want {
			t.Errorf("%s = %q, want %q", check.name, check.got, check.want)
		}
	}
}

func TestViewDataHelpersFallBackToThePageTitle(t *testing.T) {
	t.Parallel()

	var zero swagger.SwaggerViewData
	if got := zero.BrandOrDefault(); got != "" {
		t.Errorf("zero BrandOrDefault() = %q, want empty", got)
	}
	if got := zero.LogoURLOrDefault(); got != "" {
		t.Errorf("zero LogoURLOrDefault() = %q, want empty", got)
	}

	titled := swagger.SwaggerViewData{}
	titled.Title = "Billing API"
	if got := titled.BrandOrDefault(); got != "Billing API" {
		t.Errorf("BrandOrDefault() = %q, want the page title", got)
	}
}

func TestPublishedViewsNameNoProductAndDrawTheLogoOnlyWhenConfigured(t *testing.T) {
	t.Parallel()

	module := newModule(t, enabledConfig())
	for _, publication := range module.Publishes() {
		err := fs.WalkDir(publication.Files, publication.From, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			source, err := fs.ReadFile(publication.Files, path)
			if err != nil {
				return err
			}
			assertNoProductTrace(t, path, string(source))

			if strings.HasSuffix(path, "/topbar.kyse.go") {
				text := string(source)
				guard := strings.Index(text, `@if(.LogoURLOrDefault() != "")`)
				image := strings.Index(text, "<img")
				if image < 0 || guard < 0 || guard > image {
					t.Errorf("%s draws the logo image without checking that one is configured:\n%s", path, text)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk published views: %v", err)
		}
	}
}
