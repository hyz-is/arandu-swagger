package feature_test

import (
	"strings"
	"testing"

	swagger "github.com/hyz-is/arandu-swagger"
)

func TestUIBootstrapAllowsUnsafeInlineOnlyForSwaggerStyleAttributes(t *testing.T) {
	t.Parallel()

	router, _ := mount(t, enabledConfig())
	page := request(router, swagger.DefaultUIPath)
	csp := page.Header().Get("Content-Security-Policy")
	if strings.Count(csp, "'unsafe-inline'") != 1 || !strings.Contains(csp, "style-src-attr 'unsafe-inline'") {
		t.Fatalf("Content-Security-Policy = %q, want one style-src-attr exception only", csp)
	}

	initializer := request(router, swagger.DefaultUIPath+"/swagger-initializer.js")
	if strings.Count(initializer.Body.String(), "validatorUrl: null") != 1 {
		t.Fatalf("initializer = %q, want exactly one disabled remote validator", initializer.Body.String())
	}
}
