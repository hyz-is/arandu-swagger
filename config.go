package swagger

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
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

	// JSONSchemaDialect overrides the default OpenAPI 3.1 schema dialect URI.
	// When empty, it defaults to "https://spec.openapis.org/oas/3.1/dialect/base".
	JSONSchemaDialect string

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

	// Locale specifies the UI language and localization (e.g. "pt-BR", "en").
	// When empty, it defaults to "en".
	Locale string
	// Translations supplies custom key-value translation mappings to override
	// or extend Swagger UI text strings.
	Translations map[string]string
	// ViewName specifies the Kyse view name rendered for the UI endpoint.
	// Defaults to "docs.swagger". If the view is not registered or fails to render,
	// the embedded standalone HTML shell is served as fallback.
	ViewName string

	// Theme configures visual styling, dark mode, colors, branding, and HTMX behavior.
	Theme Theme

	// ImageOrigins lists the exact HTTPS origins, such as
	// "https://cdn.example.com", beyond its own origin and data: URIs that the
	// UI page may load images from, for example an image in an API description.
	// The origin of an absolute Theme.Logo.URL or Theme.Favicon is admitted
	// without being listed here. Each entry is a scheme and host with an
	// optional port; a path, wildcard, or scheme other than https is rejected.
	ImageOrigins []string

	// UIMiddleware wraps the UI and embedded asset handlers in declaration order.
	UIMiddleware []Middleware
	// SpecMiddleware wraps the OpenAPI JSON handler in declaration order.
	SpecMiddleware []Middleware
}

// Theme controls visual styling, dark mode, custom colors, branding, and HTMX behavior.
type Theme struct {
	// Title overrides the HTML document title. When empty, Config.Title is used.
	Title string
	// DarkMode starts the UI in the dark palette.
	DarkMode bool
	// Locale specifies the UI language (e.g. "pt-BR", "en").
	Locale string
	// Translations supplies custom translation overrides.
	Translations map[string]string
	// ViewName specifies the Kyse view name rendered for the UI endpoint.
	ViewName string
	// PrimaryColor sets the accent color for buttons, badges, links, and highlights
	// (e.g. "#14b8a6", "#5eead4", or any valid CSS color).
	PrimaryColor string
	// BackgroundColor overrides the page background color (default "#0a0a0a" in dark mode).
	BackgroundColor string
	// CardColor overrides the card/container background color (default "#121214" in dark mode).
	CardColor string
	// TextColor overrides the primary text color (default "#f4f4f5" in dark mode).
	TextColor string
	// MutedColor overrides subtle/secondary text (default "#a1a1aa" in dark mode).
	MutedColor string
	// BorderColor overrides borders (default "#27272a" in dark mode).
	BorderColor string
	// Logo configures an optional branding logo in the topbar header. When nil,
	// the topbar draws no image, names the documentation by its title, and links
	// home to "/".
	Logo *ThemeLogo
	// BackURL configures the "Voltar para o site" link target in the topbar.
	// When empty, defaults to ThemeLogo.Href or "/".
	BackURL string
	// BackText overrides the back link text (defaults to "Voltar para o site" for pt/pt-BR and "Back to site" for other locales).
	BackText string
	// BackTarget is the anchor target for the back link (defaults to "_self").
	BackTarget string
	// DisableThemeToggle hides the light/dark theme switch in the topbar.
	DisableThemeToggle bool
	// Favicon is the URL to a custom favicon (e.g. "/favicon.ico" or "/favicon.png").
	// It follows the rules of ThemeLogo.URL: an absolute URL must use https, and
	// its origin is added to the page's img-src directive.
	Favicon string
	// CustomCSS is custom CSS appended to the theme stylesheet.
	CustomCSS string
	// CustomJS is custom JavaScript served and executed on the page.
	CustomJS string
	// HTMX configures HTMX-specific compatibility options.
	HTMX HTMXConfig
}

// ThemeLogo configures the brand logo in the Swagger UI topbar.
type ThemeLogo struct {
	// URL is the image URL or data URI of the logo. A same-origin path or a
	// data URI needs nothing more; an absolute URL must use https, and its
	// origin is added to the page's img-src directive.
	URL string
	// Href is the link target when clicking the logo (defaults to "/").
	Href string
	// Alt is the alternative text for the logo (defaults to the API title).
	Alt string
	// Target is the anchor target, e.g. "_blank" or "_self" (defaults to "_self").
	Target string
}

// HTMXConfig configures HTMX compatibility behavior.
type HTMXConfig struct {
	// Enabled adds HTMX lifecycle support to Swagger UI initialization.
	Enabled bool
	// Boost controls whether the Swagger UI container sets hx-boost="false"
	// to prevent HTMX from intercepting Swagger UI's internal buttons and forms.
	// When Enabled is true, Boost defaults to true.
	Boost bool
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
		themeCSSPath := resolved.UIPath + "/theme.css"
		if resolved.SpecPath == themeCSSPath {
			issues = append(issues, fmt.Errorf("swagger: Config.SpecPath %q conflicts with the UI theme stylesheet endpoint", resolved.SpecPath))
		}
		themeJSPath := resolved.UIPath + "/theme.js"
		if resolved.SpecPath == themeJSPath {
			issues = append(issues, fmt.Errorf("swagger: Config.SpecPath %q conflicts with the UI theme script endpoint", resolved.SpecPath))
		}
	}

	if c.Theme.Logo != nil && strings.TrimSpace(c.Theme.Logo.URL) == "" {
		issues = append(issues, fmt.Errorf("swagger: Config.Theme.Logo.URL must not be blank when Logo is configured"))
	}
	if _, err := imageOrigins(c); err != nil {
		issues = append(issues, err)
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

// HasTheme reports whether any visual styling, branding, or custom asset is configured.
func (c Config) HasTheme() bool {
	return c.Theme.DarkMode ||
		c.Theme.PrimaryColor != "" ||
		c.Theme.BackgroundColor != "" ||
		c.Theme.CardColor != "" ||
		c.Theme.TextColor != "" ||
		c.Theme.MutedColor != "" ||
		c.Theme.BorderColor != "" ||
		c.Theme.CustomCSS != "" ||
		c.Theme.Logo != nil ||
		c.Theme.BackURL != "" ||
		!c.Theme.DisableThemeToggle
}

// withDefaults returns a copy containing the effective endpoint paths and defaults.
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
	if c.Locale == "" && c.Theme.Locale != "" {
		c.Locale = c.Theme.Locale
	}
	if c.Locale == "" {
		c.Locale = "en"
	}
	if c.ViewName == "" {
		if c.Theme.ViewName != "" {
			c.ViewName = c.Theme.ViewName
		} else {
			c.ViewName = "docs.swagger"
		}
	}
	if len(c.Translations) == 0 && len(c.Theme.Translations) > 0 {
		c.Translations = cloneTranslations(c.Theme.Translations)
	}
	if c.Theme.BackURL == "" {
		if c.Theme.Logo != nil && c.Theme.Logo.Href != "" {
			c.Theme.BackURL = c.Theme.Logo.Href
		} else {
			c.Theme.BackURL = "/"
		}
	}
	if c.Theme.BackText == "" {
		if strings.HasPrefix(strings.ToLower(c.Locale), "pt") {
			c.Theme.BackText = "Voltar para o site"
		} else {
			c.Theme.BackText = "Back to site"
		}
	}
	if c.Theme.BackTarget == "" {
		c.Theme.BackTarget = "_self"
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
	copy.Translations = cloneTranslations(source.Translations)
	copy.Theme = cloneTheme(source.Theme)
	copy.ImageOrigins = append([]string(nil), source.ImageOrigins...)
	copy.UIMiddleware = append([]Middleware(nil), source.UIMiddleware...)
	copy.SpecMiddleware = append([]Middleware(nil), source.SpecMiddleware...)
	return copy
}

func cloneTheme(source Theme) Theme {
	copy := source
	copy.Translations = cloneTranslations(source.Translations)
	if source.Logo != nil {
		logoCopy := *source.Logo
		copy.Logo = &logoCopy
	}
	return copy
}

func cloneTranslations(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	copy := make(map[string]string, len(source))
	for k, v := range source {
		copy[k] = v
	}
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

// imageOrigins returns the origins the UI page's img-src directive admits
// beyond 'self' and data:, each once and in the order the configuration names
// them: the origin of an absolute Theme.Logo.URL, of an absolute
// Theme.Favicon, then every ImageOrigins entry. A same-origin path or a data:
// URI adds nothing. It reports the first configured value that is not one of
// those or an https URL whose origin the directive can name exactly.
func imageOrigins(c Config) ([]string, error) {
	var origins []string
	add := func(origin string) {
		if origin == "" || slices.Contains(origins, origin) {
			return
		}
		origins = append(origins, origin)
	}

	if c.Theme.Logo != nil && strings.TrimSpace(c.Theme.Logo.URL) != "" {
		origin, err := imageURLOrigin("Config.Theme.Logo.URL", c.Theme.Logo.URL)
		if err != nil {
			return nil, err
		}
		add(origin)
	}
	if c.Theme.Favicon != "" {
		origin, err := imageURLOrigin("Config.Theme.Favicon", c.Theme.Favicon)
		if err != nil {
			return nil, err
		}
		add(origin)
	}
	for index, entry := range c.ImageOrigins {
		origin, err := exactImageOrigin(fmt.Sprintf("Config.ImageOrigins[%d]", index), entry)
		if err != nil {
			return nil, err
		}
		add(origin)
	}
	return origins, nil
}

// imageURLOrigin returns the origin an image URL is loaded from when that
// origin is not the page's own: empty for a data: URI or a same-origin
// reference, the https origin of an absolute URL, and an error for anything
// else.
func imageURLOrigin(field, raw string) (string, error) {
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		return "", nil
	}
	if err := rejectSpaceAndControl(field, raw); err != nil {
		return "", err
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("swagger: %s %q is not a valid URL: %w", field, raw, err)
	}
	if parsed.Scheme == "" && parsed.Host == "" && !strings.HasPrefix(raw, "//") {
		return "", nil
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("swagger: %s %q must be a same-origin path, a data: URI, or an absolute https URL", field, raw)
	}
	return httpsOrigin(field, raw, parsed)
}

// exactImageOrigin validates one ImageOrigins entry, which names an origin
// and nothing more, and returns it in the form the directive writes.
func exactImageOrigin(field, raw string) (string, error) {
	if err := rejectSpaceAndControl(field, raw); err != nil {
		return "", err
	}
	if strings.Contains(raw, "*") {
		return "", fmt.Errorf("swagger: %s %q must name one exact origin, not a wildcard", field, raw)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("swagger: %s %q is not a valid URL: %w", field, raw, err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return "", fmt.Errorf("swagger: %s %q must be an https origin such as \"https://cdn.example.com\"", field, raw)
	}
	if parsed.Opaque != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", fmt.Errorf("swagger: %s %q must be an origin without a path, query, or fragment", field, raw)
	}
	return httpsOrigin(field, raw, parsed)
}

// httpsOrigin returns the scheme, host, and port of an https URL as a CSP
// host source. The host is lowercased and the default port dropped. A host
// the CSP grammar cannot name exactly, such as a wildcard or an IPv6 literal,
// and a URL carrying credentials are rejected.
func httpsOrigin(field, raw string, parsed *url.URL) (string, error) {
	if parsed.User != nil {
		return "", fmt.Errorf("swagger: %s %q must not carry credentials", field, raw)
	}
	host := strings.ToLower(parsed.Hostname())
	if strings.Contains(host, "*") {
		return "", fmt.Errorf("swagger: %s %q must name one exact host, not a wildcard", field, raw)
	}
	if !validSourceHost(host) {
		return "", fmt.Errorf("swagger: %s %q must name a host made of letters, digits, hyphens, and dots", field, raw)
	}
	origin := "https://" + host
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", fmt.Errorf("swagger: %s %q has an invalid port", field, raw)
		}
		if number != 443 {
			origin += ":" + strconv.Itoa(number)
		}
	} else if strings.HasSuffix(parsed.Host, ":") {
		return "", fmt.Errorf("swagger: %s %q has an invalid port", field, raw)
	}
	return origin, nil
}

// validSourceHost reports whether host is a non-empty sequence of dot-separated
// labels of ASCII letters, digits, and hyphens, which is all a CSP host source
// can name without a wildcard.
func validSourceHost(host string) bool {
	if host == "" {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-') {
				return false
			}
		}
	}
	return true
}

func rejectSpaceAndControl(field, raw string) error {
	if strings.IndexFunc(raw, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) >= 0 {
		return fmt.Errorf("swagger: %s %q must not contain whitespace or control characters", field, raw)
	}
	return nil
}
