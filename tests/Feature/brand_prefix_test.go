package feature_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	swagger "github.com/hyz-is/arandu-swagger"
)

// An application whose customers read the document names its own extensions:
// the served document and the exported contract carry x-acme-*, and not one
// x-arandu-*.
func TestExtensionPrefixNamesTheDocumentsExtensions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	config := exportConfig(dir)
	config.ExtensionPrefix = "acme"
	router, module := mount(t, config)
	documentOrders(router, module)

	served := request(router, swagger.DefaultSpecPath)
	if served.Code != http.StatusOK {
		t.Fatalf("specification status = %d", served.Code)
	}
	if _, err := runExport(t, module); err != nil {
		t.Fatalf("openapi:export error = %v", err)
	}
	written, err := os.ReadFile(filepath.Join(dir, "api.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"served": served.Body.String(), "exported": string(written)} {
		if strings.Contains(body, "arandu") {
			t.Errorf("the %s document names arandu:\n%s", name, body)
		}
		if !strings.Contains(body, `"x-acme-route-name": "orders.index"`) && !strings.Contains(body, `"x-acme-route-name":"orders.index"`) {
			t.Errorf("the %s document lost x-acme-route-name:\n%s", name, body)
		}
	}
	if !strings.Contains(served.Body.String(), `"x-acme-module":"orders"`) {
		t.Errorf("the served document lost x-acme-module:\n%s", served.Body.String())
	}
}

// OmitRouteNames publishes no route name: the operationId identifies the
// operation, and the document carries no extension of the route's name at all.
func TestOmitRouteNamesLeavesTheRouteNameOutOfBothDocuments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	config := exportConfig(dir)
	config.OmitRouteNames = true
	router, module := mount(t, config)
	documentOrders(router, module)

	served := request(router, swagger.DefaultSpecPath)
	if _, err := runExport(t, module); err != nil {
		t.Fatalf("openapi:export error = %v", err)
	}
	written, err := os.ReadFile(filepath.Join(dir, "api.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"served": served.Body.String(), "exported": string(written)} {
		if strings.Contains(body, "route-name") {
			t.Errorf("the %s document carries a route-name extension:\n%s", name, body)
		}
		if !strings.Contains(body, `"operationId"`) || !strings.Contains(body, "orders.index") {
			t.Errorf("the %s document lost the operationId:\n%s", name, body)
		}
	}
	if out, err := runExport(t, module, "--check"); err != nil {
		t.Fatalf("--check = %v (%s)", err, out)
	}
}

// The default prefix is unchanged, so a document nobody configured is the
// document earlier releases wrote.
func TestExtensionPrefixDefaultsToArandu(t *testing.T) {
	t.Parallel()

	router, module := mount(t, exportConfig(t.TempDir()))
	documentOrders(router, module)
	served := request(router, swagger.DefaultSpecPath).Body.String()
	if !strings.Contains(served, `"x-arandu-route-name":"orders.index"`) {
		t.Errorf("the default document lost x-arandu-route-name:\n%s", served)
	}
}

// A prefix that cannot be an extension name or a CSS class is refused when the
// module is built.
func TestPrefixesAreValidated(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{"Acme", "1acme", "ac me", "ac_me", "acme.", "-acme", "ac\"me"} {
		config := enabledConfig()
		config.ExtensionPrefix = bad
		if _, err := swagger.New(config); err == nil {
			t.Errorf("ExtensionPrefix %q was accepted", bad)
		}
		config = enabledConfig()
		config.Theme.ClassPrefix = bad
		if _, err := swagger.New(config); err == nil {
			t.Errorf("Theme.ClassPrefix %q was accepted", bad)
		}
	}
	config := enabledConfig()
	config.ExtensionPrefix = "acme-2"
	config.Theme.ClassPrefix = "acme-docs"
	if _, err := swagger.New(config); err != nil {
		t.Errorf("a valid prefix was refused: %v", err)
	}
}

// Theme.ClassPrefix renames every class, the data attribute and the storage
// key the page, the stylesheet and the initializer share, on the embedded page
// and on a published view alike, and none of the three says arandu.
func TestClassPrefixRenamesThePageTheStylesheetAndTheInitializer(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.Theme.ClassPrefix = "acme-docs"
	config.Theme.Logo = &swagger.ThemeLogo{URL: "/logo.svg", Alt: "Acme"}

	embedded := embeddedConfig()
	embedded.Theme = config.Theme
	pages := map[string]string{}
	embeddedRouter, _ := mount(t, embedded)
	pages["embedded page"] = request(embeddedRouter, "/docs").Body.String()
	viewRouter := mountRendered(t, config)
	response := request(viewRouter, "/docs")
	if response.Code != http.StatusOK {
		t.Fatalf("GET /docs answered %d", response.Code)
	}
	pages["published view"] = response.Body.String()
	router, _ := mount(t, config)
	pages["initializer"] = request(router, "/docs/swagger-initializer.js").Body.String()
	pages["stylesheet"] = request(router, "/docs/theme.css").Body.String()

	for name, body := range pages {
		if strings.Contains(strings.ToLower(body), "arandu") {
			t.Errorf("the %s names arandu", name)
		}
	}
	for _, want := range []string{
		`class="acme-docs-topbar"`, `class="acme-docs-logo-link"`, `class="acme-docs-theme-toggle"`,
		`class="acme-docs-glyph-light"`,
	} {
		for _, name := range []string{"embedded page", "published view"} {
			if !strings.Contains(pages[name], want) {
				t.Errorf("the %s lacks %s", name, want)
			}
		}
	}
	if !strings.Contains(pages["published view"], `class="acme-docs-container"`) {
		t.Errorf("the published view lacks the container class:\n%s", pages["published view"])
	}
	for _, want := range []string{`".acme-docs-theme-toggle"`, `"acme-docs-theme"`, `[data-acme-docs-authorize]`, `.acme-docs-authorize`} {
		if !strings.Contains(pages["initializer"], want) {
			t.Errorf("the initializer lacks %s:\n%s", want, pages["initializer"])
		}
	}
	for _, want := range []string{".acme-docs-topbar {", ".acme-docs-theme-toggle {", ".acme-docs-glyph-light"} {
		if !strings.Contains(pages["stylesheet"], want) {
			t.Errorf("the stylesheet lacks %s", want)
		}
	}
}
