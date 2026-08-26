# Explicit bootstrap wiring

Arandu Swagger is constructed and registered in `bootstrap/app.go`. There is
no provider, container binding, package singleton, or discovery step.

## 1. Construct the documentation module

```go
docs, err := swagger.New(swagger.Config{
	Enabled:     cfg.App.Env == config.EnvDev,
	Title:       "Example API",
	Version:     "1.0.0",
	Description: "HTTP API documentation.",
})
if err != nil {
	return App{}, err
}
```

## 2. Register shared components

Components belong to this module instance. Registering the same name and same
content is idempotent; registering the same name with different content is an
error.

```go
if err := docs.Schema("Problem", problemSchema); err != nil {
	return App{}, err
}
if err := docs.SecurityScheme("bearerAuth", swagger.HTTPBearer("JWT")); err != nil {
	return App{}, err
}
```

## 3. Pass only the capability each module needs

An application module normally stores the small interface rather than the full
Swagger module:

```go
type Module struct {
	docs swagger.Documenter
}

func New(cfg Config, db *data.DB, sessions security.Sessions, docs swagger.Documenter) (*Module, error) {
	return &Module{docs: docs}, nil
}
```

This keeps route documentation explicit and makes the application module easy
to test with a dedicated `swagger.Registry`.

## 4. Register Swagger last

```go
usersModule, err := users.New(users.Config{}, db, sessions, docs)
if err != nil {
	return App{}, err
}

app.Register(usersModule, billingModule, docs)
```

Registering it last makes the composition order readable and ensures its own
routes appear after application routes. Correct generation is also lazy: its
`Routes` method stores the shared router pointer, and the first request after
`foundation.Application.Boot` reads `Router.Routes()`. It does not take a route
snapshot inside `New` or while earlier modules are still registering.

Do not call `Generate` from another module's `Routes` method. Generate after
application boot, from a test after all routes are registered, or through the
specification endpoint.

The complete compiling form is
[`examples/basic/basic.go`](../examples/basic/basic.go).
