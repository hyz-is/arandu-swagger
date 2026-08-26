package unit_test

import (
	"net/http"
	"strings"
	"testing"

	swagger "github.com/hyz-is/arandu-swagger"
)

func TestDisabledDocumentationAcceptsTheZeroConfiguration(t *testing.T) {
	t.Parallel()

	if err := (swagger.Config{}).Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestDisabledDocumentationIgnoresDormantEndpointSettings(t *testing.T) {
	t.Parallel()

	config := swagger.Config{
		UIPath:         "not-an-endpoint",
		DisableSpec:    true,
		UIMiddleware:   []swagger.Middleware{nil},
		SpecMiddleware: []swagger.Middleware{nil},
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestEnabledDocumentationRequiresItsIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  swagger.Config
		missing string
	}{
		{
			name: "title",
			config: swagger.Config{
				Enabled: true,
				Version: "1.0.0",
			},
			missing: "Config.Title",
		},
		{
			name: "version",
			config: swagger.Config{
				Enabled: true,
				Title:   "Example API",
			},
			missing: "Config.Version",
		},
		{
			name: "whitespace-only title",
			config: swagger.Config{
				Enabled: true,
				Title:   "\t",
				Version: "1.0.0",
			},
			missing: "Config.Title",
		},
		{
			name: "whitespace-only version",
			config: swagger.Config{
				Enabled: true,
				Title:   "Example API",
				Version: "\n",
			},
			missing: "Config.Version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.config.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.missing) {
				t.Fatalf("Validate() error = %v, want an error naming %s", err, tt.missing)
			}
		})
	}
}

func TestEnabledDocumentationAcceptsTheDefaultEndpointPaths(t *testing.T) {
	t.Parallel()

	config := swagger.Config{
		Enabled: true,
		Title:   "Example API",
		Version: "1.0.0",
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if swagger.DefaultUIPath != "/docs" {
		t.Fatalf("DefaultUIPath = %q, want /docs", swagger.DefaultUIPath)
	}
	if swagger.DefaultSpecPath != "/docs/openapi.json" {
		t.Fatalf("DefaultSpecPath = %q, want /docs/openapi.json", swagger.DefaultSpecPath)
	}
}

func TestEnabledDocumentationRejectsInvalidStaticMetadataDuringConstruction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*swagger.Config)
		field  string
	}{
		{
			name: "relative terms of service URL",
			mutate: func(config *swagger.Config) {
				config.TermsOfService = "/terms"
			},
			field: "Config.TermsOfService",
		},
		{
			name: "relative contact URL",
			mutate: func(config *swagger.Config) {
				config.Contact = &swagger.Contact{URL: "/contact"}
			},
			field: "Config.Contact.URL",
		},
		{
			name: "invalid contact email",
			mutate: func(config *swagger.Config) {
				config.Contact = &swagger.Contact{Email: "not an email"}
			},
			field: "Config.Contact.Email",
		},
		{
			name: "license without a name",
			mutate: func(config *swagger.Config) {
				config.License = &swagger.License{Identifier: "MIT"}
			},
			field: "Config.License.Name",
		},
		{
			name: "license with identifier and URL",
			mutate: func(config *swagger.Config) {
				config.License = &swagger.License{
					Name:       "MIT",
					Identifier: "MIT",
					URL:        "https://example.test/license",
				}
			},
			field: "Config.License",
		},
		{
			name: "server with an undeclared variable",
			mutate: func(config *swagger.Config) {
				config.Servers = []swagger.Server{{URL: "https://{tenant}.example.test"}}
			},
			field: "Config.Servers[0]",
		},
		{
			name: "blank tag name",
			mutate: func(config *swagger.Config) {
				config.Tags = []swagger.Tag{{Name: " "}}
			},
			field: "Config.Tags[0].Name",
		},
		{
			name: "duplicate tag name",
			mutate: func(config *swagger.Config) {
				config.Tags = []swagger.Tag{{Name: "Users"}, {Name: "Users"}}
			},
			field: "duplicate name",
		},
		{
			name: "invalid tag external documentation URL",
			mutate: func(config *swagger.Config) {
				config.Tags = []swagger.Tag{{
					Name:         "Users",
					ExternalDocs: &swagger.ExternalDocumentation{URL: "/users"},
				}}
			},
			field: "Config.Tags[0].ExternalDocs.URL",
		},
		{
			name: "invalid external documentation URL",
			mutate: func(config *swagger.Config) {
				config.ExternalDocs = &swagger.ExternalDocumentation{URL: "/reference"}
			},
			field: "Config.ExternalDocs.URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := validConfig()
			tt.mutate(&config)

			validateErr := config.Validate()
			if validateErr == nil || !strings.Contains(validateErr.Error(), tt.field) {
				t.Fatalf("Validate() error = %v, want an error naming %s", validateErr, tt.field)
			}

			module, newErr := swagger.New(config)
			if newErr == nil || !strings.Contains(newErr.Error(), tt.field) {
				t.Fatalf("New() error = %v, want an error naming %s", newErr, tt.field)
			}
			if module != nil {
				t.Fatalf("New() module = %#v, want nil", module)
			}
		})
	}
}

func TestDisabledDocumentationIgnoresDormantStaticMetadata(t *testing.T) {
	t.Parallel()

	config := swagger.Config{
		TermsOfService: "/terms",
		Contact:        &swagger.Contact{Email: "not an email"},
		License:        &swagger.License{},
		Servers:        []swagger.Server{{URL: "https://{tenant}.example.test"}},
		Tags:           []swagger.Tag{{Name: " "}},
		ExternalDocs:   &swagger.ExternalDocumentation{URL: "/reference"},
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if _, err := swagger.New(config); err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
}

func TestEnabledDocumentationAcceptsValidStaticMetadataDuringConstruction(t *testing.T) {
	t.Parallel()

	config := validConfig()
	config.TermsOfService = "https://example.test/terms"
	config.Contact = &swagger.Contact{
		Name:  "API Team",
		URL:   "https://example.test/contact",
		Email: "api@example.test",
	}
	config.License = &swagger.License{Name: "MIT", Identifier: "MIT"}
	config.Servers = []swagger.Server{{
		URL: "https://{tenant}.example.test",
		Variables: map[string]swagger.ServerVariable{
			"tenant": {Default: "demo", Enum: []string{"demo", "production"}},
		},
	}}
	config.Tags = []swagger.Tag{{
		Name:         "Users",
		ExternalDocs: &swagger.ExternalDocumentation{URL: "https://example.test/users"},
	}}
	config.ExternalDocs = &swagger.ExternalDocumentation{URL: "https://example.test/reference"}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	module, err := swagger.New(config)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if module == nil {
		t.Fatal("New() module = nil, want a module")
	}
}

func TestEndpointPathsMustBeStaticAbsoluteAndClean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*swagger.Config)
		field  string
	}{
		{
			name: "relative UI path",
			mutate: func(config *swagger.Config) {
				config.UIPath = "docs"
			},
			field: "Config.UIPath",
		},
		{
			name: "unclean UI path",
			mutate: func(config *swagger.Config) {
				config.UIPath = "/reference/../docs"
			},
			field: "Config.UIPath",
		},
		{
			name: "UI route parameter",
			mutate: func(config *swagger.Config) {
				config.UIPath = "/docs/{tenant}"
			},
			field: "Config.UIPath",
		},
		{
			name: "spec query",
			mutate: func(config *swagger.Config) {
				config.SpecPath = "/openapi.json?format=json"
			},
			field: "Config.SpecPath",
		},
		{
			name: "spec fragment",
			mutate: func(config *swagger.Config) {
				config.SpecPath = "/openapi.json#public"
			},
			field: "Config.SpecPath",
		},
		{
			name: "spec wildcard",
			mutate: func(config *swagger.Config) {
				config.SpecPath = "/{document...}"
			},
			field: "Config.SpecPath",
		},
		{
			name: "malformed percent escape",
			mutate: func(config *swagger.Config) {
				config.SpecPath = "/docs/openapi%zz.json"
			},
			field: "Config.SpecPath",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := validConfig()
			tt.mutate(&config)

			err := config.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("Validate() error = %v, want an error naming %s", err, tt.field)
			}
		})
	}
}

func TestCustomUIPathDerivesAnAvailableSpecPath(t *testing.T) {
	t.Parallel()

	config := validConfig()
	config.UIPath = "/reference"

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestPublicUIRequiresTheSpecificationEndpoint(t *testing.T) {
	t.Parallel()

	config := validConfig()
	config.DisableSpec = true

	err := config.Validate()
	if err == nil || !strings.Contains(err.Error(), "DisableSpec") || !strings.Contains(err.Error(), "DisableUI") {
		t.Fatalf("Validate() error = %v, want an error explaining the UI/spec dependency", err)
	}
}

func TestProgrammaticOnlyGenerationCanDisableBothEndpoints(t *testing.T) {
	t.Parallel()

	config := validConfig()
	config.DisableUI = true
	config.DisableSpec = true

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestEnabledEndpointsCannotUseTheSamePath(t *testing.T) {
	t.Parallel()

	config := validConfig()
	config.UIPath = "/reference"
	config.SpecPath = "/reference"

	err := config.Validate()
	if err == nil || !strings.Contains(err.Error(), "same path") {
		t.Fatalf("Validate() error = %v, want a path conflict error", err)
	}
}

func TestSpecificationCannotShadowTheUIAssetNamespace(t *testing.T) {
	t.Parallel()

	config := validConfig()
	config.SpecPath = "/docs/assets/openapi.json"

	err := config.Validate()
	if err == nil || !strings.Contains(err.Error(), "asset") {
		t.Fatalf("Validate() error = %v, want an asset path conflict error", err)
	}
}

func TestSpecificationCannotShadowTheUIInitializer(t *testing.T) {
	t.Parallel()

	config := validConfig()
	config.SpecPath = "/docs/swagger-initializer.js"

	err := config.Validate()
	if err == nil || !strings.Contains(err.Error(), "initializer") {
		t.Fatalf("Validate() error = %v, want an initializer path conflict error", err)
	}
}

func TestMiddlewareMustBeCallable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*swagger.Config)
		field  string
	}{
		{
			name: "UI middleware",
			mutate: func(config *swagger.Config) {
				config.UIMiddleware = []swagger.Middleware{nil}
			},
			field: "Config.UIMiddleware[0]",
		},
		{
			name: "spec middleware",
			mutate: func(config *swagger.Config) {
				config.SpecMiddleware = []swagger.Middleware{nil}
			},
			field: "Config.SpecMiddleware[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := validConfig()
			tt.mutate(&config)

			err := config.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("Validate() error = %v, want an error naming %s", err, tt.field)
			}
		})
	}
}

func TestNetHTTPMiddlewareIsAccepted(t *testing.T) {
	t.Parallel()

	config := validConfig()
	config.UIMiddleware = []swagger.Middleware{
		func(next http.Handler) http.Handler { return next },
	}
	config.SpecMiddleware = []swagger.Middleware{
		func(next http.Handler) http.Handler { return next },
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestRouteFiltersRejectAmbiguousValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		filter swagger.RouteFilter
		field  string
	}{
		{
			name: "relative included prefix",
			filter: swagger.RouteFilter{
				IncludePrefixes: []string{"api"},
			},
			field: "IncludePrefixes[0]",
		},
		{
			name: "blank route name",
			filter: swagger.RouteFilter{
				ExcludeNames: []string{" "},
			},
			field: "ExcludeNames[0]",
		},
		{
			name: "blank method",
			filter: swagger.RouteFilter{
				IncludeMethods: []string{""},
			},
			field: "IncludeMethods[0]",
		},
		{
			name: "same method included and excluded",
			filter: swagger.RouteFilter{
				IncludeMethods: []string{"get"},
				ExcludeMethods: []string{"GET"},
			},
			field: "IncludeMethods",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := validConfig()
			config.Filter = tt.filter

			err := config.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("Validate() error = %v, want an error naming %s", err, tt.field)
			}
		})
	}
}

func validConfig() swagger.Config {
	return swagger.Config{
		Enabled: true,
		Title:   "Example API",
		Version: "1.0.0",
	}
}
