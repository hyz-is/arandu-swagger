package unit_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/jsonschema"
	"github.com/arandu-io/hesape/routing"
	"github.com/arandu-io/swagger"
)

func TestGenerateBuildsGlobalInformationAndDeterministicJSON(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter().ForModule("users")
	route := router.Get("/users", func(http.ResponseWriter, *http.Request) {}).Name("users.index")
	registry := swagger.NewRegistry()
	registry.Route(route).Response(http.StatusOK, "Users")
	config := generationConfig()
	config.Summary = "User API"
	config.Description = "Manages users."
	config.TermsOfService = "https://example.test/terms"
	config.Contact = &swagger.Contact{Name: "API team", Email: "api@example.test"}
	config.License = &swagger.License{Name: "MIT", Identifier: "MIT"}
	config.Servers = []swagger.Server{{URL: "https://api.example.test"}}
	config.Tags = []swagger.Tag{{Name: "Users", Description: "User operations"}}
	config.ExternalDocs = &swagger.ExternalDocumentation{URL: "https://example.test/docs"}

	first, err := swagger.GenerateJSON(router.Routes(), registry, config)
	if err != nil {
		t.Fatalf("first GenerateJSON() error = %v", err)
	}
	second, err := swagger.GenerateJSON(router.Routes(), registry, config)
	if err != nil {
		t.Fatalf("second GenerateJSON() error = %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("JSON changed between runs:\nfirst:  %s\nsecond: %s", first, second)
	}
	var document swagger.Document
	if err := json.Unmarshal(first, &document); err != nil {
		t.Fatalf("decode generated OpenAPI JSON: %v", err)
	}
	if document.OpenAPI != "3.1.0" || document.Info.Title != config.Title || document.Info.Version != config.Version {
		t.Fatalf("document identity = %#v", document)
	}
	if len(document.Servers) != 1 || len(document.Tags) != 1 || document.ExternalDocs == nil {
		t.Fatalf("global metadata was not preserved: %#v", document)
	}
}

func TestOnlyDocumentedRoutesAreIncludedByDefault(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	documented := router.Get("/documented", func(http.ResponseWriter, *http.Request) {}).Name("documented")
	router.Get("/undocumented", func(http.ResponseWriter, *http.Request) {}).Name("undocumented")
	registry := swagger.NewRegistry()
	registry.Route(documented).Response(http.StatusNoContent, "Done")

	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if _, ok := document.Paths["/documented"]; !ok {
		t.Error("documented route is absent")
	}
	if _, ok := document.Paths["/undocumented"]; ok {
		t.Error("undocumented route was published")
	}

	config := generationConfig()
	config.IncludeUndocumented = true
	document, err = swagger.Generate(router.Routes(), registry, config)
	if err != nil {
		t.Fatalf("Generate() with IncludeUndocumented error = %v", err)
	}
	operation := document.Paths["/undocumented"].Get
	if operation == nil || operation.OperationID != "undocumented" || operation.Responses["default"].Description == "" {
		t.Fatalf("basic undocumented operation = %#v", operation)
	}
}

func TestInternalFallbackHiddenAndSwaggerRoutesStayExcluded(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	internal := router.Get("/_arandu/health", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("internal.health")
	fallback := router.Fallback(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	own := router.ForModule("swagger").Get("/docs/openapi.json", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("swagger.spec")
	hidden := router.Get("/secret", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("secret")
	registry := swagger.NewRegistry()
	for _, route := range []*routing.Route{internal, fallback, own} {
		registry.Route(route).Response(http.StatusOK, "OK")
	}
	registry.Route(hidden).Hidden().Response(http.StatusOK, "Secret")

	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(document.Paths) != 0 {
		t.Fatalf("excluded paths = %#v, want none", document.Paths)
	}
}

func TestRouteFiltersApplyEverySupportedDimension(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter().ForModule("users")
	wanted := router.Get("/api/users", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("users.index").Domain("api.example.test")
	other := router.Post("/api/users", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("users.store").Domain("api.example.test")
	registry := swagger.NewRegistry()
	registry.Route(wanted).Response(http.StatusOK, "Users")
	registry.Route(other).Response(http.StatusCreated, "Created")
	config := generationConfig()
	config.Filter = swagger.RouteFilter{
		IncludePrefixes: []string{"/api"},
		IncludeNames:    []string{"users.index"},
		IncludeModules:  []string{"users"},
		IncludeMethods:  []string{"GET"},
		IncludeDomains:  []string{"api.example.test"},
		IncludePredicate: func(route *fhttp.Route) bool {
			return route.GetName() == "users.index"
		},
	}

	document, err := swagger.Generate(router.Routes(), registry, config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	item := document.Paths["/api/users"]
	if item == nil || item.Get == nil || item.Post != nil {
		t.Fatalf("filtered path item = %#v", item)
	}
}

func TestPathParametersAndConstraintsAreInferredConservatively(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	route := router.Get("/users/{id}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		Name("users.show").
		WhereNumber("id")
	registry := swagger.NewRegistry()
	registry.Route(route).Response(http.StatusOK, "User")
	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	parameter := document.Paths["/users/{id}"].Get.Parameters[0]
	if !parameter.Required || parameter.In != swagger.ParameterInPath {
		t.Fatalf("path parameter = %#v", parameter)
	}
	encoded, err := json.Marshal(parameter.Schema)
	if err != nil {
		t.Fatalf("marshal parameter schema: %v", err)
	}
	if got := string(encoded); !strings.Contains(got, `"type":"string"`) || !strings.Contains(got, `"pattern":"[0-9]+"`) {
		t.Fatalf("constraint schema = %s, want string with exact pattern", got)
	}
}

func TestMatchProducesOneOperationPerConcreteMethod(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	route := router.Match([]string{http.MethodGet, http.MethodPost}, "/search", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("search")
	registry := swagger.NewRegistry()
	registry.Route(route).
		OperationIDFor(http.MethodGet, "search.read").
		OperationIDFor(http.MethodPost, "search.write").
		Response(http.StatusOK, "Search result")

	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	item := document.Paths["/search"]
	if item == nil || item.Get == nil || item.Post == nil {
		t.Fatalf("Match path item = %#v", item)
	}
	if item.Get.OperationID != "search.read" || item.Post.OperationID != "search.write" {
		t.Fatalf("Match operation IDs = %q, %q", item.Get.OperationID, item.Post.OperationID)
	}
}

func TestAutomaticMatchOperationIDsAreMethodQualified(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	route := router.Match([]string{http.MethodGet, http.MethodPost}, "/search", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("search")
	registry := swagger.NewRegistry()
	registry.Route(route).Response(http.StatusOK, "Search result")
	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if document.Paths["/search"].Get.OperationID != "search.get" || document.Paths["/search"].Post.OperationID != "search.post" {
		t.Fatalf("automatic IDs = %q, %q", document.Paths["/search"].Get.OperationID, document.Paths["/search"].Post.OperationID)
	}
}

func TestAnyRequiresAnExplicitConcreteMethodDecision(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	any := router.Any("/hook", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("hook")
	registry := swagger.NewRegistry()
	registry.Route(any).Response(http.StatusOK, "Accepted")
	if _, err := swagger.Generate(router.Routes(), registry, generationConfig()); err == nil || !strings.Contains(err.Error(), "ANY") {
		t.Fatalf("Generate() error = %v, want ANY diagnostic", err)
	}

	registry.Route(any).Methods(http.MethodPost)
	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() with explicit method error = %v", err)
	}
	if item := document.Paths["/hook"]; item == nil || item.Post == nil {
		t.Fatalf("explicit ANY operation = %#v", item)
	}
}

func TestHEADAndOPTIONSAreRepresentedOnlyWhenExplicit(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	get := router.Get("/health", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("health.get")
	head := router.Match([]string{http.MethodHead}, "/health", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("health.head")
	options := router.Options("/health", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("health.options")
	registry := swagger.NewRegistry()
	for _, route := range []*routing.Route{get, head, options} {
		registry.Route(route).Response(http.StatusNoContent, "Healthy")
	}
	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	item := document.Paths["/health"]
	if item.Get == nil || item.Head == nil || item.Options == nil {
		t.Fatalf("explicit methods = %#v", item)
	}

	getOnlyRouter := routing.NewRouter()
	getOnly := getOnlyRouter.Get("/ready", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("ready")
	getOnlyRegistry := swagger.NewRegistry()
	getOnlyRegistry.Route(getOnly).Response(http.StatusNoContent, "Ready")
	document, err = swagger.Generate(getOnlyRouter.Routes(), getOnlyRegistry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() GET-only error = %v", err)
	}
	if document.Paths["/ready"].Head != nil {
		t.Fatal("implicit ServeMux HEAD was documented")
	}
}

func TestUnsupportedRouteShapesReturnActionableErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		register func(*routing.Router) *routing.Route
		want     string
	}{
		{name: "optional parameter", register: func(router *routing.Router) *routing.Route {
			return router.NewRoute([]string{http.MethodGet}, "/users/{id?}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		}, want: "optional"},
		{name: "multi segment wildcard", register: func(router *routing.Router) *routing.Route {
			return router.NewRoute([]string{http.MethodGet}, "/files/{path...}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		}, want: "wildcard"},
		{name: "custom method", register: func(router *routing.Router) *routing.Route {
			return router.NewRoute([]string{"CONNECT"}, "/tunnel", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		}, want: "CONNECT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			router := routing.NewRouter()
			route := test.register(router)
			registry := swagger.NewRegistry()
			registry.Route(route).Response(http.StatusOK, "OK")
			if _, err := swagger.Generate([]*routing.Route{route}, registry, generationConfig()); err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.want)) {
				t.Fatalf("Generate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRootAnchorIsCanonicalizedWithoutInventingAParameter(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	route := router.Get("/api/{$}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("api.root")
	registry := swagger.NewRegistry()
	registry.Route(route).Response(http.StatusOK, "Docs")
	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	operation := document.Paths["/api/"].Get
	if operation == nil {
		t.Fatal("canonical /api/ operation is absent")
	}
	if len(operation.Parameters) != 0 {
		t.Fatalf("root anchor produced parameters: %#v", operation.Parameters)
	}
}

func TestDuplicateOperationIDsAndMethodPathsAreErrors(t *testing.T) {
	t.Parallel()

	firstRouter := routing.NewRouter()
	secondRouter := routing.NewRouter()
	first := firstRouter.NewRoute([]string{http.MethodGet}, "/one", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("one")
	second := secondRouter.NewRoute([]string{http.MethodGet}, "/two", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("two")
	registry := swagger.NewRegistry()
	registry.Route(first).OperationID("duplicate").Response(http.StatusOK, "One")
	registry.Route(second).OperationID("duplicate").Response(http.StatusOK, "Two")
	if _, err := swagger.Generate([]*routing.Route{first, second}, registry, generationConfig()); err == nil || !strings.Contains(err.Error(), "operationId") {
		t.Fatalf("duplicate operationId error = %v", err)
	}

	duplicate := secondRouter.NewRoute([]string{http.MethodGet}, "/one", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("different")
	registry.Route(duplicate).Response(http.StatusOK, "Duplicate")
	if _, err := swagger.Generate([]*routing.Route{first, duplicate}, registry, generationConfig()); err == nil || !strings.Contains(err.Error(), "GET /one") {
		t.Fatalf("duplicate method/path error = %v", err)
	}
}

func TestMissingSchemaAndSecurityReferencesAreErrors(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	route := router.Get("/users", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("users")
	registry := swagger.NewRegistry()
	registry.Route(route).
		Response(http.StatusOK, "Users", swagger.JSONRef("Missing"))
	registry.Route(route).Security("missingAuth")
	_, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err == nil {
		t.Fatal("Generate() error = nil, want missing references")
	}
	for _, fragment := range []string{"Missing", "missingAuth"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("error %q does not contain %q", err, fragment)
		}
	}
}

func TestDeprecationAndRouteMetadataArePreserved(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter().ForModule("legacy")
	since := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	sunset := since.AddDate(1, 0, 0)
	route := router.Get("/legacy", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		Name("legacy.index").
		Domain("legacy.example.test").
		Deprecated(since, sunset)
	registry := swagger.NewRegistry()
	registry.Route(route).Response(http.StatusOK, "Legacy")
	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	operation := document.Paths["/legacy"].Get
	if !operation.Deprecated || len(operation.Tags) != 1 || operation.Tags[0] != "legacy" {
		t.Fatalf("deprecated operation = %#v", operation)
	}
	encoded, err := json.Marshal(operation)
	if err != nil {
		t.Fatalf("marshal operation: %v", err)
	}
	for _, fragment := range []string{"x-arandu-domain", "x-arandu-deprecated-since", "x-arandu-sunset"} {
		if !strings.Contains(string(encoded), fragment) {
			t.Errorf("operation metadata does not contain %q: %s", fragment, encoded)
		}
	}
}

func TestExplicitPathParameterMustExistAndResponsesNeedDescriptions(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	route := router.Get("/users/{id}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("users.show")
	registry := swagger.NewRegistry()
	registry.Route(route).PathParameter("other").Response(http.StatusOK, "")
	_, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err == nil || !strings.Contains(err.Error(), "other") || !strings.Contains(err.Error(), "description") {
		t.Fatalf("Generate() error = %v, want parameter and response diagnostics", err)
	}
}

func TestARepresentableGenericConstraintMustBePortable(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	route := router.Get("/values/{value}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("values").Where("value", `(?i)yes|no`)
	registry := swagger.NewRegistry()
	registry.Route(route).Response(http.StatusOK, "Value")
	if _, err := swagger.Generate(router.Routes(), registry, generationConfig()); err == nil || !strings.Contains(err.Error(), "ECMAScript") {
		t.Fatalf("Generate() error = %v, want regex portability diagnostic", err)
	}
}

func TestReusableParameterAndResponseReferencesAreValidated(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	identifier, err := swagger.SchemaFrom(jsonschema.String().Format("uuid"))
	if err != nil {
		t.Fatalf("snapshot identifier: %v", err)
	}
	if err := registry.Parameter("TraceID", swagger.Parameter{Name: "X-Trace-ID", In: swagger.ParameterInHeader, Schema: &identifier}); err != nil {
		t.Fatalf("register parameter: %v", err)
	}
	if err := registry.Response("Problem", swagger.Response{Description: "Problem"}); err != nil {
		t.Fatalf("register response: %v", err)
	}

	router := routing.NewRouter()
	route := router.Get("/health", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("health")
	registry.Route(route).
		ParameterRef("TraceID").
		ResponseRef(http.StatusServiceUnavailable, "Problem")
	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	operation := document.Paths["/health"].Get
	if operation.Parameters[0].Ref != "#/components/parameters/TraceID" {
		t.Fatalf("parameter reference = %#v", operation.Parameters[0])
	}
	if operation.Responses["503"].Ref != "#/components/responses/Problem" {
		t.Fatalf("response reference = %#v", operation.Responses["503"])
	}
}
