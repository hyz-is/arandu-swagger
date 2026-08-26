---
name: swagger-module
description: Change the implementation of the Arandu Swagger module, including configuration, route interpretation, OpenAPI generation, registry builders, cache, HTTP handlers, or embedded UI assets.
license: MIT
---

# Changing the Swagger module

Preserve these boundaries before adding behavior:

- `Module` implements `foundation.Module` and `foundation.Diagnostic`, never
  `foundation.Migratable`.
- `New(Config)` validates and snapshots configuration. It performs no boot
  work, I/O, route scan, or global registration.
- The registry belongs to the module instance and is mutex-protected.
- `Routes` captures the shared Arandu router and registers only the configured
  UI/specification handlers.
- Generation reads `Router.Routes()` lazily, snapshots registry state, builds a
  typed OpenAPI document, validates it, then serializes it.
- Handlers never infer bodies, responses, authorization, or secrets.

## File responsibilities

| File | Responsibility |
| --- | --- |
| `config.go` | typed settings, defaults, validation, immutable snapshot |
| `registry.go` | route builders, component registration, deep copies |
| `schema.go` | thin immutable adapter over `hesape/jsonschema.Type` |
| `openapi.go`, `security.go` | typed OpenAPI 3.1 model |
| `generator.go` | route binding, filtering, inference, validation, JSON |
| `module.go` | Arandu lifecycle, endpoints, cache, diagnostics |
| `internal/ui` | allowlisted embedded browser distribution and shell |

## Route interpretation

Use public Hesape metadata only: `Routes`, `Methods`, `URI`, `ParameterNames`,
`GetWheres`, `GetName`, `GetDomain`, `IsFallback`, deprecation, and module
metadata. Do not store documentation in route defaults or action maps.

`Match` creates concrete sibling rows. Bind documentation from the returned
route to those rows, then read each concrete row's metadata. Never invent HEAD
from GET. Never expand `ANY` without explicit methods. Reject optional and
multi-segment path parameters rather than emitting invalid OpenAPI. Reject a raw
qualified binding such as `{post:slug}`: Hesape must first normalize it to the
runtime `{post}` pattern while retaining the binding field as metadata.

## Configuration and cache changes

For every new config field, add a doc comment, validation, a default only when
zero has a documented meaning, deep-copy logic when it contains mutable state,
and unit/feature coverage. Do not read environment variables inside the module.

Cache keys must cover both registry revision and route-table state. Cache only
validated serialized bytes or a stable generation error, and keep the first
concurrent generation race-free.

## Assets and capabilities

UI assets are fixed-version, allowlisted `go:embed` files. Keep scripts and
styles same-origin, retain strict `script-src`, scope Swagger UI's required
inline-style exception to `style-src-attr`, serve correct MIME types with
`nosniff`, and use immutable caching only for versioned URLs. Update
`THIRD_PARTY.md`, retained notices, integrity hashes, tests, and the version
constant together.

Do not add filesystem, outbound network, process, or migration behavior without
updating `arandu.mod.toml` in the same change. Server-side `net/http` handlers do
not constitute outbound network access.

## Verification

Run all four gates with `GOWORK=off`:

```sh
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race -count=1 ./...
```

`aru doctor` targets an application, not this package repository.
