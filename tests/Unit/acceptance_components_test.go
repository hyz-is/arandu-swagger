package unit_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/arandu-io/hesape/jsonschema"
	"github.com/arandu-io/hesape/routing"
	swagger "github.com/hyz-is/arandu-swagger"
)

func TestAcceptanceEquivalentSchemasAndComponentsRegisterIdempotently(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	firstObject := jsonschema.Object(
		jsonschema.Prop("name", jsonschema.String().Required()),
		jsonschema.Prop("age", jsonschema.Integer().Required()),
	)
	secondObject := jsonschema.Object(
		jsonschema.Prop("age", jsonschema.Integer().Required()),
		jsonschema.Prop("name", jsonschema.String().Required()),
	)
	if err := registry.Schema("Person", firstObject); err != nil {
		t.Fatalf("first Schema() error = %v", err)
	}
	afterFirstSchema := registry.Revision()
	if err := registry.Schema("Person", secondObject); err != nil {
		t.Fatalf("semantically equivalent Schema() error = %v", err)
	}
	if got := registry.Revision(); got != afterFirstSchema {
		t.Fatalf("idempotent schema changed revision from %d to %d", afterFirstSchema, got)
	}
	if err := registry.Schema("Role", jsonschema.String().Enum("admin", "member")); err != nil {
		t.Fatalf("first enum Schema() error = %v", err)
	}
	afterFirstEnum := registry.Revision()
	if err := registry.Schema("Role", jsonschema.String().Enum("member", "admin")); err != nil {
		t.Fatalf("semantically equivalent enum Schema() error = %v", err)
	}
	if got := registry.Revision(); got != afterFirstEnum {
		t.Fatalf("idempotent enum schema changed revision from %d to %d", afterFirstEnum, got)
	}

	stringSchema := acceptanceSchema(t, jsonschema.String())
	registrations := []struct {
		name     string
		register func() error
	}{
		{
			name: "parameter",
			register: func() error {
				return registry.Parameter("TraceID", swagger.Parameter{
					Name: "X-Trace-ID", In: swagger.ParameterInHeader, Schema: &stringSchema,
				})
			},
		},
		{
			name: "response",
			register: func() error {
				return registry.Response("Accepted", swagger.Response{
					Description: "Accepted",
					Headers: map[string]*swagger.Header{
						"X-Trace-ID": {Description: "Trace", Schema: &stringSchema},
					},
				})
			},
		},
		{
			name: "request body",
			register: func() error {
				return registry.RequestBody("Payload", swagger.RequestBody{
					Required: true,
					Content: swagger.Content{
						"application/json": {Schema: &stringSchema},
					},
				})
			},
		},
		{
			name: "header",
			register: func() error {
				return registry.Header("RequestID", swagger.Header{
					Description: "Request identifier", Schema: &stringSchema,
				})
			},
		},
		{
			name: "example",
			register: func() error {
				return registry.ExampleComponent("Greeting", swagger.Example{
					Summary: "Greeting", Value: json.RawMessage(`{ "message": "hello" }`),
				})
			},
		},
		{
			name: "security scheme",
			register: func() error {
				return registry.SecurityScheme("basicAuth", swagger.HTTPBasic())
			},
		},
	}

	for _, registration := range registrations {
		if err := registration.register(); err != nil {
			t.Fatalf("first %s registration error = %v", registration.name, err)
		}
		revision := registry.Revision()
		if err := registration.register(); err != nil {
			t.Fatalf("idempotent %s registration error = %v", registration.name, err)
		}
		if got := registry.Revision(); got != revision {
			t.Errorf("idempotent %s changed revision from %d to %d", registration.name, revision, got)
		}
	}
}

func TestAcceptanceReusableBodiesHeadersAndExamplesRemainReferenceable(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	if err := registry.Schema("Payload", jsonschema.Object(
		jsonschema.Prop("message", jsonschema.String().Required()),
	)); err != nil {
		t.Fatalf("register Payload schema: %v", err)
	}
	integerSchema := acceptanceSchema(t, jsonschema.Integer().Min(0))
	if err := registry.Header("RateLimit", swagger.Header{
		Description: "Remaining requests", Schema: &integerSchema,
	}); err != nil {
		t.Fatalf("register RateLimit header: %v", err)
	}
	if err := registry.ExampleComponent("PayloadExample", swagger.Example{
		Summary: "Greeting", Value: json.RawMessage(`{"message":"hello"}`),
	}); err != nil {
		t.Fatalf("register PayloadExample: %v", err)
	}
	payloadMedia := &swagger.MediaType{
		Schema: acceptanceSchemaPointer(swagger.SchemaRef("Payload")),
		Examples: map[string]*swagger.Example{
			"sample": {Ref: "#/components/examples/PayloadExample"},
		},
	}
	if err := registry.RequestBody("PayloadBody", swagger.RequestBody{
		Description: "Greeting payload",
		Required:    true,
		Content:     swagger.Content{"application/json": payloadMedia},
	}); err != nil {
		t.Fatalf("register PayloadBody: %v", err)
	}
	if err := registry.Response("Accepted", swagger.Response{
		Description: "Accepted",
		Headers: map[string]*swagger.Header{
			"X-RateLimit-Remaining": {Ref: "#/components/headers/RateLimit"},
		},
		Content: swagger.Content{"application/json": payloadMedia},
	}); err != nil {
		t.Fatalf("register Accepted response: %v", err)
	}

	router := routing.NewRouter()
	route := router.Post(
		"/messages",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).Name("messages.store")
	registry.Route(route).
		RequestBodyRef("PayloadBody").
		ResponseRef(http.StatusAccepted, "Accepted")

	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	components := document.Components
	if components == nil {
		t.Fatal("components are absent")
	}
	if components.RequestBodies["PayloadBody"] == nil || components.Headers["RateLimit"] == nil || components.Examples["PayloadExample"] == nil {
		t.Fatalf("reusable components = %#v", components)
	}
	response := document.Paths["/messages"].Post.Responses["202"]
	if got := document.Paths["/messages"].Post.RequestBody.Ref; got != "#/components/requestBodies/PayloadBody" {
		t.Errorf("operation request body reference = %q", got)
	}
	if got := response.Ref; got != "#/components/responses/Accepted" {
		t.Errorf("operation response reference = %q", got)
	}
	accepted := components.Responses["Accepted"]
	if got := accepted.Headers["X-RateLimit-Remaining"].Ref; got != "#/components/headers/RateLimit" {
		t.Errorf("header reference = %q", got)
	}
	if got := accepted.Content["application/json"].Examples["sample"].Ref; got != "#/components/examples/PayloadExample" {
		t.Errorf("example reference = %q", got)
	}
}

func TestAcceptanceEveryLocalComponentReferenceMustResolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		register func(*swagger.Registry) error
		missing  string
	}{
		{
			name: "parameter",
			register: func(registry *swagger.Registry) error {
				return registry.Parameter("Alias", swagger.Parameter{Ref: "#/components/parameters/MissingParameter"})
			},
			missing: "MissingParameter",
		},
		{
			name: "response",
			register: func(registry *swagger.Registry) error {
				return registry.Response("Alias", swagger.Response{Ref: "#/components/responses/MissingResponse"})
			},
			missing: "MissingResponse",
		},
		{
			name: "request body",
			register: func(registry *swagger.Registry) error {
				return registry.RequestBody("Alias", swagger.RequestBody{Ref: "#/components/requestBodies/MissingBody"})
			},
			missing: "MissingBody",
		},
		{
			name: "header",
			register: func(registry *swagger.Registry) error {
				return registry.Header("Alias", swagger.Header{Ref: "#/components/headers/MissingHeader"})
			},
			missing: "MissingHeader",
		},
		{
			name: "example",
			register: func(registry *swagger.Registry) error {
				return registry.ExampleComponent("Alias", swagger.Example{Ref: "#/components/examples/MissingExample"})
			},
			missing: "MissingExample",
		},
		{
			name: "security scheme",
			register: func(registry *swagger.Registry) error {
				return registry.SecurityScheme("Alias", swagger.SecurityScheme{Ref: "#/components/securitySchemes/MissingSecurity"})
			},
			missing: "MissingSecurity",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			registry := swagger.NewRegistry()
			if err := test.register(registry); err != nil {
				t.Fatalf("register reference: %v", err)
			}
			_, err := swagger.Generate(nil, registry, generationConfig())
			if err == nil || !strings.Contains(err.Error(), test.missing) {
				t.Fatalf("Generate() error = %v, want missing local reference %q", err, test.missing)
			}
		})
	}
}

func TestAcceptanceExternalComponentReferencesRemainResolvableByTheConsumer(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	if err := registry.Response("RemoteProblem", swagger.Response{
		Ref: "https://schemas.example.test/openapi.json#/components/responses/Problem",
	}); err != nil {
		t.Fatalf("register external reference: %v", err)
	}
	if _, err := swagger.Generate(nil, registry, generationConfig()); err != nil {
		t.Fatalf("Generate() external reference error = %v", err)
	}
}

func TestAcceptanceNestedExampleReferencesMustResolve(t *testing.T) {
	t.Parallel()

	schema := acceptanceSchema(t, jsonschema.String())
	tests := []struct {
		name     string
		register func(*swagger.Registry) error
	}{
		{
			name: "parameter example",
			register: func(registry *swagger.Registry) error {
				return registry.Parameter("Query", swagger.Parameter{
					Name: "q", In: swagger.ParameterInQuery, Schema: &schema,
					Examples: map[string]*swagger.Example{
						"missing": {Ref: "#/components/examples/MissingExample"},
					},
				})
			},
		},
		{
			name: "header example",
			register: func(registry *swagger.Registry) error {
				return registry.Header("Trace", swagger.Header{
					Schema: &schema,
					Examples: map[string]*swagger.Example{
						"missing": {Ref: "#/components/examples/MissingExample"},
					},
				})
			},
		},
		{
			name: "media example",
			register: func(registry *swagger.Registry) error {
				return registry.RequestBody("Payload", swagger.RequestBody{
					Content: swagger.Content{
						"application/json": {
							Schema: &schema,
							Examples: map[string]*swagger.Example{
								"missing": {Ref: "#/components/examples/MissingExample"},
							},
						},
					},
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			registry := swagger.NewRegistry()
			if err := test.register(registry); err != nil {
				t.Fatalf("register component: %v", err)
			}
			_, err := swagger.Generate(nil, registry, generationConfig())
			if err == nil || !strings.Contains(err.Error(), "MissingExample") {
				t.Fatalf("Generate() error = %v, want missing nested example reference", err)
			}
		})
	}
}

func TestAcceptanceMutuallyExclusiveOpenAPIFieldsAreRejected(t *testing.T) {
	t.Parallel()

	schema := acceptanceSchema(t, jsonschema.String())
	content := swagger.Content{
		"application/json": {Schema: &schema},
	}
	tests := []struct {
		name     string
		register func(*swagger.Registry) error
		want     []string
	}{
		{
			name: "parameter schema and content",
			register: func(registry *swagger.Registry) error {
				return registry.Parameter("AmbiguousParameter", swagger.Parameter{
					Name: "q", In: swagger.ParameterInQuery, Schema: &schema, Content: content,
				})
			},
			want: []string{"AmbiguousParameter", "schema", "content"},
		},
		{
			name: "header schema and content",
			register: func(registry *swagger.Registry) error {
				return registry.Header("AmbiguousHeader", swagger.Header{Schema: &schema, Content: content})
			},
			want: []string{"AmbiguousHeader", "schema", "content"},
		},
		{
			name: "parameter reference and concrete fields",
			register: func(registry *swagger.Registry) error {
				if err := registry.Parameter("TargetParameter", swagger.Parameter{
					Name: "q", In: swagger.ParameterInQuery, Schema: &schema,
				}); err != nil {
					return err
				}
				return registry.Parameter("ReferencedParameter", swagger.Parameter{
					Ref: "#/components/parameters/TargetParameter", Name: "ignored", In: swagger.ParameterInQuery,
				})
			},
			want: []string{"ReferencedParameter", "$ref"},
		},
		{
			name: "header reference and concrete schema",
			register: func(registry *swagger.Registry) error {
				if err := registry.Header("TargetHeader", swagger.Header{Schema: &schema}); err != nil {
					return err
				}
				return registry.Header("ReferencedHeader", swagger.Header{
					Ref: "#/components/headers/TargetHeader", Schema: &schema,
				})
			},
			want: []string{"ReferencedHeader", "$ref"},
		},
		{
			name: "response reference and content",
			register: func(registry *swagger.Registry) error {
				if err := registry.Response("TargetResponse", swagger.Response{Description: "Target"}); err != nil {
					return err
				}
				return registry.Response("AmbiguousResponse", swagger.Response{
					Ref: "#/components/responses/TargetResponse", Content: content,
				})
			},
			want: []string{"AmbiguousResponse", "$ref"},
		},
		{
			name: "request body reference and content",
			register: func(registry *swagger.Registry) error {
				if err := registry.RequestBody("TargetBody", swagger.RequestBody{Content: content}); err != nil {
					return err
				}
				return registry.RequestBody("AmbiguousBody", swagger.RequestBody{
					Ref: "#/components/requestBodies/TargetBody", Content: content, Required: true,
				})
			},
			want: []string{"AmbiguousBody", "$ref"},
		},
		{
			name: "example reference and value",
			register: func(registry *swagger.Registry) error {
				if err := registry.ExampleComponent("TargetExample", swagger.Example{Value: json.RawMessage(`"target"`)}); err != nil {
					return err
				}
				return registry.ExampleComponent("AmbiguousExample", swagger.Example{
					Ref: "#/components/examples/TargetExample", Value: json.RawMessage(`"ignored"`),
				})
			},
			want: []string{"AmbiguousExample", "$ref"},
		},
		{
			name: "security reference and concrete type",
			register: func(registry *swagger.Registry) error {
				if err := registry.SecurityScheme("TargetSecurity", swagger.HTTPBasic()); err != nil {
					return err
				}
				return registry.SecurityScheme("AmbiguousSecurity", swagger.SecurityScheme{
					Ref: "#/components/securitySchemes/TargetSecurity", Type: swagger.SecuritySchemeHTTP, Scheme: "basic",
				})
			},
			want: []string{"AmbiguousSecurity", "$ref"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			registry := swagger.NewRegistry()
			if err := test.register(registry); err != nil {
				t.Fatalf("register component: %v", err)
			}
			_, err := swagger.Generate(nil, registry, generationConfig())
			if err == nil {
				t.Fatal("Generate() error = nil, want mutually exclusive field diagnostic")
			}
			for _, fragment := range test.want {
				if !strings.Contains(err.Error(), fragment) {
					t.Errorf("Generate() error = %q, want %q", err, fragment)
				}
			}
		})
	}
}

func acceptanceSchema(t *testing.T, native jsonschema.Type) swagger.Schema {
	t.Helper()

	schema, err := swagger.SchemaFrom(native)
	if err != nil {
		t.Fatalf("SchemaFrom() error = %v", err)
	}
	return schema
}

func acceptanceSchemaPointer(schema swagger.Schema) *swagger.Schema {
	return &schema
}
