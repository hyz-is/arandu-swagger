package unit_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/jsonschema"
	swagger "github.com/hyz-is/arandu-swagger"
)

func TestRouteDocumentationBuildsBodiesResponsesParametersAndSecurity(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter().ForModule("users")
	route := router.Post("/api/users/{id}", func(http.ResponseWriter, *http.Request) {}).
		Name("users.update").
		WhereUuid("id")
	registry := swagger.NewRegistry()

	if err := registry.Schema("User", jsonschema.Object(
		jsonschema.Prop("id", jsonschema.String().Format("uuid").Required()),
	)); err != nil {
		t.Fatalf("register User schema: %v", err)
	}
	if err := registry.Schema("Problem", jsonschema.Object(
		jsonschema.Prop("message", jsonschema.String().Required()),
	)); err != nil {
		t.Fatalf("register Problem schema: %v", err)
	}
	if err := registry.SecurityScheme("bearerAuth", swagger.HTTPBearer("JWT")); err != nil {
		t.Fatalf("register security scheme: %v", err)
	}

	registry.Route(route).
		Summary("Update a user").
		Description("Updates one user in the authenticated tenant.").
		Tags("Users").
		PathParameter("id", swagger.Description("User identifier."), swagger.ExampleValue("01900000-0000-7000-8000-000000000000")).
		QueryParameter("include", jsonschema.String().Enum("profile", "permissions"), swagger.Description("Relations to include.")).
		HeaderParameter("X-Trace-ID", jsonschema.String()).
		CookieParameter("locale", jsonschema.String()).
		RequestBody(swagger.JSONRef("User").Required().Example(map[string]any{"id": "01900000-0000-7000-8000-000000000000"})).
		Response(http.StatusOK, "User updated", swagger.JSONRef("User")).
		Response(http.StatusUnprocessableEntity, "Validation failed", swagger.JSONRef("Problem")).
		ResponseHeader(http.StatusOK, "X-Request-ID", swagger.Header{Description: "Request identifier"}).
		Security("bearerAuth")

	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	operation := document.Paths["/api/users/{id}"].Post
	if operation == nil {
		t.Fatal("POST /api/users/{id} was not generated")
	}
	if got, want := operation.OperationID, "users.update"; got != want {
		t.Errorf("OperationID = %q, want %q", got, want)
	}
	if got, want := operation.Summary, "Update a user"; got != want {
		t.Errorf("Summary = %q, want %q", got, want)
	}
	if operation.RequestBody == nil || !operation.RequestBody.Required {
		t.Fatal("required request body was not generated")
	}
	if got := operation.RequestBody.Content["application/json"].Schema.Reference(); got != "#/components/schemas/User" {
		t.Errorf("request schema reference = %q", got)
	}
	if got := operation.Responses["200"].Headers["X-Request-ID"].Description; got != "Request identifier" {
		t.Errorf("response header description = %q", got)
	}
	if len(operation.Security) != 1 || operation.Security[0]["bearerAuth"] == nil {
		t.Errorf("security = %#v, want bearerAuth", operation.Security)
	}

	locations := map[swagger.ParameterLocation]bool{}
	for _, parameter := range operation.Parameters {
		locations[parameter.In] = true
		if parameter.In == swagger.ParameterInPath && (!parameter.Required || parameter.Name != "id") {
			t.Errorf("path parameter = %#v, want required id", parameter)
		}
	}
	for _, location := range []swagger.ParameterLocation{
		swagger.ParameterInPath,
		swagger.ParameterInQuery,
		swagger.ParameterInHeader,
		swagger.ParameterInCookie,
	} {
		if !locations[location] {
			t.Errorf("no %s parameter was generated", location)
		}
	}
}

func TestRegistryRejectsConflictingComponentsButAcceptsIdempotentRegistration(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	first := jsonschema.String().Description("user identifier")
	if err := registry.Schema("UserID", first); err != nil {
		t.Fatalf("first Schema() error = %v", err)
	}
	if err := registry.Schema("UserID", jsonschema.String().Description("user identifier")); err != nil {
		t.Fatalf("idempotent Schema() error = %v", err)
	}
	if err := registry.Schema("UserID", jsonschema.Integer()); err == nil || !strings.Contains(err.Error(), "UserID") {
		t.Fatalf("conflicting Schema() error = %v, want component name", err)
	}

	if err := registry.Response("Problem", swagger.Response{Description: "Problem"}); err != nil {
		t.Fatalf("first Response() error = %v", err)
	}
	if err := registry.Response("Problem", swagger.Response{Description: "Different"}); err == nil || !strings.Contains(err.Error(), "Problem") {
		t.Fatalf("conflicting Response() error = %v, want component name", err)
	}
}

func TestEveryRegistryMutationInvalidatesItsRevision(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	route := router.Get("/health", func(http.ResponseWriter, *http.Request) {})
	registry := swagger.NewRegistry()
	initial := registry.Revision()
	builder := registry.Route(route)
	afterRoute := registry.Revision()
	builder.Summary("Health")
	afterSummary := registry.Revision()
	builder.Response(http.StatusNoContent, "Healthy")
	afterResponse := registry.Revision()

	if !(initial < afterRoute && afterRoute < afterSummary && afterSummary < afterResponse) {
		t.Fatalf("revisions did not advance: %d, %d, %d, %d", initial, afterRoute, afterSummary, afterResponse)
	}
}

func TestRegistriesAreIsolatedByInstance(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	route := router.Get("/health", func(http.ResponseWriter, *http.Request) {}).Name("health")
	first := swagger.NewRegistry()
	second := swagger.NewRegistry()
	first.Route(route).Response(http.StatusNoContent, "Healthy")

	firstDocument, err := swagger.Generate(router.Routes(), first, generationConfig())
	if err != nil {
		t.Fatalf("generate first registry: %v", err)
	}
	secondDocument, err := swagger.Generate(router.Routes(), second, generationConfig())
	if err != nil {
		t.Fatalf("generate second registry: %v", err)
	}
	if len(firstDocument.Paths) != 1 {
		t.Fatalf("first registry paths = %d, want 1", len(firstDocument.Paths))
	}
	if len(secondDocument.Paths) != 0 {
		t.Fatalf("second registry paths = %d, want 0", len(secondDocument.Paths))
	}
}

func TestBuilderErrorsNameTheOperationAndPointOfCorrection(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	route := router.Post("/users/{id}", func(http.ResponseWriter, *http.Request) {}).Name("users.update")
	registry := swagger.NewRegistry()
	registry.Route(route).
		PathParameter("missing").
		QueryParameter("q", nil).
		RequestBody(swagger.JSON(nil)).
		Response(http.StatusOK, "", swagger.JSON(nil)).
		Response(http.StatusCreated, "").
		Extension("not-x", true)

	_, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err == nil {
		t.Fatal("Generate() error = nil, want accumulated builder errors")
	}
	for _, fragment := range []string{"users.update", "missing", "schema", "description", "x-"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("error %q does not contain %q", err, fragment)
		}
	}
}

func TestMediaExamplesAreSnapshotted(t *testing.T) {
	t.Parallel()

	example := map[string]any{"name": "before"}
	media := swagger.JSON(jsonschema.Object(jsonschema.Prop("name", jsonschema.String()))).Example(example)
	example["name"] = "after"

	router := fhttp.NewRouter()
	route := router.Get("/example", func(http.ResponseWriter, *http.Request) {}).Name("example")
	registry := swagger.NewRegistry()
	registry.Route(route).Response(http.StatusOK, "Example", media)
	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	raw := document.Paths["/example"].Get.Responses["200"].Content["application/json"].Example
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode example: %v", err)
	}
	if got["name"] != "before" {
		t.Fatalf("example name = %v, want before", got["name"])
	}
}

func generationConfig() swagger.Config {
	return swagger.Config{Enabled: true, Title: "Example API", Version: "1.0.0"}
}
