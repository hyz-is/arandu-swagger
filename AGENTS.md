# Working on Arandu Swagger

This repository is the official `github.com/arandu-io/swagger` package. It
derives an OpenAPI 3.1 document from the public Arandu route table, enriches it
through an instance-owned registry, and optionally serves embedded Swagger UI
assets. It is a library that an application registers explicitly in
`bootstrap/app.go`; there is no service provider, discovery hook, or global
registry.

Read the relevant procedure under `.agents/skills/` before changing code:

- `swagger-package` for public API, OpenAPI generation, and documentation.
- `swagger-module` for framework integration, routing, caching, and handlers.
- `swagger-security` for filtering, validation, CSP, and information exposure.
- `swagger-release` for dependency, asset, license, and release checks.

## Non-negotiable contracts

- `Module` implements `foundation.Module`, `foundation.Diagnostic`, and
  `Documenter`; it deliberately does not implement migrations or boot hooks.
- `New(Config)` validates wiring and snapshots every mutable configuration
  value. `Config{}` is a complete opt-out and registers no routes.
- Every module owns its own `Registry`. There is no package-level mutable state
  and no `init` side effect.
- Generation is lazy because `Routes` is the only framework callback that can
  capture the shared router, and later modules may still add routes.
- A cached specification is invalidated by both registry revision and a
  fingerprint of public route metadata.
- Fluent builders snapshot caller-owned slices, maps, and values. Concurrent
  generation and registration must remain race-free.
- Schemas use `github.com/arandu-io/hesape/jsonschema`; this package adapts and
  validates immutable snapshots instead of introducing a competing builder.
- Runtime code performs no filesystem access, outbound network request,
  process execution, or migration. Keep every permission in
  `arandu.mod.toml` false unless the architecture intentionally changes.

## Route interpretation

The Hesape route table is the routing source of truth. Preserve these rules:

- Exclude Swagger-owned, fallback, hidden, and undocumented routes by default.
- Treat `/_arandu` as internal unless `IncludeInternal` is enabled.
- Preserve portable constraints as JSON Schema string patterns; do not infer
  integer, UUID, enum, or other semantics that route metadata cannot prove.
- Reject optional parameters, multi-segment wildcards, non-portable regular
  expressions, ambiguous subtree patterns, raw qualified bindings that Hesape
  failed to normalize, and invalid path parameters with an actionable
  generation error.
- Expand concrete `Match` methods. Require explicit documentation methods for
  `ANY`. Do not invent implicit `HEAD` operations for `GET`.
- Preserve route domains only as `x-arandu-domain`; the current router records
  them as metadata but does not enforce host dispatch.

## HTTP and UI surface

The JSON endpoint, UI page, redirect, initializer, and embedded allowlisted
assets all run through ordinary Arandu routes. Keep Swagger UI version-pinned,
serve it locally, disable its remote validator, preserve the documented CSP,
and retain all upstream license and hash records in `THIRD_PARTY.md`.

Dynamic responses use `Cache-Control: no-store`; versioned static assets use an
immutable cache policy. Do not expose generation diagnostics in public HTTP
responses. Application middleware owns authentication and authorization for
the documentation endpoints.

## Source and tests

All code, identifiers, comments, errors, tests, and project documentation are
written in English. Every exported symbol needs a focused doc comment.

Production files at the module root are organized by responsibility:

```text
config.go           typed configuration and validation
module.go           framework integration, HTTP delivery, cache, diagnostics
module_registry.go  component registration methods on Module
openapi.go          typed OpenAPI 3.1 model
registry.go         instance registry and fluent operation builders
schema.go           Hesape JSON Schema adapter and snapshot validation
security.go         security-scheme helpers
generator.go        route interpretation, document generation, validation
internal/ui/        embedded Swagger UI page, initializer, and allowlisted assets
```

Tests live under `tests/Unit` and `tests/Feature` as external packages. Use an
internal `*_test.go` beside production code only when a behavior cannot be
observed through the public API. Examples must compile and must not contain real
credentials or captured production data.

## Required gates

Nothing is finished until all four commands exit zero from this repository:

```sh
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

Both `gofmt` filters are intentional because it ignores build tags. Keep
`GOWORK=off`: a neighboring workspace may not list this standalone module, and
CI resolves the dependency versions in `go.mod`.

Do not run `aru doctor` here. It validates complete applications containing
`go.mod`, `main.go`, and `arandu.toml`; this repository is an installable
library. Run it in an application that installs the package when integration
validation is required.
