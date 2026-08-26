package feature_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/arandu-io/framework/foundation"
	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/swagger"
)

func TestModuleImplementsOnlyItsIntendedFoundationContracts(t *testing.T) {
	t.Parallel()

	module := newModule(t, swagger.Config{})
	if _, ok := any(module).(foundation.Module); !ok {
		t.Fatal("Module does not implement foundation.Module")
	}
	if _, ok := any(module).(foundation.Diagnostic); !ok {
		t.Fatal("Module does not implement foundation.Diagnostic")
	}
	if _, ok := any(module).(foundation.Migratable); ok {
		t.Fatal("documentation-only module unexpectedly implements foundation.Migratable")
	}
	if got := module.Name(); got != "swagger" {
		t.Fatalf("Name() = %q, want swagger", got)
	}
}

func TestProgrammaticGenerationRequiresTheApplicationRouter(t *testing.T) {
	t.Parallel()

	module := newModule(t, enabledConfig())
	if _, err := module.Generate(); err == nil || !strings.Contains(err.Error(), "Routes") {
		t.Fatalf("Generate() error = %v, want a Routes lifecycle error", err)
	}
	if _, err := module.GenerateJSON(); err == nil || !strings.Contains(err.Error(), "Routes") {
		t.Fatalf("GenerateJSON() error = %v, want a Routes lifecycle error", err)
	}
}

func TestDisabledDocumentationRegistersNoHTTPRoutes(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	module := newModule(t, swagger.Config{})
	module.Routes(router.ForModule(module.Name()))

	if routes := router.Routes(); len(routes) != 0 {
		t.Fatalf("disabled module registered %d routes, want none", len(routes))
	}
	for _, target := range []string{swagger.DefaultUIPath, swagger.DefaultSpecPath} {
		if response := request(router, target); response.Code != http.StatusNotFound {
			t.Errorf("GET %s answered %d, want 404", target, response.Code)
		}
	}
}

func TestProgrammaticOnlyConfigurationStillGenerates(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.DisableUI = true
	config.DisableSpec = true
	router := fhttp.NewRouter()
	route := router.Get("/health", emptyHandler).Name("health")
	module := newModule(t, config)
	module.Route(route).Response(http.StatusNoContent, "Healthy")
	module.Routes(router.ForModule(module.Name()))

	document, err := module.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if item := document.Paths["/health"]; item == nil || item.Get == nil {
		t.Fatalf("generated /health operation = %#v", item)
	}
	if response := request(router, swagger.DefaultUIPath); response.Code != http.StatusNotFound {
		t.Fatalf("disabled UI answered %d, want 404", response.Code)
	}
}

func TestFirstSpecificationRequestSeesRoutesRegisteredAfterSwagger(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.DisableUI = true
	router := fhttp.NewRouter()
	module := newModule(t, config)
	module.Routes(router.ForModule(module.Name()))

	late := router.ForModule("health").Get("/health", emptyHandler).Name("health.show")
	module.Route(late).Response(http.StatusNoContent, "Healthy")

	response := request(router, swagger.DefaultSpecPath)
	if response.Code != http.StatusOK {
		t.Fatalf("specification answered %d: %s", response.Code, response.Body.String())
	}
	assertHeader(t, response, "Content-Type", "application/json")
	assertHeader(t, response, "Cache-Control", "no-store")
	assertHeader(t, response, "X-Content-Type-Options", "nosniff")

	var document swagger.Document
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode specification: %v", err)
	}
	if item := document.Paths["/health"]; item == nil || item.Get == nil {
		t.Fatalf("late route operation = %#v", item)
	}
}

func TestUIIsSelfHostedCanonicalAndProtectedByStrictHeaders(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.Title = "Example <API>"
	config.PersistAuthorization = true
	config.DisableTryItOut = true
	router, _ := mount(t, config)

	page := request(router, swagger.DefaultUIPath)
	if page.Code != http.StatusOK {
		t.Fatalf("UI answered %d: %s", page.Code, page.Body.String())
	}
	assertHeader(t, page, "Content-Type", "text/html; charset=utf-8")
	assertHeader(t, page, "Cache-Control", "no-store")
	assertHeader(t, page, "X-Content-Type-Options", "nosniff")
	csp := page.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || !strings.Contains(csp, "connect-src 'self'") || !strings.Contains(csp, "style-src-attr 'unsafe-inline'") || strings.Contains(csp, "script-src 'self' 'unsafe-inline'") || strings.Contains(csp, "connect-src 'self' http:") {
		t.Fatalf("Content-Security-Policy = %q, want same-origin scripts/connections and only the required style-attribute exception", csp)
	}
	for _, fragment := range []string{
		"Example &lt;API&gt;",
		"/docs/assets/5.32.14/swagger-ui.css",
		"/docs/assets/5.32.14/swagger-ui-bundle.js",
		"/docs/swagger-initializer.js",
	} {
		if !strings.Contains(page.Body.String(), fragment) {
			t.Errorf("UI HTML does not contain %q", fragment)
		}
	}
	if strings.Contains(page.Body.String(), "https://") || strings.Contains(page.Body.String(), "<script>") {
		t.Fatalf("UI HTML contains a remote or inline script: %s", page.Body.String())
	}

	initializer := request(router, "/docs/swagger-initializer.js")
	if initializer.Code != http.StatusOK {
		t.Fatalf("initializer answered %d", initializer.Code)
	}
	assertHeader(t, initializer, "Content-Type", "text/javascript; charset=utf-8")
	assertHeader(t, initializer, "Cache-Control", "no-store")
	for _, fragment := range []string{
		`url: "/docs/openapi.json"`,
		"validatorUrl: null",
		`layout: "BaseLayout"`,
		"SwaggerUIBundle.presets.apis",
		"persistAuthorization: true",
		"supportedSubmitMethods: []",
	} {
		if !strings.Contains(initializer.Body.String(), fragment) {
			t.Errorf("initializer does not contain %q: %s", fragment, initializer.Body.String())
		}
	}

	stylesheet := request(router, "/docs/assets/5.32.14/swagger-ui.css")
	if stylesheet.Code != http.StatusOK || stylesheet.Body.Len() == 0 {
		t.Fatalf("stylesheet answered %d with %d bytes", stylesheet.Code, stylesheet.Body.Len())
	}
	assertHeader(t, stylesheet, "Content-Type", "text/css; charset=utf-8")
	assertHeader(t, stylesheet, "Cache-Control", "public, max-age=31536000, immutable")
	assertHeader(t, stylesheet, "X-Content-Type-Options", "nosniff")

	trailingSlash := request(router, "/docs/")
	if trailingSlash.Code != http.StatusPermanentRedirect || trailingSlash.Header().Get("Location") != "/docs" {
		t.Fatalf("GET /docs/ answered %d Location=%q, want 308 /docs", trailingSlash.Code, trailingSlash.Header().Get("Location"))
	}
	if unknown := request(router, "/docs/assets/5.32.14/not-vendored.js"); unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown asset answered %d, want 404", unknown.Code)
	}
}

func TestUIAndSpecificationMiddlewareRemainIsolated(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.UIMiddleware = []swagger.Middleware{headerMiddleware("X-UI-Middleware", "applied")}
	config.SpecMiddleware = []swagger.Middleware{headerMiddleware("X-Spec-Middleware", "applied")}
	router, _ := mount(t, config)

	for _, target := range []string{"/docs", "/docs/swagger-initializer.js", "/docs/assets/5.32.14/swagger-ui.css"} {
		response := request(router, target)
		assertHeader(t, response, "X-UI-Middleware", "applied")
		if got := response.Header().Get("X-Spec-Middleware"); got != "" {
			t.Errorf("GET %s received specification middleware header %q", target, got)
		}
	}

	specification := request(router, swagger.DefaultSpecPath)
	assertHeader(t, specification, "X-Spec-Middleware", "applied")
	if got := specification.Header().Get("X-UI-Middleware"); got != "" {
		t.Fatalf("specification received UI middleware header %q", got)
	}
}

func TestEndpointSwitchesAreIndependent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		configure func(*swagger.Config)
		wantUI    int
		wantSpec  int
	}{
		{
			name: "UI disabled",
			configure: func(config *swagger.Config) {
				config.DisableUI = true
			},
			wantUI: http.StatusNotFound, wantSpec: http.StatusOK,
		},
		{
			name: "both endpoints disabled",
			configure: func(config *swagger.Config) {
				config.DisableUI = true
				config.DisableSpec = true
			},
			wantUI: http.StatusNotFound, wantSpec: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			config := enabledConfig()
			test.configure(&config)
			router, _ := mount(t, config)
			if got := request(router, swagger.DefaultUIPath).Code; got != test.wantUI {
				t.Errorf("UI status = %d, want %d", got, test.wantUI)
			}
			if got := request(router, swagger.DefaultSpecPath).Code; got != test.wantSpec {
				t.Errorf("spec status = %d, want %d", got, test.wantSpec)
			}
		})
	}
}

func TestTwoInstancesKeepTheirEndpointsAndRegistriesIsolated(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	firstRoute := router.ForModule("first").Get("/first", emptyHandler).Name("first.index")
	secondRoute := router.ForModule("second").Get("/second", emptyHandler).Name("second.index")

	firstConfig := enabledConfig()
	firstConfig.Title = "First API"
	firstConfig.UIPath = "/first-docs"
	secondConfig := enabledConfig()
	secondConfig.Title = "Second API"
	secondConfig.UIPath = "/second-docs"

	first := newModule(t, firstConfig)
	second := newModule(t, secondConfig)
	first.Route(firstRoute).Response(http.StatusOK, "First")
	second.Route(secondRoute).Response(http.StatusOK, "Second")
	first.Routes(router.ForModule(first.Name()))
	second.Routes(router.ForModule(second.Name()))

	assertPaths(t, request(router, "/first-docs/openapi.json"), "/first", "/second")
	assertPaths(t, request(router, "/second-docs/openapi.json"), "/second", "/first")

	firstPage := request(router, "/first-docs")
	secondPage := request(router, "/second-docs")
	if !strings.Contains(firstPage.Body.String(), "First API") || strings.Contains(firstPage.Body.String(), "Second API") {
		t.Fatalf("first UI has the wrong identity: %s", firstPage.Body.String())
	}
	if !strings.Contains(secondPage.Body.String(), "Second API") || strings.Contains(secondPage.Body.String(), "First API") {
		t.Fatalf("second UI has the wrong identity: %s", secondPage.Body.String())
	}
	firstInitializer := request(router, "/first-docs/swagger-initializer.js").Body.String()
	secondInitializer := request(router, "/second-docs/swagger-initializer.js").Body.String()
	if !strings.Contains(firstInitializer, `"/first-docs/openapi.json"`) || strings.Contains(firstInitializer, `"/second-docs/openapi.json"`) {
		t.Fatalf("first initializer has the wrong specification URL: %s", firstInitializer)
	}
	if !strings.Contains(secondInitializer, `"/second-docs/openapi.json"`) || strings.Contains(secondInitializer, `"/first-docs/openapi.json"`) {
		t.Fatalf("second initializer has the wrong specification URL: %s", secondInitializer)
	}
}

func TestCachedSpecificationIsConcurrentAndInvalidatedByBuilderChanges(t *testing.T) {
	t.Parallel()

	var predicateCalls atomic.Int64
	config := enabledConfig()
	config.DisableUI = true
	config.CacheSpec = true
	config.Filter.IncludePredicate = func(*fhttp.Route) bool {
		predicateCalls.Add(1)
		return true
	}

	router := fhttp.NewRouter()
	route := router.ForModule("health").Get("/health", emptyHandler).Name("health")
	module := newModule(t, config)
	builder := module.Route(route).Response(http.StatusNoContent, "Healthy")
	module.Routes(router.ForModule(module.Name()))

	first := request(router, swagger.DefaultSpecPath)
	if first.Code != http.StatusOK {
		t.Fatalf("warm specification answered %d: %s", first.Code, first.Body.String())
	}
	warmCalls := predicateCalls.Load()
	if warmCalls == 0 {
		t.Fatal("route predicate was not evaluated during initial generation")
	}

	const readers = 24
	errors := make(chan error, readers)
	var wait sync.WaitGroup
	for range readers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			response := request(router, swagger.DefaultSpecPath)
			if response.Code != http.StatusOK {
				errors <- fmt.Errorf("cached specification answered %d", response.Code)
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if got := predicateCalls.Load(); got != warmCalls {
		t.Fatalf("cached requests reevaluated the filter: calls = %d, want %d", got, warmCalls)
	}

	builder.Summary("Current health")
	updated := request(router, swagger.DefaultSpecPath)
	if updated.Code != http.StatusOK {
		t.Fatalf("updated specification answered %d: %s", updated.Code, updated.Body.String())
	}
	if predicateCalls.Load() <= warmCalls {
		t.Fatal("builder mutation did not invalidate the cached specification")
	}
	if !strings.Contains(updated.Body.String(), `"summary":"Current health"`) {
		t.Fatalf("updated specification did not contain the new summary: %s", updated.Body.String())
	}
}

func TestCachedSpecificationIsInvalidatedByRouteTableChanges(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.DisableUI = true
	config.CacheSpec = true
	config.IncludeUndocumented = true
	router := fhttp.NewRouter()
	router.ForModule("health").Get("/health", emptyHandler).Name("health")
	module := newModule(t, config)
	module.Routes(router.ForModule(module.Name()))

	first := request(router, swagger.DefaultSpecPath)
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"/health"`) {
		t.Fatalf("warm specification = %d %s", first.Code, first.Body.String())
	}

	router.ForModule("metrics").Get("/metrics", emptyHandler).Name("metrics")
	updated := request(router, swagger.DefaultSpecPath)
	if updated.Code != http.StatusOK {
		t.Fatalf("updated specification answered %d: %s", updated.Code, updated.Body.String())
	}
	if !strings.Contains(updated.Body.String(), `"/metrics"`) {
		t.Fatalf("route-table mutation did not invalidate the cache: %s", updated.Body.String())
	}
}

func TestNewSnapshotsMutableConfiguration(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.DisableUI = true
	config.Contact = &swagger.Contact{Name: "Original contact"}
	config.Servers = []swagger.Server{{URL: "https://api.example.test"}}
	config.Filter.IncludePrefixes = []string{"/public"}
	module := newModule(t, config)

	config.Contact.Name = "Mutated contact"
	config.Servers[0].URL = "https://mutated.example.test"
	config.Filter.IncludePrefixes[0] = "/private"

	router := fhttp.NewRouter()
	route := router.Get("/public/health", emptyHandler).Name("health")
	module.Route(route).Response(http.StatusNoContent, "Healthy")
	module.Routes(router.ForModule(module.Name()))

	response := request(router, swagger.DefaultSpecPath)
	if response.Code != http.StatusOK {
		t.Fatalf("specification answered %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{"Original contact", "https://api.example.test", `"/public/health"`} {
		if !strings.Contains(body, want) {
			t.Errorf("snapshotted specification does not contain %q: %s", want, body)
		}
	}
	for _, absent := range []string{"Mutated contact", "https://mutated.example.test"} {
		if strings.Contains(body, absent) {
			t.Errorf("snapshotted specification contains later mutation %q: %s", absent, body)
		}
	}
}

func TestUncachedSpecificationIsRegeneratedPerRequest(t *testing.T) {
	t.Parallel()

	var predicateCalls atomic.Int64
	config := enabledConfig()
	config.DisableUI = true
	config.Filter.IncludePredicate = func(*fhttp.Route) bool {
		predicateCalls.Add(1)
		return true
	}
	router := fhttp.NewRouter()
	route := router.ForModule("health").Get("/health", emptyHandler).Name("health")
	module := newModule(t, config)
	module.Route(route).Response(http.StatusNoContent, "Healthy")
	module.Routes(router.ForModule(module.Name()))

	if response := request(router, swagger.DefaultSpecPath); response.Code != http.StatusOK {
		t.Fatalf("first specification answered %d", response.Code)
	}
	firstCalls := predicateCalls.Load()
	if response := request(router, swagger.DefaultSpecPath); response.Code != http.StatusOK {
		t.Fatalf("second specification answered %d", response.Code)
	}
	if got := predicateCalls.Load(); got <= firstCalls {
		t.Fatalf("uncached second request did not regenerate: calls = %d, first = %d", got, firstCalls)
	}
}

func TestGenerationErrorsAreSanitizedAndRemainDiagnosable(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.DisableUI = true
	router := fhttp.NewRouter()
	route := router.Get("/broken", emptyHandler).Name("broken")
	module := newModule(t, config)
	module.Route(route).Response(http.StatusOK, "")
	module.Routes(router.ForModule(module.Name()))

	response := request(router, swagger.DefaultSpecPath)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("broken generation answered %d: %s", response.Code, response.Body.String())
	}
	assertHeader(t, response, "Content-Type", "application/json")
	if got := strings.TrimSpace(response.Body.String()); got != `{"error":"failed to generate OpenAPI document"}` {
		t.Fatalf("public error = %q", got)
	}
	if strings.Contains(strings.ToLower(response.Body.String()), "description") {
		t.Fatalf("public error leaked generation details: %s", response.Body.String())
	}

	diagnoses := module.Diagnose(context.Background())
	if len(diagnoses) != 1 || !strings.Contains(strings.ToLower(diagnoses[0]), "description") {
		t.Fatalf("Diagnose() = %#v, want the actionable generation error", diagnoses)
	}
}

var emptyHandler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

func enabledConfig() swagger.Config {
	return swagger.Config{
		Enabled: true,
		Title:   "Example API",
		Version: "1.0.0",
	}
}

func newModule(t *testing.T, config swagger.Config) *swagger.Module {
	t.Helper()
	module, err := swagger.New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return module
}

func mount(t *testing.T, config swagger.Config) (*fhttp.Router, *swagger.Module) {
	t.Helper()
	router := fhttp.NewRouter()
	module := newModule(t, config)
	module.Routes(router.ForModule(module.Name()))
	return router, module
}

func request(router *fhttp.Router, target string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	return response
}

func assertHeader(t *testing.T, response *httptest.ResponseRecorder, name, want string) {
	t.Helper()
	if got := response.Header().Get(name); got != want {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

func headerMiddleware(name, value string) swagger.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			w.Header().Set(name, value)
			next.ServeHTTP(w, request)
		})
	}
}

func assertPaths(t *testing.T, response *httptest.ResponseRecorder, present, absent string) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("specification answered %d: %s", response.Code, response.Body.String())
	}
	var document swagger.Document
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode specification: %v", err)
	}
	if _, ok := document.Paths[present]; !ok {
		t.Errorf("document does not contain %s", present)
	}
	if _, ok := document.Paths[absent]; ok {
		t.Errorf("document unexpectedly contains %s", absent)
	}
}
