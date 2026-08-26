package unit_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/arandu-io/hesape/routing"
	"github.com/arandu-io/swagger"
)

func TestAcceptanceSupportedSecuritySchemesPassGenerationValidation(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	schemes := map[string]swagger.SecurityScheme{
		"basicAuth":     swagger.HTTPBasic(),
		"headerKey":     swagger.APIKey("X-API-Key", swagger.APIKeyInHeader),
		"queryKey":      swagger.APIKey("api_key", swagger.APIKeyInQuery),
		"cookieKey":     swagger.APIKey("api_key", swagger.APIKeyInCookie),
		"openidConnect": swagger.OpenIDConnect("https://identity.example.test/.well-known/openid-configuration"),
		"oauth": swagger.OAuth2(swagger.OAuthFlows{
			Implicit: &swagger.OAuthFlow{
				AuthorizationURL: "https://identity.example.test/authorize",
				Scopes:           map[string]string{},
			},
			Password: &swagger.OAuthFlow{
				TokenURL: "https://identity.example.test/token",
				Scopes:   map[string]string{},
			},
			ClientCredentials: &swagger.OAuthFlow{
				TokenURL: "https://identity.example.test/token",
				Scopes:   map[string]string{"users:read": "Read users"},
			},
			AuthorizationCode: &swagger.OAuthFlow{
				AuthorizationURL: "https://identity.example.test/authorize",
				TokenURL:         "https://identity.example.test/token",
				RefreshURL:       "https://identity.example.test/refresh",
				Scopes:           map[string]string{"users:write": "Write users"},
			},
		}),
	}
	for name, scheme := range schemes {
		if err := registry.SecurityScheme(name, scheme); err != nil {
			t.Fatalf("SecurityScheme(%q) error = %v", name, err)
		}
	}
	if _, err := swagger.Generate(nil, registry, generationConfig()); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
}

func TestAcceptanceInvalidSecuritySchemesIdentifyTheirCorrectionPoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		scheme swagger.SecurityScheme
		want   string
	}{
		{name: "API key without name", scheme: swagger.APIKey(" ", swagger.APIKeyInHeader), want: "needs a name"},
		{name: "API key with invalid location", scheme: swagger.APIKey("key", swagger.APIKeyLocation("body")), want: "invalid location"},
		{name: "HTTP without scheme", scheme: swagger.SecurityScheme{Type: swagger.SecuritySchemeHTTP}, want: "HTTP scheme"},
		{name: "OAuth without flows", scheme: swagger.SecurityScheme{Type: swagger.SecuritySchemeOAuth2}, want: "needs flows"},
		{name: "OAuth without a concrete flow", scheme: swagger.OAuth2(swagger.OAuthFlows{}), want: "at least one flow"},
		{
			name: "implicit OAuth without authorization URL",
			scheme: swagger.OAuth2(swagger.OAuthFlows{Implicit: &swagger.OAuthFlow{
				Scopes: map[string]string{},
			}}),
			want: "authorizationUrl",
		},
		{
			name: "password OAuth without token URL",
			scheme: swagger.OAuth2(swagger.OAuthFlows{Password: &swagger.OAuthFlow{
				Scopes: map[string]string{},
			}}),
			want: "tokenUrl",
		},
		{
			name: "OAuth without a scopes object",
			scheme: swagger.OAuth2(swagger.OAuthFlows{ClientCredentials: &swagger.OAuthFlow{
				TokenURL: "https://identity.example.test/token",
			}}),
			want: "scopes",
		},
		{
			name: "OAuth with invalid refresh URL",
			scheme: swagger.OAuth2(swagger.OAuthFlows{ClientCredentials: &swagger.OAuthFlow{
				TokenURL:   "https://identity.example.test/token",
				RefreshURL: "/refresh",
				Scopes:     map[string]string{},
			}}),
			want: "refreshUrl",
		},
		{name: "OpenID Connect with relative URL", scheme: swagger.OpenIDConnect("/.well-known/openid-configuration"), want: "openIdConnectUrl"},
		{name: "OpenID Connect with unsupported scheme", scheme: swagger.OpenIDConnect("ftp://identity.example.test/openid"), want: "http or https"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			registry := swagger.NewRegistry()
			if err := registry.SecurityScheme("auth", test.scheme); err != nil {
				t.Fatalf("SecurityScheme() error = %v", err)
			}
			_, err := swagger.Generate(nil, registry, generationConfig())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Generate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAcceptanceNonOAuthSecurityRequirementsMayDeclareRoles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		scheme swagger.SecurityScheme
	}{
		{name: "Basic", scheme: swagger.HTTPBasic()},
		{name: "API key", scheme: swagger.APIKey("X-API-Key", swagger.APIKeyInHeader)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			router := routing.NewRouter()
			route := router.Get(
				"/private",
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
			).Name("private.index")
			registry := swagger.NewRegistry()
			if err := registry.SecurityScheme("auth", test.scheme); err != nil {
				t.Fatalf("SecurityScheme() error = %v", err)
			}
			registry.Route(route).
				Response(http.StatusOK, "Private resource").
				Security("auth", "administrator")

			document, err := swagger.Generate(router.Routes(), registry, generationConfig())
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			encoded, err := json.Marshal(document.Paths["/private"].Get.Security)
			if err != nil {
				t.Fatalf("marshal Security Requirement: %v", err)
			}
			if got, want := string(encoded), `[{"auth":["administrator"]}]`; got != want {
				t.Fatalf("Security Requirement = %s, want %s", got, want)
			}
		})
	}
}

func TestAcceptanceOAuthRequirementsUseDeclaredScopes(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	route := router.Get(
		"/users",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).Name("users.index")
	registry := swagger.NewRegistry()
	if err := registry.SecurityScheme("oauth", swagger.OAuth2(swagger.OAuthFlows{
		ClientCredentials: &swagger.OAuthFlow{
			TokenURL: "https://identity.example.test/token",
			Scopes:   map[string]string{"users:read": "Read users"},
		},
	})); err != nil {
		t.Fatalf("SecurityScheme() error = %v", err)
	}
	registry.Route(route).
		Response(http.StatusOK, "Users").
		Security("oauth", "users:delete")

	_, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err == nil || !strings.Contains(err.Error(), "users:delete") || !strings.Contains(err.Error(), "oauth") {
		t.Fatalf("Generate() error = %v, want undeclared OAuth scope diagnostic", err)
	}
}

func TestAcceptanceInvalidGlobalMetadataIdentifiesItsField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*swagger.Config)
		want   string
	}{
		{
			name: "relative terms URL",
			mutate: func(config *swagger.Config) {
				config.TermsOfService = "/terms"
			},
			want: "Config.TermsOfService",
		},
		{
			name: "relative contact URL",
			mutate: func(config *swagger.Config) {
				config.Contact = &swagger.Contact{URL: "/contact"}
			},
			want: "Config.Contact.URL",
		},
		{
			name: "invalid contact email",
			mutate: func(config *swagger.Config) {
				config.Contact = &swagger.Contact{Email: "invalid@"}
			},
			want: "Config.Contact.Email",
		},
		{
			name: "license without name",
			mutate: func(config *swagger.Config) {
				config.License = &swagger.License{Identifier: "MIT"}
			},
			want: "Config.License.Name",
		},
		{
			name: "license identifier and URL",
			mutate: func(config *swagger.Config) {
				config.License = &swagger.License{Name: "MIT", Identifier: "MIT", URL: "https://example.test/license"}
			},
			want: "both Identifier and URL",
		},
		{
			name: "invalid license URL",
			mutate: func(config *swagger.Config) {
				config.License = &swagger.License{Name: "Custom", URL: "file:///license"}
			},
			want: "Config.License.URL",
		},
		{
			name: "server without URL",
			mutate: func(config *swagger.Config) {
				config.Servers = []swagger.Server{{}}
			},
			want: "Config.Servers[0].URL",
		},
		{
			name: "server variable is undeclared",
			mutate: func(config *swagger.Config) {
				config.Servers = []swagger.Server{{URL: "https://{region}.example.test"}}
			},
			want: "without a declaration",
		},
		{
			name: "server variable default is empty",
			mutate: func(config *swagger.Config) {
				config.Servers = []swagger.Server{{
					URL: "https://{region}.example.test",
					Variables: map[string]swagger.ServerVariable{
						"region": {},
					},
				}}
			},
			want: "Default",
		},
		{
			name: "server variable default is outside enum",
			mutate: func(config *swagger.Config) {
				config.Servers = []swagger.Server{{
					URL: "https://{region}.example.test",
					Variables: map[string]swagger.ServerVariable{
						"region": {Default: "us", Enum: []string{"eu"}},
					},
				}}
			},
			want: "Enum",
		},
		{
			name: "unused server variable",
			mutate: func(config *swagger.Config) {
				config.Servers = []swagger.Server{{
					URL: "https://api.example.test",
					Variables: map[string]swagger.ServerVariable{
						"region": {Default: "us"},
					},
				}}
			},
			want: "URL does not use",
		},
		{
			name: "tag without name",
			mutate: func(config *swagger.Config) {
				config.Tags = []swagger.Tag{{}}
			},
			want: "Config.Tags[0].Name",
		},
		{
			name: "duplicate tag",
			mutate: func(config *swagger.Config) {
				config.Tags = []swagger.Tag{{Name: "Users"}, {Name: "Users"}}
			},
			want: "duplicate name",
		},
		{
			name: "invalid tag external documentation",
			mutate: func(config *swagger.Config) {
				config.Tags = []swagger.Tag{{Name: "Users", ExternalDocs: &swagger.ExternalDocumentation{URL: "/users"}}}
			},
			want: "Config.Tags[0].ExternalDocs.URL",
		},
		{
			name: "invalid global external documentation",
			mutate: func(config *swagger.Config) {
				config.ExternalDocs = &swagger.ExternalDocumentation{URL: "mailto:docs@example.test"}
			},
			want: "Config.ExternalDocs.URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config := generationConfig()
			test.mutate(&config)
			_, err := swagger.Generate(nil, swagger.NewRegistry(), config)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Generate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAcceptanceInvalidOperationMetadataNamesTheRoute(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	route := router.Get(
		"/users",
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).Name("users.index")
	registry := swagger.NewRegistry()
	registry.Route(route).
		ExternalDocumentation(swagger.ExternalDocumentation{URL: "/users"}).
		Response(http.StatusOK, "Users")

	_, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err == nil || !strings.Contains(err.Error(), "GET /users") || !strings.Contains(err.Error(), "externalDocs") {
		t.Fatalf("Generate() error = %v, want route-scoped externalDocs error", err)
	}
}
