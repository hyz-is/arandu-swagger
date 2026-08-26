package unit_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/routing"
	swagger "github.com/hyz-is/arandu-swagger"
)

func TestAcceptanceHesapePathConstraintsArePreservedExactly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		apply   func(*routing.Route) *routing.Route
		pattern string
	}{
		{
			name: "UUID",
			apply: func(route *routing.Route) *routing.Route {
				return route.WhereUuid("value")
			},
			pattern: `[\da-fA-F]{8}-[\da-fA-F]{4}-[\da-fA-F]{4}-[\da-fA-F]{4}-[\da-fA-F]{12}`,
		},
		{
			name: "ULID",
			apply: func(route *routing.Route) *routing.Route {
				return route.WhereUlid("value")
			},
			pattern: `[0-7][0-9a-hjkmnp-tv-zA-HJKMNP-TV-Z]{25}`,
		},
		{
			name: "allowed values",
			apply: func(route *routing.Route) *routing.Route {
				return route.WhereIn("value", "draft", "published")
			},
			pattern: `draft|published`,
		},
		{
			name: "generic portable expression",
			apply: func(route *routing.Route) *routing.Route {
				return route.Where("value", `[A-Z]{2}[0-9]+`)
			},
			pattern: `[A-Z]{2}[0-9]+`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			router := routing.NewRouter()
			route := test.apply(router.Get(
				"/values/{value}",
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
			)).Name("values.show")
			registry := swagger.NewRegistry()
			registry.Route(route).Response(http.StatusOK, "Value")

			document, err := swagger.Generate(router.Routes(), registry, generationConfig())
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			parameter := document.Paths["/values/{value}"].Get.Parameters[0]
			encoded, err := json.Marshal(parameter.Schema)
			if err != nil {
				t.Fatalf("marshal inferred schema: %v", err)
			}
			var schema map[string]any
			if err := json.Unmarshal(encoded, &schema); err != nil {
				t.Fatalf("decode inferred schema: %v", err)
			}
			if got := schema["type"]; got != "string" {
				t.Errorf("schema type = %v, want string", got)
			}
			if got := schema["pattern"]; got != test.pattern {
				t.Errorf("schema pattern = %v, want %q", got, test.pattern)
			}
		})
	}
}

func TestAcceptanceEveryExclusionFilterOverridesAnEligibleRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		filter swagger.RouteFilter
	}{
		{name: "prefix", filter: swagger.RouteFilter{ExcludePrefixes: []string{"/blocked"}}},
		{name: "name", filter: swagger.RouteFilter{ExcludeNames: []string{"blocked.index"}}},
		{name: "module", filter: swagger.RouteFilter{ExcludeModules: []string{"blocked-module"}}},
		{name: "method", filter: swagger.RouteFilter{ExcludeMethods: []string{"get"}}},
		{name: "domain", filter: swagger.RouteFilter{ExcludeDomains: []string{"blocked.example.test"}}},
		{
			name: "predicate",
			filter: swagger.RouteFilter{ExcludePredicate: func(route *fhttp.Route) bool {
				return route.GetName() == "blocked.index"
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			router := routing.NewRouter()
			blocked := router.ForModule("blocked-module").Get(
				"/blocked/items",
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
			).Name("blocked.index").Domain("blocked.example.test")
			kept := router.ForModule("kept-module").Post(
				"/kept/items",
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
			).Name("kept.store").Domain("kept.example.test")
			registry := swagger.NewRegistry()
			registry.Route(blocked).Response(http.StatusOK, "Blocked")
			registry.Route(kept).Response(http.StatusCreated, "Kept")
			config := generationConfig()
			config.Filter = test.filter

			document, err := swagger.Generate(router.Routes(), registry, config)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			if _, exists := document.Paths["/blocked/items"]; exists {
				t.Error("excluded route was published")
			}
			if operation := document.Paths["/kept/items"]; operation == nil || operation.Post == nil {
				t.Fatalf("eligible route = %#v, want POST /kept/items", operation)
			}
		})
	}
}

func TestAcceptanceInternalRoutesRequireAnExplicitOptIn(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter().ForModule("foundation")
	route := router.Get(
		"/_arandu/health",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).Name("arandu.health")
	registry := swagger.NewRegistry()
	registry.Route(route).Response(http.StatusOK, "Healthy")

	config := generationConfig()
	config.IncludeInternal = true
	document, err := swagger.Generate(router.Routes(), registry, config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if item := document.Paths["/_arandu/health"]; item == nil || item.Get == nil {
		t.Fatalf("internal route = %#v, want opted-in GET operation", item)
	}
}

func TestAcceptanceUnnamedRoutesDoNotInventOperationIdentifiers(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	explicit := router.Get(
		"/explicit-unnamed",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)
	router.Post(
		"/undocumented-unnamed",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)
	registry := swagger.NewRegistry()
	registry.Route(explicit).Response(http.StatusNoContent, "Done")
	config := generationConfig()
	config.IncludeUndocumented = true

	document, err := swagger.Generate(router.Routes(), registry, config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	operations := []*swagger.Operation{
		document.Paths["/explicit-unnamed"].Get,
		document.Paths["/undocumented-unnamed"].Post,
	}
	for _, operation := range operations {
		if operation == nil {
			t.Fatal("unnamed route was omitted")
		}
		if operation.OperationID != "" {
			t.Errorf("unnamed route operationId = %q, want empty", operation.OperationID)
		}
		encoded, err := json.Marshal(operation)
		if err != nil {
			t.Fatalf("marshal unnamed operation: %v", err)
		}
		if strings.Contains(string(encoded), "x-arandu-route-name") {
			t.Errorf("unnamed operation invented route metadata: %s", encoded)
		}
	}
}

func TestAcceptanceUndocumentedMatchRoutesHaveMethodQualifiedOperationIDs(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	router.Match(
		[]string{http.MethodPut, http.MethodPatch},
		"/posts/{id}",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).Name("posts.update")
	config := generationConfig()
	config.IncludeUndocumented = true

	document, err := swagger.Generate(router.Routes(), swagger.NewRegistry(), config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	item := document.Paths["/posts/{id}"]
	if item == nil || item.Put == nil || item.Patch == nil {
		t.Fatalf("undocumented Match path item = %#v, want PUT and PATCH", item)
	}
	if item.Put.OperationID != "posts.update.put" || item.Patch.OperationID != "posts.update.patch" {
		t.Fatalf("operation IDs = %q, %q, want method-qualified IDs", item.Put.OperationID, item.Patch.OperationID)
	}
}

func TestAcceptanceUndocumentedMatchLinkageDoesNotCrossRouteNames(t *testing.T) {
	t.Parallel()

	matched := routing.NewRouter()
	matched.Match(
		[]string{http.MethodPut, http.MethodPatch},
		"/posts/{id}",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).Name("posts.update").Domain("matched.example.test")
	independent := routing.NewRouter()
	independent.Put(
		"/posts/{id}",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).Name("posts.replace").Domain("independent.example.test")
	routes := append([]*routing.Route(nil), matched.Routes()...)
	routes = append(routes, independent.Routes()...)
	config := generationConfig()
	config.IncludeUndocumented = true
	config.Filter.IncludeDomains = []string{"independent.example.test"}

	document, err := swagger.Generate(routes, swagger.NewRegistry(), config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	item := document.Paths["/posts/{id}"]
	if item == nil || item.Put == nil {
		t.Fatalf("independent path item = %#v, want PUT", item)
	}
	if item.Put.OperationID != "posts.replace" {
		t.Fatalf("operationId = %q, want single-method route name", item.Put.OperationID)
	}
}

func TestAcceptanceUndocumentedMatchLinkageDoesNotCrossDomains(t *testing.T) {
	t.Parallel()

	matched := routing.NewRouter()
	matched.Match(
		[]string{http.MethodPut, http.MethodPatch},
		"/posts/{id}",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).Name("posts.update").Domain("matched.example.test")
	sameName := routing.NewRouter()
	sameName.Put(
		"/posts/{id}",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).Name("posts.update").Domain("independent.example.test")
	routes := append([]*routing.Route(nil), matched.Routes()...)
	routes = append(routes, sameName.Routes()...)
	config := generationConfig()
	config.IncludeUndocumented = true
	config.Filter.IncludeDomains = []string{"independent.example.test"}

	document, err := swagger.Generate(routes, swagger.NewRegistry(), config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	item := document.Paths["/posts/{id}"]
	if item == nil || item.Put == nil {
		t.Fatalf("same-name other-domain path item = %#v, want PUT", item)
	}
	if item.Put.OperationID != "posts.update" {
		t.Fatalf("operationId = %q, want the unqualified name of a single-method route", item.Put.OperationID)
	}
}

func TestAcceptanceOneExplicitOperationIDCannotDescribeMultipleMethods(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	route := router.Match(
		[]string{http.MethodGet, http.MethodPost},
		"/search",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).Name("search")
	registry := swagger.NewRegistry()
	registry.Route(route).
		OperationID("search.execute").
		Response(http.StatusOK, "Search results")

	_, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err == nil {
		t.Fatal("Generate() error = nil, want an ambiguous operationId error")
	}
	for _, fragment := range []string{`operationId "search.execute"`, "GET /search", "POST /search"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("Generate() error = %q, want %q", err, fragment)
		}
	}
}
