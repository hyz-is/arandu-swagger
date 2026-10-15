package feature_test

import (
	"net/http"
	"strings"
	"testing"

	fhttp "github.com/arandu-io/framework/http"

	swagger "github.com/hyz-is/arandu-swagger"
)

// uiPolicies returns the content security policy the docs.swagger view and
// the embedded page answer with under config.
func uiPolicies(t *testing.T, config swagger.Config) map[string]string {
	t.Helper()
	embedded := config
	embedded.ViewName = unpublishedView
	embeddedRouter, _ := mount(t, embedded)
	return answeredPolicies(t, map[string]*fhttp.Router{
		"the docs.swagger view": mountRendered(t, config),
		"the embedded page":     embeddedRouter,
	})
}

// embeddedPolicy returns the content security policy the embedded page
// answers with under config.
func embeddedPolicy(t *testing.T, config swagger.Config) map[string]string {
	t.Helper()
	config.ViewName = unpublishedView
	router, _ := mount(t, config)
	return answeredPolicies(t, map[string]*fhttp.Router{"the embedded page": router})
}

func answeredPolicies(t *testing.T, routers map[string]*fhttp.Router) map[string]string {
	t.Helper()
	policies := map[string]string{}
	for name, router := range routers {
		response := request(router, "/docs")
		if response.Code != http.StatusOK {
			t.Fatalf("%s answered %d: %s", name, response.Code, response.Body.String())
		}
		policies[name] = response.Header().Get("Content-Security-Policy")
	}
	return policies
}

func TestDefaultUIPolicyLoadsImagesOnlyFromItsOwnOriginAndDataURIs(t *testing.T) {
	t.Parallel()

	for name, policy := range uiPolicies(t, enabledConfig()) {
		if got := directives(t, policy)["img-src"]; got != "'self' data:" {
			t.Errorf("%s: img-src = %q, want 'self' data:", name, got)
		}
		for _, token := range strings.Fields(strings.ReplaceAll(policy, ";", " ")) {
			if token == "https:" || token == "http:" || token == "*" {
				t.Errorf("%s: policy %q admits the whole %s source", name, policy, token)
			}
		}
	}
}

func TestUIPolicyAdmitsExactlyTheOriginsTheConfigurationNames(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		mutate func(*swagger.Config)
		want   string
		// embeddedOnly marks a logo the published view refuses to draw, so
		// only the embedded page answers it.
		embeddedOnly bool
	}{
		{
			name: "an external logo",
			mutate: func(c *swagger.Config) {
				c.Theme.Logo = &swagger.ThemeLogo{URL: "https://cdn.example.com/brand/logo.svg?v=2"}
			},
			want: "'self' data: https://cdn.example.com",
		},
		{
			name: "an external logo with a port and an upper-case host",
			mutate: func(c *swagger.Config) {
				c.Theme.Logo = &swagger.ThemeLogo{URL: "https://CDN.Example.com:8443/logo.png"}
			},
			want: "'self' data: https://cdn.example.com:8443",
		},
		{
			name: "the default port",
			mutate: func(c *swagger.Config) {
				c.Theme.Logo = &swagger.ThemeLogo{URL: "https://cdn.example.com:443/logo.png"}
			},
			want: "'self' data: https://cdn.example.com",
		},
		{
			name: "an external favicon",
			mutate: func(c *swagger.Config) {
				c.Theme.Favicon = "https://static.example.net/favicon.ico"
			},
			want: "'self' data: https://static.example.net",
		},
		{
			name: "a logo, a favicon and listed origins, each once",
			mutate: func(c *swagger.Config) {
				c.Theme.Logo = &swagger.ThemeLogo{URL: "https://cdn.example.com/logo.svg"}
				c.Theme.Favicon = "https://cdn.example.com/favicon.ico"
				c.ImageOrigins = []string{"https://img.example.org", "https://cdn.example.com/", "https://img.example.org:8443"}
			},
			want: "'self' data: https://cdn.example.com https://img.example.org https://img.example.org:8443",
		},
		{
			name: "a same-origin logo and favicon",
			mutate: func(c *swagger.Config) {
				c.Theme.Logo = &swagger.ThemeLogo{URL: "/brand/acme.svg"}
				c.Theme.Favicon = "favicon.png"
			},
			want: "'self' data:",
		},
		{
			name: "a data URI logo",
			mutate: func(c *swagger.Config) {
				c.Theme.Logo = &swagger.ThemeLogo{URL: "data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg'/>"}
			},
			want:         "'self' data:",
			embeddedOnly: true,
		},
	} {
		config := enabledConfig()
		tc.mutate(&config)
		policies := embeddedPolicy(t, config)
		if !tc.embeddedOnly {
			policies = uiPolicies(t, config)
		}
		for page, policy := range policies {
			if got := directives(t, policy)["img-src"]; got != tc.want {
				t.Errorf("%s, %s: img-src = %q, want %q", tc.name, page, got, tc.want)
			}
			if strings.Contains(policy, "/brand") || strings.Contains(policy, "logo") || strings.Contains(policy, "favicon") {
				t.Errorf("%s, %s: policy %q carries a path", tc.name, page, policy)
			}
		}
	}
}

func TestUIPolicyWithImageOriginsStillDiffersOnlyInFormAction(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.Theme.Logo = &swagger.ThemeLogo{URL: "https://cdn.example.com/logo.svg"}
	config.ImageOrigins = []string{"https://img.example.org"}
	policies := uiPolicies(t, config)
	view := directives(t, policies["the docs.swagger view"])
	embedded := directives(t, policies["the embedded page"])
	if view["form-action"] != "'self'" || embedded["form-action"] != "'none'" {
		t.Errorf("form-action = %q and %q, want 'self' for the view and 'none' for the embedded page", view["form-action"], embedded["form-action"])
	}
	if len(view) != len(embedded) {
		t.Fatalf("the policies have %d and %d directives", len(view), len(embedded))
	}
	for name, value := range embedded {
		if name != "form-action" && view[name] != value {
			t.Errorf("the view's %s = %q, want the embedded page's %q", name, view[name], value)
		}
	}
}

func TestNewRefusesImageURLsTheDirectiveCannotNameExactly(t *testing.T) {
	t.Parallel()

	logo := func(url string) func(*swagger.Config) {
		return func(c *swagger.Config) { c.Theme.Logo = &swagger.ThemeLogo{URL: url} }
	}
	favicon := func(url string) func(*swagger.Config) {
		return func(c *swagger.Config) { c.Theme.Favicon = url }
	}
	origins := func(values ...string) func(*swagger.Config) {
		return func(c *swagger.Config) { c.ImageOrigins = values }
	}

	for _, tc := range []struct {
		name   string
		mutate func(*swagger.Config)
		want   string
	}{
		{"an http logo", logo("http://cdn.example.com/logo.svg"), "Config.Theme.Logo.URL"},
		{"a protocol-relative logo", logo("//cdn.example.com/logo.svg"), "Config.Theme.Logo.URL"},
		{"a javascript logo", logo("javascript:alert(1)"), "Config.Theme.Logo.URL"},
		{"a logo with credentials", logo("https://user:secret@cdn.example.com/logo.svg"), "must not carry credentials"},
		{"a logo on a wildcard host", logo("https://*.example.com/logo.svg"), "Config.Theme.Logo.URL"},
		{"a logo on an IPv6 literal", logo("https://[::1]/logo.svg"), "Config.Theme.Logo.URL"},
		{"a logo with an invalid port", logo("https://cdn.example.com:99999/logo.svg"), "Config.Theme.Logo.URL"},
		{"a logo with whitespace", logo("https://cdn.example.com/logo.svg; img-src *"), "whitespace"},
		{"an http favicon", favicon("http://static.example.net/favicon.ico"), "Config.Theme.Favicon"},
		{"the https scheme alone", origins("https:"), "Config.ImageOrigins[0]"},
		{"an http origin", origins("http://img.example.org"), "Config.ImageOrigins[0]"},
		{"a wildcard origin", origins("https://img.example.org", "https://*.example.org"), "Config.ImageOrigins[1]"},
		{"a bare wildcard", origins("*"), "wildcard"},
		{"an origin with a path", origins("https://img.example.org/images"), "without a path"},
		{"an origin with a query", origins("https://img.example.org?x=1"), "without a path"},
		{"an origin with a fragment", origins("https://img.example.org#top"), "without a path"},
		{"a host without a scheme", origins("img.example.org"), "Config.ImageOrigins[0]"},
		{"a blank origin", origins(" "), "Config.ImageOrigins[0]"},
		{"a quoted keyword", origins("'unsafe-inline'"), "Config.ImageOrigins[0]"},
	} {
		config := enabledConfig()
		tc.mutate(&config)
		_, err := swagger.New(config)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: New error = %v, want one naming %q", tc.name, err, tc.want)
		}
		if validateErr := config.Validate(); validateErr == nil {
			t.Errorf("%s: Validate accepted the configuration", tc.name)
		}
	}
}

func TestDisabledConfigurationIgnoresImageURLs(t *testing.T) {
	t.Parallel()

	config := swagger.Config{
		Theme:        swagger.Theme{Logo: &swagger.ThemeLogo{URL: "http://cdn.example.com/logo.svg"}},
		ImageOrigins: []string{"https://*.example.org"},
	}
	if _, err := swagger.New(config); err != nil {
		t.Fatalf("New(disabled) error = %v, want the zero-value opt-out to stay valid", err)
	}
}

func TestImageOriginsAreSnapshotAtNew(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.ImageOrigins = []string{"https://img.example.org"}
	config.ViewName = unpublishedView
	router, _ := mount(t, config)
	config.ImageOrigins[0] = "https://evil.example.com"

	policy := request(router, "/docs").Header().Get("Content-Security-Policy")
	if got := directives(t, policy)["img-src"]; got != "'self' data: https://img.example.org" {
		t.Errorf("img-src = %q, want the origin configured when New ran", got)
	}
}
