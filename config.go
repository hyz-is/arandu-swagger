package swagger

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"unicode"

	frameworkhttp "github.com/arandu-io/framework/http"
)

const (
	// DefaultUIPath is the documentation UI endpoint used when UIPath is empty.
	DefaultUIPath = "/docs"
	// DefaultSpecPath is the specification endpoint used when both UIPath and
	// SpecPath are empty.
	DefaultSpecPath = "/docs/openapi.json"
)

// Middleware wraps one of the HTTP handlers exposed by the package.
type Middleware = func(http.Handler) http.Handler

// RoutePredicate decides whether a route passes a caller-defined filter.
type RoutePredicate func(*frameworkhttp.Route) bool

// RouteFilter limits which application routes may appear in the document.
//
// A non-empty include list acts as an allowlist for that dimension. Exclusions
// are applied after inclusions. Prefixes match route paths; all other list
// fields match their route metadata. IncludePredicate must accept a route and
// ExcludePredicate must not reject it.
type RouteFilter struct {
	// IncludePrefixes allows routes whose paths start with one of these values.
	IncludePrefixes []string
	// ExcludePrefixes rejects routes whose paths start with one of these values.
	ExcludePrefixes []string
	// IncludeNames allows routes with one of these route names.
	IncludeNames []string
	// ExcludeNames rejects routes with one of these route names.
	ExcludeNames []string
	// IncludeModules allows routes owned by one of these module names.
	IncludeModules []string
	// ExcludeModules rejects routes owned by one of these module names.
	ExcludeModules []string
	// IncludeMethods allows routes using one of these HTTP methods.
	IncludeMethods []string
	// ExcludeMethods rejects routes using one of these HTTP methods.
	ExcludeMethods []string
	// IncludeDomains allows routes scoped to one of these domains.
	IncludeDomains []string
	// ExcludeDomains rejects routes scoped to one of these domains.
	ExcludeDomains []string
	// IncludePredicate applies an additional caller-defined allow rule.
	IncludePredicate RoutePredicate
	// ExcludePredicate applies an additional caller-defined deny rule.
	ExcludePredicate RoutePredicate
}

// Config controls document generation and the optional HTTP endpoints.
//
// Documentation is opt-in: the zero value is disabled and valid. When Enabled
// is true, Title and Version identify the generated OpenAPI document. An empty
// UIPath defaults to DefaultUIPath. An empty SpecPath is placed below the
// effective UIPath as openapi.json.
type Config struct {
	// Enabled registers the configured documentation endpoints when true.
	Enabled bool

	// Title is the API title written to the OpenAPI Info Object.
	Title string
	// Version is the documented API version written to the OpenAPI Info Object.
	Version string
	// Summary is the short API summary written to the OpenAPI Info Object.
	Summary string
	// Description is the API description written to the OpenAPI Info Object.
	Description string
	// TermsOfService is the API terms-of-service URL.
	TermsOfService string
	// Contact describes the API contact.
	Contact *Contact
	// License describes the API license.
	License *License
	// Servers lists the API server alternatives.
	Servers []Server
	// Tags declares the document-level tag metadata.
	Tags []Tag
	// ExternalDocs links to additional API documentation.
	ExternalDocs *ExternalDocumentation

	// UIPath is the exact route serving the Swagger UI.
	UIPath string
	// SpecPath is the exact route serving the OpenAPI JSON document.
	SpecPath string
	// DisableUI prevents registration of the Swagger UI and its assets.
	DisableUI bool
	// DisableSpec prevents public registration of the OpenAPI JSON endpoint.
	// It requires DisableUI because the embedded UI consumes that endpoint.
	DisableSpec bool

	// IncludeUndocumented includes a basic operation for eligible routes that
	// were not explicitly registered with the Documenter.
	IncludeUndocumented bool
	// IncludeInternal allows internal Arandu routes to pass the default filter.
	IncludeInternal bool
	// Filter applies explicit route metadata and predicate filters.
	Filter RouteFilter

	// CacheSpec reuses serialized document bytes until registry or public route
	// metadata changes.
	CacheSpec bool
	// PersistAuthorization asks Swagger UI to retain authorization data.
	PersistAuthorization bool
	// DisableTryItOut removes interactive request execution from Swagger UI.
	DisableTryItOut bool

	// UIMiddleware wraps the UI and embedded asset handlers in declaration order.
	UIMiddleware []Middleware
	// SpecMiddleware wraps the OpenAPI JSON handler in declaration order.
	SpecMiddleware []Middleware
}

// Validate reports configuration that cannot be served or generated safely.
// Dormant settings are ignored while Enabled is false so the zero value remains
// a complete opt-out switch.
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if strings.TrimSpace(c.Title) == "" {
		return fmt.Errorf("swagger: Config.Title is required when documentation is enabled")
	}
	if strings.TrimSpace(c.Version) == "" {
		return fmt.Errorf("swagger: Config.Version is required when documentation is enabled")
	}
	if issues := validateGenerationConfig(c); len(issues) > 0 {
		return issues[0]
	}

	return nil
}

func validateOperationalConfig(c Config) []error {
	var issues []error
	if c.DisableSpec && !c.DisableUI {
		issues = append(issues, fmt.Errorf("swagger: Config.DisableSpec requires Config.DisableUI because the UI reads the public specification endpoint"))
	}

	resolved := c.withDefaults()
	if err := validateStaticEndpointPath("Config.UIPath", resolved.UIPath); err != nil {
		issues = append(issues, err)
	}
	if err := validateStaticEndpointPath("Config.SpecPath", resolved.SpecPath); err != nil {
		issues = append(issues, err)
	}
	if !resolved.DisableUI && !resolved.DisableSpec {
		if resolved.UIPath == resolved.SpecPath {
			issues = append(issues, fmt.Errorf("swagger: Config.UIPath and Config.SpecPath resolve to the same path %q", resolved.UIPath))
		}
		assetPath := resolved.UIPath + "/assets"
		if resolved.SpecPath == assetPath || strings.HasPrefix(resolved.SpecPath, assetPath+"/") {
			issues = append(issues, fmt.Errorf("swagger: Config.SpecPath %q conflicts with the UI asset namespace %q", resolved.SpecPath, assetPath))
		}
		initializerPath := resolved.UIPath + "/swagger-initializer.js"
		if resolved.SpecPath == initializerPath {
			issues = append(issues, fmt.Errorf("swagger: Config.SpecPath %q conflicts with the UI initializer endpoint", resolved.SpecPath))
		}
	}

	for index, middleware := range c.UIMiddleware {
		if middleware == nil {
			issues = append(issues, fmt.Errorf("swagger: Config.UIMiddleware[%d] is nil", index))
		}
	}
	for index, middleware := range c.SpecMiddleware {
		if middleware == nil {
			issues = append(issues, fmt.Errorf("swagger: Config.SpecMiddleware[%d] is nil", index))
		}
	}
	if err := c.Filter.validate(); err != nil {
		issues = append(issues, err)
	}
	return issues
}

// withDefaults returns a copy containing the effective endpoint paths.
func (c Config) withDefaults() Config {
	if c.UIPath == "" {
		c.UIPath = DefaultUIPath
	}
	if c.SpecPath == "" {
		if c.UIPath == DefaultUIPath {
			c.SpecPath = DefaultSpecPath
		} else {
			c.SpecPath = c.UIPath + "/openapi.json"
		}
	}
	return c
}

func cloneConfig(source Config) Config {
	copy := source
	copy.Contact = cloneContact(source.Contact)
	copy.License = cloneLicense(source.License)
	copy.Servers = cloneServers(source.Servers)
	copy.Tags = cloneTags(source.Tags)
	copy.ExternalDocs = cloneExternalDocs(source.ExternalDocs)
	copy.Filter = cloneRouteFilter(source.Filter)
	copy.UIMiddleware = append([]Middleware(nil), source.UIMiddleware...)
	copy.SpecMiddleware = append([]Middleware(nil), source.SpecMiddleware...)
	return copy
}

func cloneRouteFilter(source RouteFilter) RouteFilter {
	copy := source
	copy.IncludePrefixes = append([]string(nil), source.IncludePrefixes...)
	copy.ExcludePrefixes = append([]string(nil), source.ExcludePrefixes...)
	copy.IncludeNames = append([]string(nil), source.IncludeNames...)
	copy.ExcludeNames = append([]string(nil), source.ExcludeNames...)
	copy.IncludeModules = append([]string(nil), source.IncludeModules...)
	copy.ExcludeModules = append([]string(nil), source.ExcludeModules...)
	copy.IncludeMethods = append([]string(nil), source.IncludeMethods...)
	copy.ExcludeMethods = append([]string(nil), source.ExcludeMethods...)
	copy.IncludeDomains = append([]string(nil), source.IncludeDomains...)
	copy.ExcludeDomains = append([]string(nil), source.ExcludeDomains...)
	return copy
}

func validateStaticEndpointPath(field, value string) error {
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("swagger: %s %q must be an absolute path starting with /", field, value)
	}
	if value == "/" {
		return fmt.Errorf("swagger: %s cannot claim the application root path", field)
	}
	if path.Clean(value) != value {
		return fmt.Errorf("swagger: %s %q must be clean and must not end with /", field, value)
	}
	if strings.ContainsAny(value, "?#") {
		return fmt.Errorf("swagger: %s %q must not contain a query or fragment", field, value)
	}
	if strings.ContainsAny(value, "{}*%") {
		return fmt.Errorf("swagger: %s %q must be static and must not contain route parameters, wildcards, or percent escapes", field, value)
	}
	if strings.HasPrefix(value, "/_arandu/") || value == "/_arandu" {
		return fmt.Errorf("swagger: %s %q uses the reserved Arandu internal path", field, value)
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) >= 0 {
		return fmt.Errorf("swagger: %s %q must not contain whitespace or control characters", field, value)
	}
	return nil
}

func (f RouteFilter) validate() error {
	if err := validatePrefixes("IncludePrefixes", f.IncludePrefixes); err != nil {
		return err
	}
	if err := validatePrefixes("ExcludePrefixes", f.ExcludePrefixes); err != nil {
		return err
	}

	fields := []struct {
		name   string
		values []string
	}{
		{name: "IncludeNames", values: f.IncludeNames},
		{name: "ExcludeNames", values: f.ExcludeNames},
		{name: "IncludeModules", values: f.IncludeModules},
		{name: "ExcludeModules", values: f.ExcludeModules},
		{name: "IncludeMethods", values: f.IncludeMethods},
		{name: "ExcludeMethods", values: f.ExcludeMethods},
		{name: "IncludeDomains", values: f.IncludeDomains},
		{name: "ExcludeDomains", values: f.ExcludeDomains},
	}
	for _, field := range fields {
		for index, value := range field.values {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("swagger: Config.Filter.%s[%d] must not be blank", field.name, index)
			}
		}
	}

	overlaps := []struct {
		includeName string
		include     []string
		excludeName string
		exclude     []string
		foldCase    bool
	}{
		{includeName: "IncludePrefixes", include: f.IncludePrefixes, excludeName: "ExcludePrefixes", exclude: f.ExcludePrefixes},
		{includeName: "IncludeNames", include: f.IncludeNames, excludeName: "ExcludeNames", exclude: f.ExcludeNames},
		{includeName: "IncludeModules", include: f.IncludeModules, excludeName: "ExcludeModules", exclude: f.ExcludeModules},
		{includeName: "IncludeMethods", include: f.IncludeMethods, excludeName: "ExcludeMethods", exclude: f.ExcludeMethods, foldCase: true},
		{includeName: "IncludeDomains", include: f.IncludeDomains, excludeName: "ExcludeDomains", exclude: f.ExcludeDomains},
	}
	for _, pair := range overlaps {
		if value, ok := overlappingValue(pair.include, pair.exclude, pair.foldCase); ok {
			return fmt.Errorf("swagger: Config.Filter.%s and Config.Filter.%s both contain %q", pair.includeName, pair.excludeName, value)
		}
	}

	return nil
}

func validatePrefixes(field string, prefixes []string) error {
	for index, prefix := range prefixes {
		if !strings.HasPrefix(prefix, "/") {
			return fmt.Errorf("swagger: Config.Filter.%s[%d] %q must start with /", field, index, prefix)
		}
		if strings.ContainsAny(prefix, "?#{}*") {
			return fmt.Errorf("swagger: Config.Filter.%s[%d] %q must be a static path prefix", field, index, prefix)
		}
	}
	return nil
}

func overlappingValue(include, exclude []string, foldCase bool) (string, bool) {
	for _, included := range include {
		for _, excluded := range exclude {
			equal := included == excluded
			if foldCase {
				equal = strings.EqualFold(included, excluded)
			}
			if equal {
				return included, true
			}
		}
	}
	return "", false
}
