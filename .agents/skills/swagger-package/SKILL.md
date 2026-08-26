---
name: swagger-package
description: Install, wire, configure, and use github.com/hyz-is/arandu-swagger in an Arandu application. Use for package installation, bootstrap wiring, documenting routes, schemas, responses, security, filters, UI/spec endpoints, static generation, or troubleshooting missing documentation.
license: MIT
---

# Using Arandu Swagger

Arandu Swagger is an explicit documentation module. It owns an in-memory
registry and embedded UI assets; it owns no database, migration, repository,
policy, service provider, container, or global registry.

## Install and wire

```sh
go get github.com/hyz-is/arandu-swagger
```

Construct one instance in `bootstrap/app.go`, pass its small `Documenter`
interface to modules that declare API routes, and register Swagger explicitly:

```go
swaggerModule, err := swagger.New(swagger.Config{
	Enabled:   cfg.App.Env == "local",
	Title:     "Example API",
	Version:   "1.0.0",
	UIPath:    "/docs",
	SpecPath:  "/docs/openapi.json",
	CacheSpec: true,
})
if err != nil {
	return App{}, err
}

usersModule, err := users.New(users.Config{}, db, sessions, swaggerModule)
if err != nil {
	return App{}, err
}

app.Register(usersModule, swaggerModule)
```

Registering Swagger last is the clearest convention. Generation is lazy and
reads the shared route table, so routes added by later modules are still visible
on the first generation.

There is no migration step. All four permissions in `arandu.mod.toml` remain
false unless implementation capabilities genuinely change.

## Document beside the route

```go
route := r.Get("/api/users/{id}", http.HandlerFunc(m.show)).
	Name("users.show").
	WhereUuid("id")

m.docs.Route(route).
	Summary("Show a user").
	PathParameter("id", swagger.Description("User identifier.")).
	Response(http.StatusOK, "User found", swagger.JSONRef("User")).
	Response(http.StatusNotFound, "User not found", swagger.JSONRef("Problem")).
	Security("bearerAuth")
```

Register native Hesape schemas and security schemes on the same instance:

```go
if err := swaggerModule.Schema("User", userSchema); err != nil {
	return App{}, err
}
if err := swaggerModule.SecurityScheme("bearerAuth", swagger.HTTPBearer("JWT")); err != nil {
	return App{}, err
}
```

Only explicitly documented routes are published by default. Use
`IncludeUndocumented` deliberately, and narrow the result with `Config.Filter`.
`Hidden` and `Ignore` always remove an operation. Fallback, `/_arandu`, and the
documentation module's own routes are excluded by default.

## Endpoints and export

The defaults are `GET /docs`, `GET /docs/openapi.json`, and versioned assets
below `/docs/assets/`. The browser distribution is embedded and makes no CDN or
runtime download.

Set `DisableUI` to expose JSON only. Set both `DisableUI` and `DisableSpec` while
leaving `Enabled` true for programmatic-only generation through
`Module.Generate` or `Module.GenerateJSON`. The standalone `Generate` and
`GenerateJSON` functions accept a route-table snapshot and a `Registry`.

The zero configuration is disabled. Authentication belongs in the
application-supplied UI and specification middleware; this package does not
invent an authentication system.

## Routing diagnostics

- `ANY` requires an explicit `Methods(...)` list.
- Multi-method routes need automatic method-qualified IDs or `OperationIDFor`.
- Optional path parameters and multi-segment wildcards are rejected because
  OpenAPI path templates cannot represent them faithfully.
- A raw qualified binding such as `{post:slug}` is rejected until Hesape has
  normalized the registered route to `{post}`.
- Generic Go regular expressions are retained only when safely portable to the
  JSON Schema dialect; otherwise provide an explicit parameter schema.
- A generation failure is returned by the programmatic API, sanitized on the
  public endpoint, and available through `foundation.Diagnostic`.

Read `README.md` for the complete API and `docs/architecture.md` for the
framework-backed design decisions.
