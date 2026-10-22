# Static generation and diagnostics

Generation is separated from HTTP delivery. It reads a route-table snapshot
and an instance registry, builds and validates a typed OpenAPI document, then
serializes deterministic JSON.

## Low-level generation

Use `NewRegistry` when a test or tool owns the route table directly:

```go
registry := swagger.NewRegistry()
registry.Route(route).Response(http.StatusNoContent, "Service is healthy")

document, err := swagger.Generate(router.Routes(), registry, swagger.Config{
	Title:   "Example API",
	Version: "1.0.0",
})
if err != nil {
	return err
}
```

`Generate` returns the typed `*swagger.Document`. This is the place to apply an
application-specific advanced OpenAPI field before calling `json.Marshal`.
`GenerateJSON` performs both steps and returns deterministic JSON bytes.

`Generate` describes the application as it is composed: each operation carries
the module its route is registered under as `x-arandu-module`, and an operation
with no tags of its own is tagged with that module. This is the document the
module serves. `GenerateContract` takes the same arguments and returns the
document `openapi:export` commits:

```go
document, err := swagger.GenerateContract(router.Routes(), registry, config)
```

It leaves the module out -- no `x-arandu-module`, and an untagged operation
stays untagged -- because which module registers a route is how the
application is composed, not part of its HTTP surface. `x-arandu-route-name`,
`x-arandu-domain` and the deprecation extensions stay. When
`Config.ContractServers` is set, it is the document's `servers` in place of
`Config.Servers`.

Neither function starts a server. `Enabled` controls module endpoints, not the
low-level generator; generation itself always requires a non-empty title and
version.

## Generation through the module

After every route has been registered and `Module.Routes` has captured the
shared router:

```go
document, err := docs.Generate()
data, err := docs.GenerateJSON()
```

Do not call these before the application has booted. The error will explain
that `Routes` must be called first. The HTTP specification handler follows the
same path lazily on its first post-boot request.

With `CacheSpec` enabled, `Module.GenerateJSON` and the HTTP handler share a
serialized cache keyed by both registry revision and a fingerprint of public
route metadata. `Module.Generate` always returns a newly built typed document.

## Committed contracts: openapi:export and aru openapi:check

`swagger.Module` implements `foundation.Commandable`, so the application's
binary answers `openapi:export` once the module is registered:

```bash
aru openapi:export          # writes docs/contracts/api.openapi.json
aru openapi:check           # runs openapi:export --check, writes nothing
```

`openapi:export` generates every document the module exports from the route
table and writes each to `<ContractDir>/<ContractName>.openapi.json`
(`docs/contracts/api.openapi.json` by default), indented by two spaces and
ending in a newline. Each document is generated with `GenerateContract`, so it
carries no `x-arandu-module` and no tag taken from a module name, and lists
`Config.ContractServers` when that is set. The same route table always writes
the same bytes, whichever modules register its routes, and no
file is written unless every document generates. With `--check` nothing is
written: the command exits 1 naming each committed document that is missing or
differs from the generated one. Put `aru openapi:check` in CI beside the tests.

A route table that answers more than one contract -- a browser workspace and a
public API, for example -- declares the others on the same module, each with
its own registry and configuration:

```go
api := swagger.NewRegistry()
// ... document the public API routes on api ...
if err := docs.Contract(api, swagger.Config{
	Title:        "Public API",
	Version:      "1.0.0",
	ContractName: "public",
	Filter:       swagger.RouteFilter{IncludePrefixes: []string{"/api/"}},
}); err != nil {
	return App{}, err
}
```

The module's own document is exported whenever its configuration names a
`Title`, also while `Enabled` is false: `Enabled` decides whether the endpoints
are served, so documentation served only in development is still exported and
checked in CI. The zero configuration exports nothing and declares no command.

A contract can name the production server its SDKs call while the served
document names none, so that Swagger UI's "Try it out" sends requests to the
origin the page was loaded from and a development instance never calls
production:

```go
docs, err := swagger.New(swagger.Config{
	Enabled:         cfg.App.Env == config.EnvDev,
	Title:           "Example API",
	Version:         "1.0.0",
	ContractServers: []swagger.Server{{URL: "https://api.example.com"}},
})
```

An application keeps no `cmd/openapi` program: a contract no `swagger.Module`
produces is a command of the application in `app/Console/Commands`, which
generates it with `GenerateContract` so its bytes match what `openapi:export`
would write.

The runnable `examples/static` generator still prints a document for a tool
that wants the bytes on standard output:

```bash
go run ./examples/static > openapi.json
```

A CI test can compare the returned bytes with a reviewed artifact or feed them
to an external compatibility checker. Deterministic serialization makes a
semantic change visible without map-order noise.

## Diagnostics

`swagger.Module` implements `foundation.Diagnostic`. It remembers the most
recent error encountered by `Generate`, `GenerateJSON`, or the HTTP
specification handler:

```go
hints := docs.Diagnose(context.Background())
for _, hint := range hints {
	log.Print(hint)
}
```

A successful later generation clears the diagnostic. HTTP clients receive only
a generic 500 JSON response, so internal route and schema details are not
leaked.

Generation errors identify the relevant route, method, operation, component,
or config field. They cover, among other cases:

- missing title or version;
- duplicate `operationId` or method/path pair;
- an explicit path parameter absent from the route;
- missing response or empty response description;
- invalid schema snapshots and conflicting component registrations;
- missing local schema, response, parameter, or security references;
- unsupported methods, `ANY` without a concrete decision, optional path
  parameters, and multi-segment wildcards;
- route patterns not portable to ECMAScript;
- malformed servers, URLs, licenses, security schemes, or OAuth flows;
- values that cannot be serialized as JSON.

Builder calls remain fluent, so builder errors are accumulated in the registry
and returned at generation with route context rather than being hidden or
panicking.
