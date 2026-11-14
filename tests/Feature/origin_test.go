package feature_test

import (
	"testing"

	swagger "github.com/hyz-is/arandu-swagger"
)

// Config.Origin is the address the application is served from, from its
// configuration. The canonical address of the documentation page is written
// against it, absolute, and a published view writes the address of anything
// else it names with AbsoluteURL.
func TestOriginMakesTheCanonicalAddressAbsolute(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.Origin = "https://Acme.example/"
	got := layoutData(t, mountWithNavigation(t, config))

	if want := "https://acme.example/docs"; got.Data.Page.Canonical != want {
		t.Errorf("Canonical = %q, want %q", got.Data.Page.Canonical, want)
	}
	if got.Data.Origin != "https://acme.example" {
		t.Errorf("Origin = %q, want the origin without its trailing slash", got.Data.Origin)
	}
	if address := got.Data.AbsoluteURL("social-cover.png"); address != "https://acme.example/social-cover.png" {
		t.Errorf("AbsoluteURL = %q", address)
	}
}

// With no origin the canonical address stays the path, as it was, and nothing
// is made absolute out of thin air.
func TestNoOriginLeavesTheCanonicalAddressAPath(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	got := layoutData(t, mountWithNavigation(t, config))

	if got.Data.Page.Canonical != "/docs" {
		t.Errorf("Canonical = %q, want the path %q", got.Data.Page.Canonical, "/docs")
	}
	if address := got.Data.AbsoluteURL("/x"); address != "" {
		t.Errorf("AbsoluteURL = %q with no origin, want empty", address)
	}
}

// The origin is an address and nothing more: a path, a query, credentials or a
// scheme the browser does not open as a site are configuration errors.
func TestOriginRefusesWhatIsNotAnOrigin(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{"acme.example", "ftp://acme.example", "https://acme.example/docs", "https://acme.example?x=1", "https://user@acme.example", "https:///"} {
		config := enabledConfig()
		config.Origin = bad
		if _, err := swagger.New(config); err == nil {
			t.Errorf("Origin %q was accepted", bad)
		}
	}
	config := enabledConfig()
	config.Origin = "http://localhost:8080"
	if _, err := swagger.New(config); err != nil {
		t.Errorf("a local http origin was refused: %v", err)
	}
}
