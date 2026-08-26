# Contributing

Thank you for considering a contribution to Arandu Swagger.

## Before opening a pull request

Run the four repository gates with an enclosing Go workspace disabled:

```bash
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

The two `gofmt` filters are part of the repository contract: `gofmt` ignores
build tags, and `testdata` may intentionally contain invalid fixtures.

Tests belong under `tests/Unit` or `tests/Feature` and use external lowercase
packages (`unit_test`, `feature_test`). Use a root `*_internal_test.go` only
when behavior genuinely requires access to unexported code. Public examples
must compile; update `examples/` or `examples_test.go` with documentation API
changes.

## Design invariants

A change must preserve these properties:

1. The route table remains the public Arandu/Hesape route table. This package
   does not create another router or hide metadata in route action/default maps.
2. Documentation state belongs to a `Module` or `Registry` instance. There is
   no mutable package singleton, discovery hook, or registration `init`.
3. Only explicitly documented routes are included by default. Fallback,
   Swagger-owned, and hidden routes never become public through a filter.
4. Bodies, responses, examples, and security are explicit. The generator does
   not inspect handlers, execute requests, parse source, or invent contracts.
5. Hesape owns JSON Schema construction. Arandu Swagger snapshots native
   schemas and adds only the thin OpenAPI reference boundary.
6. An unsupported route or schema shape returns an actionable error instead of
   silently emitting a weaker or invalid OpenAPI document.
7. Generation is deterministic and race-safe, and lazy HTTP generation sees
   the complete post-boot route table.
8. Swagger UI remains embedded, fixed-version, allowlisted, same-origin, and
   correctly licensed. There is no CDN or runtime asset build.
9. `arandu.mod.toml` matches actual behavior. Adding outbound network access,
   runtime filesystem access, process execution, or migrations requires a
   deliberate capability review in the same change.

## Public API changes

Keep the public API small, typed, and route-adjacent. Prefer a new option or
typed OpenAPI object over `map[string]any`; maps are appropriate only where the
OpenAPI specification uses dynamic keys. Every exported symbol needs an
English Go doc comment.

Add focused tests for configuration validation, generated structure, error
context, ordering, cloning/snapshot semantics, caching, and concurrency as
applicable. A new builder call must not defer an untraceable error: attach it to
the registry so generation names the affected route.

Routing inference must preserve the runtime contract. Do not infer integer,
format, or enum from a regex unless the public route metadata proves how that
constraint was declared.

## Vendored Swagger UI

An asset update must be atomic:

1. select an upstream release tag;
2. vendor only the allowlisted distribution files;
3. retain upstream `LICENSE`, `NOTICE`, and minifier license text;
4. update the embedded version path and tests;
5. update source links and SHA-256 hashes in `THIRD_PARTY.md`;
6. verify CSP, MIME types, cache headers, and absence of remote resources.

Do not hand-edit minified vendor files.

## Style and commits

Write code, identifiers, comments, errors, tests, commit messages, and technical
documentation in English. Keep one logical change per commit and avoid mixing a
behavior change with unrelated formatting.

By contributing, you agree that your contribution is licensed under the MIT
license in [LICENSE.md](LICENSE.md).

## Security reports

Do not open a public issue for a vulnerability. Follow [SECURITY.md](SECURITY.md).
