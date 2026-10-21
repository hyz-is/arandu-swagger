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
ending in a newline. The same route table always writes the same bytes, and no
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

An application keeps no `cmd/openapi` program: a contract no `swagger.Module`
produces is a command of the application in `app/Console/Commands`.

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
