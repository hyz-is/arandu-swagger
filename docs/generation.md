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

## CI and static export

The repository includes a runnable generator:

```bash
go run ./examples/static > openapi.json
```

The shell performs the file write. Arandu Swagger only returns or prints bytes
and therefore does not claim runtime filesystem capability. There is no
`aru swagger:generate` command or package-specific CLI: Arandu does not expose
a package command-extension contract yet.

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
