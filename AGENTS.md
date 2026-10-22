# Working on Arandu Swagger

This repository is the official `github.com/hyz-is/arandu-swagger` package. It
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

- `Module` implements `foundation.Module`, `foundation.Diagnostic`,
  `foundation.Commandable`, and `Documenter`; it deliberately does not
  implement migrations or boot hooks.
- `Commands` declares `openapi:export`, which writes every exported document
  to `<ContractDir>/<ContractName>.openapi.json` (`docs/contracts/api.openapi.json`
  by default) as indented JSON with a trailing newline, and with `--check`
  writes nothing and fails naming each committed file that is missing or
  differs. `aru openapi:check` runs it; no application keeps a `cmd/openapi`.
  The module's own document is exported when its `Title` is set, and further
  documents of the same route table are declared with `Module.Contract`.
- Exported documents come from `GenerateContract`: no `x-arandu-module` and no
  tag derived from the module name, and `Config.ContractServers` in place of
  `Servers` when set. The served document, `Module.Generate`/`GenerateJSON`
  and the package `Generate`/`GenerateJSON` keep the composition and
  `Servers`; do not let one leak into the other.
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
  `Models` derives a component from the Go type a payload already is, so the
  contract and the encoded value cannot drift; it is a derivation, not a
  builder, and its output passes the `SchemaFromJSON` validation.
- Runtime code performs no outbound network request, process execution, or
  migration, and serving requests touches no file. The filesystem belongs to
  the `openapi:export` command in `contracts.go` alone, which is why
  `arandu.mod.toml` declares `filesystem = true`; keep the other permissions
  false unless the architecture intentionally changes.

## Route interpretation

The Hesape route table is the routing source of truth. Preserve these rules:

- Exclude Swagger-owned, fallback, hidden, and undocumented routes by default.
- Treat `/_arandu` as internal unless `IncludeInternal` is enabled.
- Webhooks are not routes: they are registered as `Webhook` values from the
  application's event catalogue and written to the `webhooks` section only.
  Route filters do not apply to them, and their operationIds share the
  document's namespace with the routes'.
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
contracts.go        the openapi:export command and the documents it writes
openapi.go          typed OpenAPI 3.1 model
registry.go         instance registry and fluent operation builders
schema.go           Hesape JSON Schema adapter and snapshot validation
models.go           schema components derived from Go types
webhooks.go         the document's webhooks section, registered as typed values
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
