# Production, cache, and middleware

Documentation is an operational surface. Treat the OpenAPI JSON as sensitive
even when Try it out is disabled: paths, parameters, schemas, and security
requirements can reveal capabilities an anonymous visitor should not know.

## Enable deliberately

```go
docs, err := swagger.New(swagger.Config{
	Enabled: cfg.App.Env == config.EnvDev,
	Title:   "Example API",
	Version: buildVersion,
})
```

The zero configuration is disabled. An enabled configuration requires `Title`
and `Version`. Use route filters and `Hidden` in addition to endpoint access
control; middleware controls who can fetch a document, while filters control
what that document contains.

## Application-owned middleware

`UIMiddleware` applies to the HTML, trailing-slash redirect, initializer, and
every embedded asset. `SpecMiddleware` applies only to OpenAPI JSON.

```go
config.UIMiddleware = []swagger.Middleware{requireDocumentationAccess}
config.SpecMiddleware = []swagger.Middleware{requireDocumentationAccess}
```

Middleware uses the ordinary `func(http.Handler) http.Handler` shape and runs
through the Arandu router. The two lists are separate so an application can,
for example, disable the UI publicly while allowing an authenticated CI client
to retrieve JSON. Arandu Swagger does not implement users, sessions, roles,
passwords, or API keys for its own routes.

## Server-side generation cache

Set `CacheSpec: true` when the route table and docs are immutable after boot.
The module caches serialized bytes, not a mutable document. Builder and
component mutations advance the instance registry revision; changes to public
route metadata change a separate route-table fingerprint. Either invalidates
the cache. Concurrent readers and supported mutations are synchronized and
covered by the race-enabled test suite.

```go
config.CacheSpec = cfg.App.Env == config.EnvProd
```

With `CacheSpec: false`, every request reads the current public route table,
snapshots the registry, validates, and serializes again. This is useful while
developing documentation. An Arandu application normally never registers or
mutates routes after boot. The cache detects route registration and public
metadata changes, but it cannot observe mutable state captured by a custom
filter predicate. Leave caching off if predicate results may change at runtime.

`CacheSpec` is not an HTTP cache toggle. Dynamic HTML, initializer JavaScript,
and specification JSON all use `Cache-Control: no-store`. Only versioned static
assets are publicly immutable.

## Interactive UI controls

- `DisableTryItOut: true` configures an empty `supportedSubmitMethods` list.
- `PersistAuthorization: false` is the safe default; true lets Swagger UI keep
  authorization state in the browser.
- `DisableUI: true` serves only JSON.
- `DisableUI: true` with `DisableSpec: true` leaves only programmatic
  `Generate` and `GenerateJSON`.

Never place real credentials or captured production data in examples.

## Failure behavior

A generation failure returns HTTP 500 with a generic JSON message. The full
error is available to the caller of `Generate`/`GenerateJSON` and through the
module's latest diagnostic. This avoids exposing route and schema details in an
error response while preserving an actionable operator message.

See [static generation and diagnostics](generation.md) for CI usage.
