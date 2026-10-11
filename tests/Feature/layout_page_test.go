package feature_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/auth"
	hhttp "github.com/arandu-io/hesape/http"
	swagger "github.com/hyz-is/arandu-swagger"
)

// requestState stands in for what an application's pipeline puts on a request
// before the UI route runs: the name the framework carries, the token the
// middleware that protects forms issued, and, when signedIn, the subject the
// application's UIMiddleware loaded from the session.
func requestState(token string, signedIn bool) swagger.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := hhttp.WithAppName(r.Context(), "Application Name")
			ctx = hhttp.WithCSRFToken(ctx, token)
			if signedIn {
				ctx = auth.WithSubject(ctx, auth.Subject{ID: "user-1", Tenant: "tenant-1"})
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// mountWithNavigation mounts the module beside the routes an application's
// layout links to, named as view.New reads them.
func mountWithNavigation(t *testing.T, config swagger.Config) *fhttp.Router {
	t.Helper()
	router := fhttp.NewRouter()
	noop := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	router.Get("/dashboard", noop).Name("home")
	router.Get("/auth/login", noop).Name("auth.login")
	router.Post("/auth/logout", noop).Name("auth.logout")
	router.Get("/auth/register", noop).Name("auth.register")
	module := newModule(t, config)
	module.Routes(router.ForModule(module.Name()))
	return router
}

type layoutViewData struct {
	View  string                  `json:"view"`
	Data  swagger.SwaggerViewData `json:"data"`
	Token string                  `json:"token"`
}

func layoutData(t *testing.T, router *fhttp.Router) layoutViewData {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/docs", nil)
	request.Header.Set("Accept", hhttp.ViewDataMediaType)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET /docs as view data answered %d: %s", response.Code, response.Body.String())
	}
	var data layoutViewData
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatalf("decode view data: %v\n%s", err, response.Body.String())
	}
	return data
}

func TestViewPageCarriesTheRequestStateTheApplicationLayoutDraws(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.UIMiddleware = []swagger.Middleware{requestState("token-from-the-csrf-middleware", true)}
	got := layoutData(t, mountWithNavigation(t, config))
	page := got.Data.Page

	if page.Token != "token-from-the-csrf-middleware" || got.Token != page.Token {
		t.Errorf("token = %q (answer %q), want the one the forms middleware issued", page.Token, got.Token)
	}
	if !page.Authenticated {
		t.Error("Authenticated = false for a request carrying a signed-in subject; the layout draws no sign-out form")
	}
	for name, pair := range map[string][2]string{
		"LoginURL":    {page.LoginURL, "/auth/login"},
		"LogoutURL":   {page.LogoutURL, "/auth/logout"},
		"RegisterURL": {page.RegisterURL, "/auth/register"},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s = %q, want %q from the route table", name, pair[0], pair[1])
		}
	}
}

func TestViewPageForAGuestDrawsNoSignedInState(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.UIMiddleware = []swagger.Middleware{requestState("guest-token", false)}
	page := layoutData(t, mountWithNavigation(t, config)).Data.Page

	if page.Authenticated {
		t.Error("Authenticated = true with no subject on the request")
	}
	if page.Token != "guest-token" {
		t.Errorf("token = %q, want the guest's guest-token", page.Token)
	}
}

func TestViewPageKeepsTheBrandTheModuleDecides(t *testing.T) {
	t.Parallel()

	logo := enabledConfig()
	logo.Description = "HTTP API documentation."
	logo.Theme.Title = "Billing API"
	logo.Theme.Logo = &swagger.ThemeLogo{URL: "/brand/acme.svg", Alt: "Acme", Href: "/acme"}
	themed := enabledConfig()
	themed.Theme.Title = "Billing API"

	for _, tc := range []struct {
		name               string
		config             swagger.Config
		title, brand, home string
	}{
		{"Theme.Logo.Alt", logo, "Billing API", "Acme", "/acme"},
		{"Theme.Title", themed, "Billing API", "Billing API", "/"},
		{"Config.Title", enabledConfig(), "Example API", "Example API", "/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			config := tc.config
			config.UIMiddleware = []swagger.Middleware{requestState("token", true)}
			page := layoutData(t, mountWithNavigation(t, config)).Data.Page

			if page.Title != tc.title {
				t.Errorf("Title = %q, want %q", page.Title, tc.title)
			}
			if page.AppName != tc.brand {
				t.Errorf("AppName = %q, want %q, not the application name on the request", page.AppName, tc.brand)
			}
			// The route table names /dashboard home; the module's home link wins.
			if page.HomeURL != tc.home {
				t.Errorf("HomeURL = %q, want %q", page.HomeURL, tc.home)
			}
			if page.Description != tc.config.Description {
				t.Errorf("Description = %q, want %q", page.Description, tc.config.Description)
			}
			if page.Canonical != "/docs" || page.Path != "/docs" {
				t.Errorf("Canonical, Path = %q, %q, want the UIPath /docs", page.Canonical, page.Path)
			}
		})
	}
}
