// Package basic demonstrates explicit Arandu Swagger wiring and route-local
// documentation with native Hesape schemas.
package basic

import (
	"net/http"

	"github.com/arandu-io/framework/foundation"
	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/jsonschema"
	swagger "github.com/hyz-is/arandu-swagger"
)

// UsersModule is an application module that depends only on the ability to
// document routes.
type UsersModule struct {
	docs swagger.Documenter
}

// NewUsersModule builds the example application module.
func NewUsersModule(docs swagger.Documenter) *UsersModule {
	return &UsersModule{docs: docs}
}

// Name returns the module's stable Arandu slug.
func (m *UsersModule) Name() string { return "users" }

// Routes registers and documents the example route at the same declaration
// site.
func (m *UsersModule) Routes(r *fhttp.Router) {
	route := r.Post("/api/users", m.store).Name("users.store")

	m.docs.Route(route).
		Summary("Create a user").
		Description("Creates a user in the authenticated tenant.").
		Tags("Users").
		QueryParameter(
			"include",
			jsonschema.String().Enum("profile", "permissions"),
			swagger.Description("Relations to include in the response."),
		).
		RequestBody(
			swagger.JSONRef("CreateUser").Required().
				Example(map[string]any{"name": "Ada", "email": "ada@example.test"}),
		).
		Response(http.StatusCreated, "User created", swagger.JSONRef("User")).
		Response(http.StatusUnprocessableEntity, "Validation failed", swagger.JSONRef("Problem")).
		Security("bearerAuth")
}

func (m *UsersModule) store(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusCreated)
}

// Register creates the Swagger module, registers shared components, gives the
// small Documenter interface to the application module, and registers Swagger
// last. This is the shape used from bootstrap/app.go.
func Register(app *foundation.Application, enabled bool) (*swagger.Module, error) {
	docs, err := swagger.New(swagger.Config{
		Enabled:     enabled,
		Title:       "Example API",
		Version:     "1.0.0",
		Description: "HTTP API documentation.",
		UIPath:      "/docs",
		SpecPath:    "/docs/openapi.json",
	})
	if err != nil {
		return nil, err
	}

	if err := docs.Schema("CreateUser", createUserSchema()); err != nil {
		return nil, err
	}
	if err := docs.Schema("User", userSchema()); err != nil {
		return nil, err
	}
	if err := docs.Schema("Problem", problemSchema()); err != nil {
		return nil, err
	}
	if err := docs.SecurityScheme("bearerAuth", swagger.HTTPBearer("JWT")); err != nil {
		return nil, err
	}

	users := NewUsersModule(docs)
	app.Register(users, docs)
	return docs, nil
}

func createUserSchema() jsonschema.Type {
	return jsonschema.Object(
		jsonschema.Prop("name", jsonschema.String().Min(1).Required()),
		jsonschema.Prop("email", jsonschema.String().Format("email").Required()),
	)
}

func userSchema() jsonschema.Type {
	return jsonschema.Object(
		jsonschema.Prop("id", jsonschema.String().Format("uuid").Required()),
		jsonschema.Prop("name", jsonschema.String().Required()),
		jsonschema.Prop("email", jsonschema.String().Format("email").Required()),
	)
}

func problemSchema() jsonschema.Type {
	return jsonschema.Object(
		jsonschema.Prop("message", jsonschema.String().Required()),
	)
}
