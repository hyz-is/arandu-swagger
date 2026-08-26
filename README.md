# Arandu Swagger

Arandu Swagger is the native OpenAPI integration for Arandu applications. It
builds an OpenAPI 3.1.0 document from the route table that the application
already owns, while request bodies, responses, examples, and security remain
explicit beside each route.

The package is intentionally an Arandu module rather than a second framework:

- it uses `framework/http` and the public Hesape route metadata;
- it accepts native `hesape/jsonschema` schemas;
- its registry belongs to one `swagger.Module` instance;
- it generates lazily after application boot and can cache serialized JSON;
- its Swagger UI 5.32.14 assets are embedded and served from the same origin;
- it has no database, migrations, service provider, container, discovery,
  mutable package registry, runtime filesystem access, CDN, or runtime Node.js;
- documentation is disabled unless `Config.Enabled` is explicitly true.

## Install

```bash
go get github.com/arandu-io/swagger
```

No migration or asset build follows installation.

## Quick start

Create the documentation module in `bootstrap/app.go`:

```go
docs, err := swagger.New(swagger.Config{
	Enabled:     cfg.App.Env == config.EnvDev,
	Title:       "Example API",
	Version:     "1.0.0",
	Description: "HTTP API documentation.",
	UIPath:      "/docs",
	SpecPath:    "/docs/openapi.json",
})
if err != nil {
	return App{}, err
}
```

Register shared native schemas and security schemes once:

```go
schemas := map[string]jsonschema.Type{
	"User": jsonschema.Object(
		jsonschema.Prop("id", jsonschema.String().Format("uuid").Required()),
		jsonschema.Prop("name", jsonschema.String().Required()),
	),
	"CreateUser": jsonschema.Object(
		jsonschema.Prop("name", jsonschema.String().Required()),
	),
	"Problem": jsonschema.Object(
		jsonschema.Prop("message", jsonschema.String().Required()),
	),
}
for name, schema := range schemas {
	if err := docs.Schema(name, schema); err != nil {
		return App{}, err
	}
}
if err := docs.SecurityScheme("bearerAuth", swagger.HTTPBearer("JWT")); err != nil {
	return App{}, err
}
```

Pass only `swagger.Documenter` to an application module, then document its
route at the declaration site:

```go
route := r.Get("/api/users/{id}", http.HandlerFunc(m.show)).
	Name("users.show").
	WhereUuid("id")

m.docs.Route(route).
	Summary("Show a user").
	Description("Returns one user by UUID.").
	Tags("Users").
	PathParameter("id", swagger.Description("User identifier.")).
	Response(http.StatusOK, "User found", swagger.JSONRef("User")).
	Response(http.StatusNotFound, "User not found", swagger.JSONRef("Problem")).
	Security("bearerAuth")
```

Register Swagger last in the application's existing module list:

```go
usersModule, err := users.New(users.Config{}, db, sessions, docs)
if err != nil {
	return App{}, err
}

app.Register(
	usersModule,
	billingModule,
	docs,
)
```

Registration remains explicit. `foundation.Application` calls `Routes` for
each module during boot; Swagger captures that shared router and builds the
document only on the first specification request. The complete compilable
wiring is in [`examples/basic`](examples/basic/basic.go), and the public API
snippets are compiled as Go examples in [`examples_test.go`](examples_test.go).

With the default paths, the module serves:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/docs` | self-hosted Swagger UI |
| `GET` | `/docs/` | permanent redirect to `/docs` |
| `GET` | `/docs/openapi.json` | OpenAPI 3.1.0 JSON |
| `GET` | `/docs/swagger-initializer.js` | CSP-safe UI initializer |
| `GET` | `/docs/assets/5.32.14/*` | fixed, allowlisted UI assets |

The UI and specification routes are not authenticated by this package. Attach
application middleware through `UIMiddleware` and `SpecMiddleware` before
enabling them outside a trusted development environment.

## Documentation model

By default, only routes passed to `Documenter.Route` are published. Arandu
Swagger reliably infers the concrete HTTP method, full path, route name,
path-parameter names, public constraints, module, domain metadata, and route
deprecation metadata. It does not inspect handlers, execute requests, parse
source code, or infer payloads and authorization.

The route name becomes `operationId` unless overridden. Every documented
operation must declare at least one response with a non-empty description.
Path parameters are always required in OpenAPI; missing declarations are added
conservatively from the route metadata.

```go
docs.Route(route).
	OperationID("users.store").
	QueryParameter("include", jsonschema.String().Enum("profile", "permissions"),
		swagger.Description("Relations to include."),
	).
	RequestBody(
		swagger.JSONRef("CreateUser").Required().
			Example(map[string]any{"name": "Ada"}),
	).
	Response(http.StatusCreated, "User created", swagger.JSONRef("User")).
	Response(http.StatusUnprocessableEntity, "Validation failed", swagger.JSONRef("Problem")).
	Security("bearerAuth")
```

See the focused guides for the complete surface:

- [Installation](docs/installation.md)
- [Configuration](docs/configuration.md)
- [Explicit bootstrap wiring](docs/bootstrap.md)
- [Documenting routes](docs/routes.md)
- [Parameters](docs/parameters.md)
- [Request bodies](docs/request-bodies.md)
- [Responses](docs/responses.md)
- [Schemas and components](docs/schemas-components.md)
- [Authentication](docs/authentication.md)
- [Filters and exclusions](docs/filters.md)
- [Production, cache, and middleware](docs/production.md)
- [Embedded assets and CSP](docs/assets-csp.md)
- [Static generation and diagnostics](docs/generation.md)
- [Supported and unsupported behavior](docs/compatibility.md)
- [Adapting from the unidentified Bolt Swagger reference](docs/bolt-swagger.md)
- [Architecture](docs/architecture.md)

## Programmatic generation

Generation does not require an HTTP server. The low-level API accepts the
public route table, an instance-owned registry, and typed configuration:

```go
registry := swagger.NewRegistry()
registry.Route(route).Response(http.StatusNoContent, "Service is healthy")

data, err := swagger.GenerateJSON(router.Routes(), registry, swagger.Config{
	Title:   "Example API",
	Version: "1.0.0",
})
```

The package returns bytes and never writes them. A calling application or CI
job decides whether and where to persist them. A runnable example is in
[`examples/static`](examples/static/main.go). There is no package CLI because
Arandu does not currently expose a package-command extension contract.

## Routing boundaries

| Route behavior | Result |
| --- | --- |
| Named single-method route | name is the automatic `operationId` |
| Unnamed route | no `operationId` unless one is set explicitly |
| `Match` with concrete methods | one operation per method; automatic IDs are method-qualified |
| `ANY` | excluded when undocumented; documented use requires explicit `Methods(...)` |
| Explicit `HEAD`, `OPTIONS`, or `TRACE` | represented |
| Implicit `HEAD` for a `GET` route | not invented |
| Fallback, Swagger-owned, or hidden route | always excluded |
| `/_arandu` route | excluded unless `IncludeInternal` is true |
| Optional `{id?}` or multi-segment `{path...}` | generation error |
| Unnormalized qualified binding such as `{post:slug}` | generation error until Hesape supplies the cleaned `{post}` pattern |
| Generic Go regex not portable to ECMAScript | generation error |
| Route domain | preserved as `x-arandu-domain`, not treated as an enforced server |

Constraints are preserved as string patterns when portable. Arandu Swagger
does not narrow a route's runtime contract by guessing integer, UUID, ULID, or
enum semantics from a pattern whose public metadata does not retain its source.

## Production defaults

- Keep `Enabled` environment-controlled. The zero configuration registers no
  endpoints.
- Set `CacheSpec: true` after routes and documentation are immutable. Registry
  changes and route-table metadata changes invalidate the serialized cache
  safely.
- Protect both UI and specification with application middleware. The package
  deliberately implements no authentication system.
- Leave `PersistAuthorization` false unless the risk of browser-side token
  persistence is accepted.
- Set `DisableTryItOut` when the documentation must remain read-only.
- Use `DisableUI: true` for JSON-only service and CI deployments.
- Use both `DisableUI: true` and `DisableSpec: true` for programmatic-only
  generation.

The dynamic HTML, initializer, and JSON responses use `Cache-Control: no-store`.
Version-pinned embedded assets use an immutable one-year cache policy; their
paths are not content-addressed. All served files are selected from a fixed
allowlist and carry `X-Content-Type-Options: nosniff`.

## Third-party material

The embedded Swagger UI distribution is Apache-2.0 licensed and pinned to
version 5.32.14. Its license, notice, source, and file hashes are recorded in
[`THIRD_PARTY.md`](THIRD_PARTY.md). No Bolt Swagger repository could be
identified in the supplied workspace, so it was treated only as an
unidentified design reference; no source from it was incorporated.

## Development

Run the same gates as CI with the enclosing Go workspace disabled:

```bash
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) before changing public behavior.

## License

Arandu Swagger is MIT licensed. See [LICENSE.md](LICENSE.md). Copyright Arandu.
