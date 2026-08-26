package unit_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/arandu-io/hesape/jsonschema"
	"github.com/arandu-io/swagger"
)

func TestTheOpenAPIModelRendersACompleteTypedDocument(t *testing.T) {
	t.Parallel()

	identifier, err := swagger.SchemaFrom(jsonschema.String().Format("uuid"))
	if err != nil {
		t.Fatalf("snapshot identifier schema: %v", err)
	}
	user, err := swagger.SchemaFrom(jsonschema.Object(
		jsonschema.Prop("id", jsonschema.String().Format("uuid").Required()),
	))
	if err != nil {
		t.Fatalf("snapshot user schema: %v", err)
	}

	explode := false
	document := swagger.Document{
		OpenAPI:           "3.1.0",
		JSONSchemaDialect: "https://json-schema.org/draft/2020-12/schema",
		Info: swagger.Info{
			Title:          "Example API",
			Summary:        "Typed OpenAPI model",
			Description:    "An API used to exercise the public model.",
			TermsOfService: "https://example.test/terms",
			Contact: &swagger.Contact{
				Name:  "API team",
				URL:   "https://example.test/support",
				Email: "api@example.test",
			},
			License: &swagger.License{
				Name:       "MIT",
				Identifier: "MIT",
			},
			Version: "1.0.0",
		},
		Servers: []swagger.Server{{
			URL:         "https://{tenant}.example.test",
			Description: "Production",
			Variables: map[string]swagger.ServerVariable{
				"tenant": {Default: "demo", Description: "Tenant slug", Enum: []string{"demo", "acme"}},
			},
		}},
		Tags: []swagger.Tag{{
			Name:        "Users",
			Description: "User operations",
			ExternalDocs: &swagger.ExternalDocumentation{
				Description: "User guide",
				URL:         "https://example.test/docs/users",
			},
		}},
		ExternalDocs: &swagger.ExternalDocumentation{URL: "https://example.test/docs"},
		Paths: swagger.Paths{
			"/users/{id}": {
				Summary: "Users by identifier",
				Get: &swagger.Operation{
					Tags:        []string{"Users"},
					Summary:     "Show a user",
					Description: "Returns a user.",
					OperationID: "users.show",
					Parameters: []*swagger.Parameter{{
						Name:        "id",
						In:          swagger.ParameterInPath,
						Description: "User identifier",
						Required:    true,
						Schema:      &identifier,
					}, {
						Name:    "include",
						In:      swagger.ParameterInQuery,
						Explode: &explode,
						Content: swagger.Content{
							"application/json": {Schema: &identifier},
						},
					}},
					RequestBody: &swagger.RequestBody{
						Description: "Update input",
						Required:    true,
						Content: swagger.Content{
							"application/json": {
								Schema:  &user,
								Example: json.RawMessage(`{"id":"01900000-0000-7000-8000-000000000000"}`),
								Encoding: map[string]*swagger.Encoding{
									"id": {ContentType: "text/plain"},
								},
							},
						},
					},
					Responses: swagger.Responses{
						"200": {
							Description: "User found",
							Headers: map[string]*swagger.Header{
								"X-Request-ID": {Description: "Request identifier", Schema: &identifier},
							},
							Content: swagger.Content{
								"application/json": {
									Schema: &user,
									Examples: map[string]*swagger.Example{
										"user": {Summary: "A user", Value: json.RawMessage(`{"id":"01900000-0000-7000-8000-000000000000"}`)},
									},
								},
							},
						},
					},
					Security: []swagger.SecurityRequirement{{"bearerAuth": {}}},
				},
				Parameters: []*swagger.Parameter{{Ref: "#/components/parameters/TraceID"}},
			},
		},
		Components: &swagger.Components{
			Schemas: map[string]swagger.Schema{"User": user},
			Parameters: map[string]*swagger.Parameter{
				"TraceID": {Name: "trace", In: swagger.ParameterInHeader, Schema: &identifier},
			},
			Responses: map[string]*swagger.Response{
				"Problem": {Description: "A problem response"},
			},
			RequestBodies: map[string]*swagger.RequestBody{
				"User": {Content: swagger.Content{"application/json": {Schema: &user}}},
			},
			Headers: map[string]*swagger.Header{
				"TraceID": {Schema: &identifier},
			},
			Examples: map[string]*swagger.Example{
				"User": {ExternalValue: "https://example.test/examples/user.json"},
			},
			SecuritySchemes: map[string]*swagger.SecurityScheme{
				"bearerAuth": ptr(swagger.HTTPBearer("JWT")),
			},
		},
		Security: []swagger.SecurityRequirement{{"bearerAuth": {}}},
		Extensions: swagger.Extensions{
			"x-arandu-module": json.RawMessage(`"users"`),
		},
	}

	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal document: %v", err)
	}
	for _, fragment := range []string{
		`"openapi":"3.1.0"`,
		`"jsonSchemaDialect":"https://json-schema.org/draft/2020-12/schema"`,
		`"operationId":"users.show"`,
		`"required":true`,
		`"format":"uuid"`,
		`"securitySchemes":{"bearerAuth":{"type":"http","scheme":"bearer","bearerFormat":"JWT"}}`,
		`"x-arandu-module":"users"`,
	} {
		if !strings.Contains(string(encoded), fragment) {
			t.Errorf("document does not contain %s:\n%s", fragment, encoded)
		}
	}

	again, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal document again: %v", err)
	}
	if string(again) != string(encoded) {
		t.Fatalf("encoding is not deterministic:\nfirst:  %s\nsecond: %s", encoded, again)
	}
}

func TestSchemaFromTakesAnImmutableSnapshot(t *testing.T) {
	t.Parallel()

	native := jsonschema.String().Description("before")
	schema, err := swagger.SchemaFrom(native)
	if err != nil {
		t.Fatalf("snapshot schema: %v", err)
	}
	native.Description("after")

	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	if got, want := string(encoded), `{"type":"string","description":"before"}`; got != want {
		t.Fatalf("schema snapshot = %s, want %s", got, want)
	}

	copyOfJSON := schema.JSON()
	copyOfJSON[0] = '['
	encoded, err = json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal schema after copy mutation: %v", err)
	}
	if got, want := string(encoded), `{"type":"string","description":"before"}`; got != want {
		t.Fatalf("schema snapshot changed through JSON copy: got %s, want %s", got, want)
	}
}

func TestSchemaFromTurnsInvalidNativeSchemasIntoErrors(t *testing.T) {
	t.Parallel()

	if _, err := swagger.SchemaFrom(nil); err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("SchemaFrom(nil) error = %v, want a nil schema error", err)
	}

	broken := jsonschema.Object(jsonschema.Prop("broken", nil))
	if _, err := swagger.SchemaFrom(broken); err == nil || !strings.Contains(err.Error(), "panic") {
		t.Fatalf("SchemaFrom(broken) error = %v, want a recovered panic error", err)
	}

	var zero swagger.Schema
	if _, err := json.Marshal(zero); err == nil || !strings.Contains(err.Error(), "empty schema") {
		t.Fatalf("marshal zero schema error = %v, want an empty schema error", err)
	}
}

func TestSchemaReferencesUseOpenAPIComponentPaths(t *testing.T) {
	t.Parallel()

	schema := swagger.SchemaRef("User")
	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal reference: %v", err)
	}
	if got, want := string(encoded), `{"$ref":"#/components/schemas/User"}`; got != want {
		t.Fatalf("reference = %s, want %s", got, want)
	}
	if got, want := schema.Reference(), "#/components/schemas/User"; got != want {
		t.Fatalf("Reference() = %q, want %q", got, want)
	}
}

func TestExtensionsRejectNonExtensionKeysAndRenderInStableOrder(t *testing.T) {
	t.Parallel()

	valid := swagger.Info{
		Title:   "Example",
		Version: "1.0.0",
		Extensions: swagger.Extensions{
			"x-z": json.RawMessage(`2`),
			"x-a": json.RawMessage(`1`),
		},
	}
	encoded, err := json.Marshal(valid)
	if err != nil {
		t.Fatalf("marshal valid extensions: %v", err)
	}
	if got, want := string(encoded), `{"title":"Example","version":"1.0.0","x-a":1,"x-z":2}`; got != want {
		t.Fatalf("extended info = %s, want %s", got, want)
	}

	invalid := swagger.Info{
		Title:      "Example",
		Version:    "1.0.0",
		Extensions: swagger.Extensions{"not-an-extension": json.RawMessage(`true`)},
	}
	if _, err := json.Marshal(invalid); err == nil || !strings.Contains(err.Error(), "x-") {
		t.Fatalf("invalid extension error = %v, want an x- key error", err)
	}
}

func TestSecuritySchemeHelpersRenderEverySupportedKind(t *testing.T) {
	t.Parallel()

	schemes := map[string]swagger.SecurityScheme{
		"basic":  swagger.HTTPBasic(),
		"bearer": swagger.HTTPBearer("JWT"),
		"apiKey": swagger.APIKey("X-API-Key", swagger.APIKeyInHeader),
		"oauth": swagger.OAuth2(swagger.OAuthFlows{
			AuthorizationCode: &swagger.OAuthFlow{
				AuthorizationURL: "https://identity.example.test/authorize",
				TokenURL:         "https://identity.example.test/token",
				Scopes:           map[string]string{"users:read": "Read users"},
			},
			ClientCredentials: &swagger.OAuthFlow{
				TokenURL: "https://identity.example.test/token",
			},
		}),
		"oidc": swagger.OpenIDConnect("https://identity.example.test/.well-known/openid-configuration"),
	}

	encoded, err := json.Marshal(schemes)
	if err != nil {
		t.Fatalf("marshal security schemes: %v", err)
	}
	for _, fragment := range []string{
		`"basic":{"type":"http","scheme":"basic"}`,
		`"bearer":{"type":"http","scheme":"bearer","bearerFormat":"JWT"}`,
		`"apiKey":{"type":"apiKey","name":"X-API-Key","in":"header"}`,
		`"oauth":{"type":"oauth2","flows":{"clientCredentials":`,
		`"authorizationCode":{"authorizationUrl":"https://identity.example.test/authorize"`,
		`"clientCredentials":{"tokenUrl":"https://identity.example.test/token","scopes":{}}`,
		`"oidc":{"type":"openIdConnect","openIdConnectUrl":"https://identity.example.test/.well-known/openid-configuration"}`,
	} {
		if !strings.Contains(string(encoded), fragment) {
			t.Errorf("security schemes do not contain %s:\n%s", fragment, encoded)
		}
	}
}

func TestSecurityRequirementHelperEmitsAnEmptyScopeArray(t *testing.T) {
	t.Parallel()

	requirement := swagger.SecurityRequirementFor("bearerAuth")
	encoded, err := json.Marshal(requirement)
	if err != nil {
		t.Fatalf("marshal security requirement: %v", err)
	}
	if got, want := string(encoded), `{"bearerAuth":[]}`; got != want {
		t.Fatalf("security requirement = %s, want %s", got, want)
	}

	encoded, err = json.Marshal(swagger.SecurityRequirement{"bearerAuth": nil})
	if err != nil {
		t.Fatalf("marshal nil security scopes: %v", err)
	}
	if got, want := string(encoded), `{"bearerAuth":[]}`; got != want {
		t.Fatalf("nil security scopes = %s, want %s", got, want)
	}
}

func ptr[T any](value T) *T {
	return &value
}
