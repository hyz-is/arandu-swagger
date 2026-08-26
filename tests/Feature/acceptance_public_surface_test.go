package feature_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/jsonschema"
	"github.com/arandu-io/swagger"
)

func TestAcceptanceModuleForwardsEveryReusableComponentRegistration(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.DisableUI = true
	config.DisableSpec = true
	module := newModule(t, config)
	payloadSchema := jsonschema.Object(
		jsonschema.Prop("message", jsonschema.String().Required()),
	)
	if err := module.Schema("Payload", payloadSchema); err != nil {
		t.Fatalf("Module.Schema() error = %v", err)
	}
	stringSchema, err := swagger.SchemaFrom(jsonschema.String())
	if err != nil {
		t.Fatalf("SchemaFrom() error = %v", err)
	}
	if err := module.Parameter("TraceID", swagger.Parameter{
		Name: "X-Trace-ID", In: swagger.ParameterInHeader, Schema: &stringSchema,
	}); err != nil {
		t.Fatalf("Module.Parameter() error = %v", err)
	}
	if err := module.Header("RequestID", swagger.Header{
		Description: "Request identifier", Schema: &stringSchema,
	}); err != nil {
		t.Fatalf("Module.Header() error = %v", err)
	}
	if err := module.ExampleComponent("PayloadExample", swagger.Example{
		Summary: "Greeting", Value: json.RawMessage(`{"message":"hello"}`),
	}); err != nil {
		t.Fatalf("Module.ExampleComponent() error = %v", err)
	}
	if err := module.RequestBody("PayloadBody", swagger.RequestBody{
		Required: true,
		Content: swagger.Content{
			"application/json": {
				Schema: acceptanceFeatureSchemaPointer(swagger.SchemaRef("Payload")),
				Examples: map[string]*swagger.Example{
					"sample": {Ref: "#/components/examples/PayloadExample"},
				},
			},
		},
	}); err != nil {
		t.Fatalf("Module.RequestBody() error = %v", err)
	}
	if err := module.Response("Accepted", swagger.Response{
		Description: "Accepted",
		Headers: map[string]*swagger.Header{
			"X-Request-ID": {Ref: "#/components/headers/RequestID"},
		},
	}); err != nil {
		t.Fatalf("Module.Response() error = %v", err)
	}
	if err := module.SecurityScheme("basicAuth", swagger.HTTPBasic()); err != nil {
		t.Fatalf("Module.SecurityScheme() error = %v", err)
	}

	router := fhttp.NewRouter()
	route := router.ForModule("messages").Post("/messages", emptyHandler).Name("messages.store")
	module.Route(route).
		ParameterRef("TraceID").
		RequestBodyRef("PayloadBody").
		ResponseRef(http.StatusAccepted, "Accepted").
		Security("basicAuth")
	module.Routes(router.ForModule(module.Name()))

	document, err := module.Generate()
	if err != nil {
		t.Fatalf("Module.Generate() error = %v", err)
	}
	components := document.Components
	if components == nil {
		t.Fatal("Module component forwarding produced no Components Object")
	}
	if len(components.Schemas) != 1 || len(components.Parameters) != 1 ||
		len(components.Responses) != 1 || len(components.RequestBodies) != 1 ||
		len(components.Headers) != 1 || len(components.Examples) != 1 ||
		len(components.SecuritySchemes) != 1 {
		t.Fatalf("forwarded components = %#v", components)
	}
	operation := document.Paths["/messages"].Post
	if operation == nil {
		t.Fatal("documented operation is absent")
	}
	if operation.RequestBody == nil || operation.RequestBody.Ref != "#/components/requestBodies/PayloadBody" {
		t.Fatalf("operation request body = %#v, want reusable PayloadBody reference", operation.RequestBody)
	}
	encoded, err := json.Marshal(operation.Security)
	if err != nil {
		t.Fatalf("marshal operation security: %v", err)
	}
	if got := string(encoded); got != `[{"basicAuth":[]}]` {
		t.Fatalf("operation security = %s, want non-OAuth scopes as []", got)
	}
}

func TestAcceptanceCustomUIAndSpecificationPathsOwnOnlyTheirNamespaces(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.UIPath = "/reference"
	config.SpecPath = "/openapi/v1.json"
	router, _ := mount(t, config)

	for _, target := range []string{swagger.DefaultUIPath, swagger.DefaultSpecPath} {
		if response := request(router, target); response.Code != http.StatusNotFound {
			t.Errorf("default path %s answered %d, want 404", target, response.Code)
		}
	}
	if response := request(router, "/reference"); response.Code != http.StatusOK {
		t.Fatalf("custom UI answered %d: %s", response.Code, response.Body.String())
	}
	specification := request(router, "/openapi/v1.json")
	if specification.Code != http.StatusOK {
		t.Fatalf("custom specification answered %d: %s", specification.Code, specification.Body.String())
	}
	var document swagger.Document
	if err := json.Unmarshal(specification.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode custom specification: %v", err)
	}
	for _, ownPath := range []string{"/reference", "/reference/swagger-initializer.js", "/openapi/v1.json"} {
		if _, exists := document.Paths[ownPath]; exists {
			t.Errorf("custom specification published its own endpoint %s", ownPath)
		}
	}
	initializer := request(router, "/reference/swagger-initializer.js")
	if initializer.Code != http.StatusOK || !strings.Contains(initializer.Body.String(), `url: "/openapi/v1.json"`) {
		t.Fatalf("custom initializer answered %d with %s", initializer.Code, initializer.Body.String())
	}
	asset := request(router, "/reference/assets/5.32.14/swagger-ui.css")
	if asset.Code != http.StatusOK || asset.Header().Get("Content-Type") != "text/css; charset=utf-8" {
		t.Fatalf("custom asset answered %d with Content-Type %q", asset.Code, asset.Header().Get("Content-Type"))
	}
	if response := request(router, "/docs/assets/5.32.14/swagger-ui.css"); response.Code != http.StatusNotFound {
		t.Fatalf("default asset namespace answered %d, want 404", response.Code)
	}
}

func acceptanceFeatureSchemaPointer(schema swagger.Schema) *swagger.Schema {
	return &schema
}
