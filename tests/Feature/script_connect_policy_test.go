package feature_test

import (
	"strings"
	"testing"

	swagger "github.com/hyz-is/arandu-swagger"
)

const (
	beaconScript  = "https://static.cloudflareinsights.com"
	beaconConnect = "https://cloudflareinsights.com"
)

func TestDefaultUIPolicyLoadsScriptsAndConnectsOnlyToItsOwnOrigin(t *testing.T) {
	t.Parallel()

	for name, policy := range uiPolicies(t, enabledConfig()) {
		got := directives(t, policy)
		if got["script-src"] != "'self'" {
			t.Errorf("%s: script-src = %q, want 'self'", name, got["script-src"])
		}
		if got["connect-src"] != "'self'" {
			t.Errorf("%s: connect-src = %q, want 'self'", name, got["connect-src"])
		}
	}
}

// A host that injects a script into every page it serves, such as Cloudflare's
// web analytics beacon, is let through on the documentation page by naming its
// two origins, and by nothing broader.
func TestUIPolicyAdmitsTheScriptAndConnectOriginsTheConfigurationNames(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.ScriptOrigins = []string{beaconScript, "https://static.cloudflareinsights.com/", "https://cdn.example.com:8443"}
	config.ConnectOrigins = []string{beaconConnect}

	for name, policy := range uiPolicies(t, config) {
		got := directives(t, policy)
		if want := "'self' " + beaconScript + " https://cdn.example.com:8443"; got["script-src"] != want {
			t.Errorf("%s: script-src = %q, want %q", name, got["script-src"], want)
		}
		if want := "'self' " + beaconConnect; got["connect-src"] != want {
			t.Errorf("%s: connect-src = %q, want %q", name, got["connect-src"], want)
		}
		if got["img-src"] != "'self' data:" || got["style-src"] != "'self'" || got["default-src"] != "'none'" {
			t.Errorf("%s: listing script and connect origins moved another directive: %q", name, policy)
		}
		for _, token := range strings.Fields(strings.ReplaceAll(policy, ";", " ")) {
			if token == "https:" || token == "http:" || token == "*" {
				t.Errorf("%s: policy %q admits the whole %s source", name, policy, token)
			}
		}
	}
}

func TestUIPolicyWithScriptOriginsStillDiffersOnlyInFormAction(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.ScriptOrigins = []string{beaconScript}
	config.ConnectOrigins = []string{beaconConnect}
	policies := uiPolicies(t, config)
	view := directives(t, policies["the docs.swagger view"])
	embedded := directives(t, policies["the embedded page"])
	if len(view) != len(embedded) {
		t.Fatalf("the policies have %d and %d directives", len(view), len(embedded))
	}
	for name, value := range embedded {
		if name != "form-action" && view[name] != value {
			t.Errorf("the view's %s = %q, want the embedded page's %q", name, view[name], value)
		}
	}
}

func TestNewRefusesScriptAndConnectOriginsTheDirectiveCannotNameExactly(t *testing.T) {
	t.Parallel()

	script := func(values ...string) func(*swagger.Config) {
		return func(c *swagger.Config) { c.ScriptOrigins = values }
	}
	connect := func(values ...string) func(*swagger.Config) {
		return func(c *swagger.Config) { c.ConnectOrigins = values }
	}

	for _, tc := range []struct {
		name   string
		mutate func(*swagger.Config)
		want   string
	}{
		{"the https scheme alone", script("https:"), "Config.ScriptOrigins[0]"},
		{"an http origin", script("http://static.example.org"), "Config.ScriptOrigins[0]"},
		{"a wildcard origin", script(beaconScript, "https://*.example.org"), "Config.ScriptOrigins[1]"},
		{"a bare wildcard", script("*"), "wildcard"},
		{"an origin with a path", script("https://static.example.org/beacon.js"), "without a path"},
		{"a quoted keyword", script("'unsafe-inline'"), "Config.ScriptOrigins[0]"},
		{"an origin carrying a directive", script("https://static.example.org; script-src *"), "whitespace"},
		{"a blank origin", script(" "), "Config.ScriptOrigins[0]"},
		{"an http connect origin", connect("http://api.example.org"), "Config.ConnectOrigins[0]"},
		{"a wss connect origin", connect("wss://api.example.org"), "Config.ConnectOrigins[0]"},
		{"a wildcard connect origin", connect("https://*.example.org"), "Config.ConnectOrigins[0]"},
		{"a connect origin with a query", connect("https://api.example.org?x=1"), "without a path"},
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

func TestDisabledConfigurationIgnoresScriptAndConnectOrigins(t *testing.T) {
	t.Parallel()

	config := swagger.Config{ScriptOrigins: []string{"https://*.example.org"}, ConnectOrigins: []string{"http://x"}}
	if _, err := swagger.New(config); err != nil {
		t.Fatalf("New(disabled) error = %v, want the zero-value opt-out to stay valid", err)
	}
}

func TestScriptAndConnectOriginsAreSnapshotAtNew(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.ScriptOrigins = []string{beaconScript}
	config.ConnectOrigins = []string{beaconConnect}
	config.ViewName = unpublishedView
	router, _ := mount(t, config)
	config.ScriptOrigins[0] = "https://evil.example.com"
	config.ConnectOrigins[0] = "https://evil.example.com"

	got := directives(t, request(router, "/docs").Header().Get("Content-Security-Policy"))
	if got["script-src"] != "'self' "+beaconScript || got["connect-src"] != "'self' "+beaconConnect {
		t.Errorf("script-src = %q and connect-src = %q, want the origins configured when New ran", got["script-src"], got["connect-src"])
	}
}
