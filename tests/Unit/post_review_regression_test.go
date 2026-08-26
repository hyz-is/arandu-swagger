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

func TestSchemaFromRejectsInvalidNativeSchemaKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema jsonschema.Type
		want   string
	}{
		{name: "negative minLength", schema: jsonschema.String().Min(-1), want: "minLength"},
		{name: "negative minItems", schema: jsonschema.Array().Min(-1), want: "minItems"},
		{name: "zero multipleOf", schema: jsonschema.Integer().MultipleOf(0), want: "multipleOf"},
		{name: "negative multipleOf", schema: jsonschema.Number().MultipleOf(-0.5), want: "multipleOf"},
		{name: "duplicate enum values", schema: jsonschema.String().Enum("active", "active"), want: "unique"},
		{name: "non-portable Go regular expression", schema: jsonschema.String().Pattern(`(?i)yes|no`), want: "ECMAScript"},
		{
			name: "duplicate nested JSON keys",
			schema: jsonschema.Object(
				jsonschema.Prop("name", jsonschema.String()),
				jsonschema.Prop("name", jsonschema.Integer()),
			),
			want: "duplicate key",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := swagger.SchemaFrom(test.schema)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("SchemaFrom() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestSchemaFromValidatesOnlyActualSubschemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema jsonschema.Type
	}{
		{
			name: "property named like a schema keyword",
			schema: jsonschema.Object(
				jsonschema.Prop("type", jsonschema.String()),
			),
		},
		{
			name: "default object containing schema-like keys",
			schema: jsonschema.Object().Default(map[string]any{
				"type":     "invoice",
				"required": []string{"customer", "total"},
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := swagger.SchemaFrom(test.schema); err != nil {
				t.Fatalf("SchemaFrom() error = %v, want valid schema", err)
			}
		})
	}
}

func TestSchemaComponentEqualityPreservesArrayOrderInsideDefaultValues(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	first := jsonschema.Object().Default(map[string]any{
		"required": []string{"first", "second"},
	})
	second := jsonschema.Object().Default(map[string]any{
		"required": []string{"second", "first"},
	})
	if err := registry.Schema("OrderedDefault", first); err != nil {
		t.Fatalf("first Schema() error = %v", err)
	}
	if err := registry.Schema("OrderedDefault", second); err == nil || !strings.Contains(err.Error(), "different content") {
		t.Fatalf("second Schema() error = %v, want ordered default conflict", err)
	}
}

func TestComponentRegistrationSnapshotsExplodePointers(t *testing.T) {
	t.Parallel()

	stringSchema, err := swagger.SchemaFrom(jsonschema.String())
	if err != nil {
		t.Fatalf("SchemaFrom(string) error = %v", err)
	}
	objectSchema, err := swagger.SchemaFrom(jsonschema.Object(
		jsonschema.Prop("field", jsonschema.String()),
	))
	if err != nil {
		t.Fatalf("SchemaFrom(object) error = %v", err)
	}

	parameterExplode := true
	headerExplode := true
	encodingExplode := true
	registry := swagger.NewRegistry()
	if err := registry.Parameter("Query", swagger.Parameter{
		Name: "q", In: swagger.ParameterInQuery, Schema: &stringSchema, Explode: &parameterExplode,
	}); err != nil {
		t.Fatalf("Parameter() error = %v", err)
	}
	if err := registry.Header("Metadata", swagger.Header{
		Schema: &stringSchema, Explode: &headerExplode,
	}); err != nil {
		t.Fatalf("Header() error = %v", err)
	}
	if err := registry.RequestBody("Form", swagger.RequestBody{
		Content: swagger.Content{
			"application/x-www-form-urlencoded": {
				Schema: &objectSchema,
				Encoding: map[string]*swagger.Encoding{
					"field": {Explode: &encodingExplode},
				},
			},
		},
	}); err != nil {
		t.Fatalf("RequestBody() error = %v", err)
	}

	parameterExplode = false
	headerExplode = false
	encodingExplode = false
	document, err := swagger.Generate(nil, registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got := document.Components.Parameters["Query"].Explode; got == nil || !*got {
		t.Fatalf("parameter explode snapshot = %v, want true", got)
	}
	if got := document.Components.Headers["Metadata"].Explode; got == nil || !*got {
		t.Fatalf("header explode snapshot = %v, want true", got)
	}
	gotEncoding := document.Components.RequestBodies["Form"].Content["application/x-www-form-urlencoded"].Encoding["field"]
	if gotEncoding == nil || gotEncoding.Explode == nil || !*gotEncoding.Explode {
		t.Fatalf("encoding explode snapshot = %#v, want true", gotEncoding)
	}
}

func TestGenerationRejectsInvalidAndEquivalentMediaTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		media []*swagger.Media
		want  string
	}{
		{
			name:  "invalid media type",
			media: []*swagger.Media{swagger.MediaOf("application", jsonschema.String())},
			want:  "invalid",
		},
		{
			name: "equivalent media types",
			media: []*swagger.Media{
				swagger.MediaOf("application/json; charset=utf-8", jsonschema.String()),
				swagger.MediaOf("Application/JSON; Charset=utf-8", jsonschema.String()),
			},
			want: "equivalent content types",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			router := routing.NewRouter()
			route := router.Get("/representation", emptyUnitHandler).Name("representation.show")
			registry := swagger.NewRegistry()
			registry.Route(route).Response(http.StatusOK, "Representation", test.media...)

			_, err := swagger.Generate(router.Routes(), registry, generationConfig())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Generate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestGenerationRejectsMalformedReferenceURIs(t *testing.T) {
	t.Parallel()

	t.Run("schema $ref", func(t *testing.T) {
		t.Parallel()

		invalid := swagger.SchemaReference("http://[::1")
		if _, err := json.Marshal(invalid); err == nil || !strings.Contains(err.Error(), "invalid URI reference") {
			t.Fatalf("json.Marshal() error = %v, want malformed URI-reference diagnostic", err)
		}
		registry := swagger.NewRegistry()
		if err := registry.Parameter("BrokenReference", swagger.Parameter{
			Name: "q", In: swagger.ParameterInQuery, Schema: &invalid,
		}); err != nil {
			t.Fatalf("Parameter() error = %v", err)
		}

		_, err := swagger.Generate(nil, registry, generationConfig())
		if err == nil || !strings.Contains(err.Error(), "$ref") || !strings.Contains(err.Error(), "invalid URI reference") {
			t.Fatalf("Generate() error = %v, want malformed $ref URI diagnostic", err)
		}
	})

	t.Run("example externalValue", func(t *testing.T) {
		t.Parallel()

		registry := swagger.NewRegistry()
		if err := registry.ExampleComponent("BrokenExternalValue", swagger.Example{
			ExternalValue: "http://[::1",
		}); err != nil {
			t.Fatalf("ExampleComponent() error = %v", err)
		}

		_, err := swagger.Generate(nil, registry, generationConfig())
		if err == nil || !strings.Contains(err.Error(), "externalValue") || !strings.Contains(err.Error(), "invalid URI reference") {
			t.Fatalf("Generate() error = %v, want malformed externalValue URI diagnostic", err)
		}
	})
}

func TestLocalSchemaReferencesMayTargetNestedSchemas(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	if err := registry.Schema("Envelope", jsonschema.Object(
		jsonschema.Prop("payload", jsonschema.Object(
			jsonschema.Prop("id", jsonschema.String()),
		)),
	)); err != nil {
		t.Fatalf("Schema() error = %v", err)
	}
	nested := swagger.SchemaReference("#/components/schemas/Envelope/properties/payload")
	if err := registry.Parameter("Payload", swagger.Parameter{
		Name: "payload", In: swagger.ParameterInQuery, Schema: &nested,
	}); err != nil {
		t.Fatalf("Parameter() error = %v", err)
	}
	if _, err := swagger.Generate(nil, registry, generationConfig()); err != nil {
		t.Fatalf("Generate() error = %v, want valid nested schema reference", err)
	}
}

func TestLocalSchemaReferencesMustResolveTheirNestedTarget(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	if err := registry.Schema("Envelope", jsonschema.Object(
		jsonschema.Prop("payload", jsonschema.String()),
	)); err != nil {
		t.Fatalf("Schema() error = %v", err)
	}
	missing := swagger.SchemaReference("#/components/schemas/Envelope/properties/missing")
	if err := registry.Parameter("Missing", swagger.Parameter{
		Name: "missing", In: swagger.ParameterInQuery, Schema: &missing,
	}); err != nil {
		t.Fatalf("Parameter() error = %v", err)
	}
	_, err := swagger.Generate(nil, registry, generationConfig())
	if err == nil || !strings.Contains(err.Error(), "missing schema target") {
		t.Fatalf("Generate() error = %v, want missing nested schema target", err)
	}
}

func TestLocalSchemaReferencesRejectNonCanonicalArrayIndexes(t *testing.T) {
	t.Parallel()

	for _, index := range []string{"+0", "-0", "01"} {
		index := index
		t.Run(index, func(t *testing.T) {
			t.Parallel()
			registry := swagger.NewRegistry()
			if err := registry.Schema("Choice", jsonschema.AnyOf(jsonschema.String(), jsonschema.Integer())); err != nil {
				t.Fatalf("Schema() error = %v", err)
			}
			invalid := swagger.SchemaReference("#/components/schemas/Choice/anyOf/" + index)
			if err := registry.Parameter("Choice", swagger.Parameter{
				Name: "choice", In: swagger.ParameterInQuery, Schema: &invalid,
			}); err != nil {
				t.Fatalf("Parameter() error = %v", err)
			}
			_, err := swagger.Generate(nil, registry, generationConfig())
			if err == nil || !strings.Contains(err.Error(), "missing schema target") {
				t.Fatalf("Generate() error = %v, want non-canonical array-index diagnostic", err)
			}
		})
	}
}

func TestGenerationValidatesRequestBodyEncodings(t *testing.T) {
	t.Parallel()

	schema, err := swagger.SchemaFrom(jsonschema.Object(
		jsonschema.Prop("field", jsonschema.String()),
	))
	if err != nil {
		t.Fatalf("SchemaFrom() error = %v", err)
	}
	stringSchema, err := swagger.SchemaFrom(jsonschema.String())
	if err != nil {
		t.Fatalf("SchemaFrom(string) error = %v", err)
	}
	tests := []struct {
		name        string
		contentType string
		encoding    *swagger.Encoding
		want        string
	}{
		{
			name:        "nil encoding",
			contentType: "application/x-www-form-urlencoded",
			want:        "is nil",
		},
		{
			name:        "encoding on JSON",
			contentType: "application/json",
			encoding:    &swagger.Encoding{},
			want:        "form or multipart",
		},
		{
			name:        "invalid property content type",
			contentType: "multipart/form-data",
			encoding:    &swagger.Encoding{ContentType: "not-a-media-type"},
			want:        "contentType",
		},
		{
			name:        "unsupported query style",
			contentType: "multipart/form-data",
			encoding:    &swagger.Encoding{Style: swagger.ParameterStyleMatrix},
			want:        "style",
		},
		{
			name:        "nil encoding header",
			contentType: "multipart/form-data",
			encoding: &swagger.Encoding{Headers: map[string]*swagger.Header{
				"X-Part": nil,
			}},
			want: "is nil",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			registry := swagger.NewRegistry()
			if err := registry.RequestBody("Payload", swagger.RequestBody{
				Content: swagger.Content{
					test.contentType: {
						Schema: &schema,
						Encoding: map[string]*swagger.Encoding{
							"field": test.encoding,
						},
					},
				},
			}); err != nil {
				t.Fatalf("RequestBody() error = %v", err)
			}
			_, err := swagger.Generate(nil, registry, generationConfig())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Generate() error = %v, want %q", err, test.want)
			}
		})
	}

	registry := swagger.NewRegistry()
	if err := registry.RequestBody("ValidForm", swagger.RequestBody{
		Content: swagger.Content{
			"application/x-www-form-urlencoded": {
				Schema: &schema,
				Encoding: map[string]*swagger.Encoding{
					"field": {
						ContentType: "text/plain, application/json",
						Style:       swagger.ParameterStyleForm,
						Headers: map[string]*swagger.Header{
							"X-Part": {Schema: &stringSchema},
						},
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("RequestBody(valid) error = %v", err)
	}
	if _, err := swagger.Generate(nil, registry, generationConfig()); err != nil {
		t.Fatalf("Generate(valid encoding) error = %v", err)
	}
}

func TestRequestBodyEncodingPropertiesMustExistInTheSchema(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	if err := registry.Schema("Form", jsonschema.Object(
		jsonschema.Prop("field", jsonschema.String()),
	)); err != nil {
		t.Fatalf("Schema() error = %v", err)
	}
	form := swagger.SchemaRef("Form")
	if err := registry.RequestBody("InvalidForm", swagger.RequestBody{
		Content: swagger.Content{
			"multipart/mixed": {
				Schema: &form,
				Encoding: map[string]*swagger.Encoding{
					"ghost": {},
				},
			},
		},
	}); err != nil {
		t.Fatalf("RequestBody() error = %v", err)
	}
	_, err := swagger.Generate(nil, registry, generationConfig())
	if err == nil || !strings.Contains(err.Error(), "schema property") || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("Generate() error = %v, want missing encoding property diagnostic", err)
	}
}

func TestRequestBodyEncodingAcceptsMultipartSubtypesAndLocalSchemaReferences(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	if err := registry.Schema("Form", jsonschema.Object(
		jsonschema.Prop("field", jsonschema.String()),
	)); err != nil {
		t.Fatalf("Schema() error = %v", err)
	}
	form := swagger.SchemaRef("Form")
	if err := registry.RequestBody("ValidMultipart", swagger.RequestBody{
		Content: swagger.Content{
			"multipart/mixed": {
				Schema: &form,
				Encoding: map[string]*swagger.Encoding{
					"field": {},
				},
			},
		},
	}); err != nil {
		t.Fatalf("RequestBody() error = %v", err)
	}
	if _, err := swagger.Generate(nil, registry, generationConfig()); err != nil {
		t.Fatalf("Generate() error = %v, want valid multipart subtype and local schema reference", err)
	}
}

func TestExternalSchemaEncodingPropertyMembershipRemainsCallerOwned(t *testing.T) {
	t.Parallel()

	external := swagger.SchemaReference("https://schemas.example.test/form.json")
	registry := swagger.NewRegistry()
	if err := registry.RequestBody("ExternalForm", swagger.RequestBody{
		Content: swagger.Content{
			"multipart/form-data": {
				Schema: &external,
				Encoding: map[string]*swagger.Encoding{
					"remoteProperty": {},
				},
			},
		},
	}); err != nil {
		t.Fatalf("RequestBody() error = %v", err)
	}
	if _, err := swagger.Generate(nil, registry, generationConfig()); err != nil {
		t.Fatalf("Generate() error = %v, want external property membership left to the caller", err)
	}
}

func TestOperationIDForMustNameAMethodDeclaredByTheRoute(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	route := router.Get("/health", emptyUnitHandler).Name("health.show")
	registry := swagger.NewRegistry()
	registry.Route(route).
		OperationIDFor(http.MethodPost, "health.create").
		Response(http.StatusOK, "Healthy")

	_, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err == nil || !strings.Contains(err.Error(), "OperationIDFor") || !strings.Contains(err.Error(), http.MethodPost) {
		t.Fatalf("Generate() error = %v, want undeclared OperationIDFor method diagnostic", err)
	}
}

func TestAnyCompatibilityMarkerUsesExplicitMethodsForIdentifiers(t *testing.T) {
	t.Parallel()

	for _, explicitIDs := range []bool{false, true} {
		explicitIDs := explicitIDs
		t.Run(map[bool]string{false: "automatic", true: "explicit"}[explicitIDs], func(t *testing.T) {
			t.Parallel()

			router := routing.NewRouter()
			route := router.Any("/hooks", emptyUnitHandler).Name("hooks")
			route.Method = ""
			registry := swagger.NewRegistry()
			builder := registry.Route(route).
				Methods(http.MethodGet, http.MethodPost).
				Response(http.StatusOK, "Accepted")
			if explicitIDs {
				builder.OperationIDFor(http.MethodGet, "hooks.read").
					OperationIDFor(http.MethodPost, "hooks.create")
			}

			document, err := swagger.Generate(router.Routes(), registry, generationConfig())
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			wantGet, wantPost := "hooks.get", "hooks.post"
			if explicitIDs {
				wantGet, wantPost = "hooks.read", "hooks.create"
			}
			item := document.Paths["/hooks"]
			if item == nil || item.Get == nil || item.Post == nil {
				t.Fatalf("ANY path item = %#v, want GET and POST", item)
			}
			if item.Get.OperationID != wantGet || item.Post.OperationID != wantPost {
				t.Fatalf("operation IDs = %q, %q, want %q, %q", item.Get.OperationID, item.Post.OperationID, wantGet, wantPost)
			}
		})
	}
}

func TestServeMuxSubtreeRoutesRequireAnExactPath(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/prefix/", "/"} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			router := routing.NewRouter()
			route := router.Get(path, emptyUnitHandler).Name("subtree.show")
			registry := swagger.NewRegistry()
			registry.Route(route).Response(http.StatusOK, "Subtree")

			_, err := swagger.Generate(router.Routes(), registry, generationConfig())
			if err == nil || !strings.Contains(err.Error(), "subtree route") || !strings.Contains(err.Error(), path+"{$}") {
				t.Fatalf("Generate() error = %v, want exact-path correction %q", err, path+"{$}")
			}
		})
	}
}

func TestUnnormalizedQualifiedBindingRoutesAreRejected(t *testing.T) {
	t.Parallel()

	route := routing.NewRouter().NewRoute(
		[]string{http.MethodGet},
		"/posts/{post:slug}",
		emptyUnitHandler,
	).Name("posts.show")
	registry := swagger.NewRegistry()
	registry.Route(route).Response(http.StatusOK, "Post")

	_, err := swagger.Generate([]*routing.Route{route}, registry, generationConfig())
	if err == nil || !strings.Contains(err.Error(), "qualified binding") || !strings.Contains(err.Error(), "{post:slug}") {
		t.Fatalf("Generate() error = %v, want unnormalized qualified-binding diagnostic", err)
	}
}

func TestDocumentationRedirectIsExcludedWithoutModuleMetadata(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	redirect := router.Get(swagger.DefaultUIPath+"/{$}", emptyUnitHandler).Name("documentation.redirect")
	registry := swagger.NewRegistry()
	registry.Route(redirect).Response(http.StatusPermanentRedirect, "Canonical documentation URL")

	document, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(document.Paths) != 0 {
		t.Fatalf("generated paths = %#v, want the package-owned redirect excluded", document.Paths)
	}
}

var emptyUnitHandler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
