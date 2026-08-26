# Arandu Swagger Architecture

## Purpose

Arandu Swagger is an explicit, instance-owned OpenAPI 3.1 integration for an
Arandu application. It documents routes beside their registration, generates a
typed document from the application's existing route table, and serves a local
Swagger UI without adding a second router, a second JSON Schema library, or a
runtime asset toolchain.

This document records decisions supported by the local Arandu sources audited
before implementation. Source references use the form
`arandu-io/<repository>/<file>:<lines>`.

## Audited local sources

| Repository | Primary surfaces | Architectural conclusion |
| --- | --- | --- |
| `framework` | `foundation.Module`, optional module contracts, `foundation.Application`, `framework/http` | Modules are explicitly registered, routes are registered during `Boot`, and the framework route type is the Hesape route type. |
| `hesape` | `routing`, `http`, `jsonschema`, embedded view assets | The route table and JSON Schema builders are the sources of truth. HTTP responses and static assets can use the standard library directly. |
| `package-skeleton` | Module/config construction, package gates, external test layout | The package uses `New(Config)`, typed configuration, English public documentation, and `tests/Unit` plus `tests/Feature`. |
| `aru` | Command dispatch and capability audit | There is no package command plug-in contract. Capabilities are inferred from concrete calls, not intent. |
| `kyse` | Component and asset model | Kyse is a compiled Go component library, not a replacement runtime for Swagger UI. |
| `joaju` | Embedded self-origin JavaScript client | Its asset handler is a useful local pattern; WebSockets are unnecessary for the first release. |

The configured package targets `github.com/arandu-io/swagger` and depends only
on the Arandu-owned `framework` and `hesape` modules. The compatibility floor is
the pair of versions pinned in this repository's `go.mod:5-8`; implementation
and tests must continue to compile against that floor.

## Framework contracts

### Module boundary

The stable framework contract is:

```go
type Module interface {
	Name() string
	Routes(r *http.Router)
}
```

Source: `arandu-io/framework/foundation/module.go:19-49`.

The router in this signature is the `framework/http` envelope, not a second
route implementation. `framework/http.Route` is an alias of
`hesape/routing.Route`, and every subrouter writes directly to the same Hesape
route table. Sources:

- `arandu-io/framework/http/router.go:23-60`
- `arandu-io/framework/http/router.go:86-105`
- `arandu-io/framework/http/router.go:139-146`
- `arandu-io/hesape/routing/router.go:111-138`
- `arandu-io/hesape/routing/router.go:226-230`

This permits the public documentation boundary to stay small:

```go
type Documenter interface {
	Route(route *http.Route) *OperationBuilder
}
```

The concrete registry belongs to the Swagger module instance. No metadata is
stored in route defaults, the route action map, an `init` function, or a package
global.

### Optional interfaces

The framework aliases the optional lifecycle contracts from Hesape: `Bootable`,
`Background`, `Closable`, `Diagnostic`, `Schedulable`, `Migratable`, `Health`,
and `ReloadTagger`; it declares `RendererProvider` locally. Sources:

- `arandu-io/framework/foundation/module.go:51-104`
- `arandu-io/framework/foundation/module.go:168-198`
- `arandu-io/hesape/foundation/module.go:52-96`
- `arandu-io/hesape/foundation/module.go:143-163`

None is a post-boot finalization hook. Arandu Swagger implements
`foundation.Module` and does not implement `foundation.Migratable`. It may
implement `foundation.Diagnostic` only to report a previously observed
generation failure; diagnostics do not drive generation.

## Lifecycle and generation time

`Application.Boot` performs these operations in order:

1. It rejects a second boot.
2. It discovers a renderer before registering routes.
3. For each module in registration order, it validates the module name.
4. It invokes `Boot(ctx)` when the module is `Bootable`.
5. It immediately invokes `Routes` for that same module.
6. After every module has registered routes, it mounts framework-internal routes.
7. It marks the application booted.

Source: `arandu-io/framework/foundation/application.go:178-230`. Internal routes
are mounted in `arandu-io/framework/foundation/application.go:276-314`, and
`Application.Routes` is empty before boot in
`arandu-io/framework/foundation/application.go:587-589`.

Consequently, `New` and `Bootable.Boot` are both too early to construct the
final OpenAPI document. Registering Swagger last is the documented wiring
convention, but correctness must not depend on it.

The Swagger module records the router it receives in `Routes`, registers its own
handlers, and generates lazily when the specification is first requested after
application boot. At that point the shared table includes earlier modules,
later modules, internal routes, and the Swagger routes themselves. Filtering
removes surfaces that must not be published.

The generation pipeline is deliberately separated:

```text
route registration -> instance registry
                         |
first spec request -> route-table snapshot + registry snapshot
                         |
                    build -> validate -> marshal
                         |
                    cache or return
```

A caller may also invoke the public generation API with a route slice without
starting an HTTP server. This is the path for tests, static export by application
code, CI checks, and future compatibility tooling.

## Registry, snapshots, and cache

The registry is owned by one module instance and keyed by route pointer. Pointer
identity is stable because `framework/http.Route` is the Hesape route itself.
This also lets two applications or two tests build independent documentation
graphs in one process.

The registry and component collections are protected by a mutex. Fluent builder
methods mutate through that owner rather than exposing mutable maps. Generation
copies both route metadata and documentation metadata into an immutable snapshot
before validation and serialization.

The route collection itself uses an `RWMutex`, and `Routes.All` returns a copy
of the registered slice. Sources:

- `arandu-io/hesape/routing/name.go:147-159`
- `arandu-io/hesape/routing/name.go:225-251`

Individual routes remain mutable fluent values without their own lock. The
supported lifecycle is therefore: register routes and documentation during
boot, then serve. Mutating a route or a native schema concurrently with
generation is outside the contract.

Production cache stores the validated serialized result, including any
generation error, so concurrent first requests perform one build. Its key
combines the registry revision with a deterministic fingerprint of public route
metadata. Development regeneration takes a fresh snapshot for each request.
Cache policy is explicit configuration, not an environment lookup hidden inside
the package.

The trade-off of lazy generation is that a document error is observed at the
generation boundary rather than by `Application.Boot`. The framework exposes no
post-boot hook that could fail startup after the complete route table exists.
The public generation function allows an application or CI job to validate the
same document eagerly when that behavior is required.

## OpenAPI document model

The canonical output is OpenAPI 3.1 JSON. The model uses typed Go structs for
fixed OpenAPI objects and maps only where the specification defines dynamic
keys, including paths, response status codes, component names, media types,
security scheme names, and `x-*` extensions.

Validation occurs before serialization and attaches failures to the relevant
route, operation, parameter, response, component, or reference. It checks at
least:

- required top-level information;
- unique operation identifiers;
- one operation per representable method and path;
- path-parameter agreement and `required: true`;
- non-empty response descriptions;
- component conflicts;
- local reference targets;
- security requirement targets;
- supported HTTP methods and route shapes.

Struct field order and deterministic map-key encoding make repeated JSON output
stable. Component equality is semantic JSON equality, not raw byte equality,
because property order is not significant to JSON Schema.

YAML is not part of the first release. Adding it would require a justified
serializer dependency and another deterministic-output contract.

## Hesape JSON Schema adapter

`hesape/jsonschema.Type` is a closed interface implemented by Hesape's object,
string, integer, number, boolean, array, union, and `anyOf` types. It also
implements `json.Marshaler`. Sources:

- `arandu-io/hesape/jsonschema/type.go:9-20`
- `arandu-io/hesape/jsonschema/type.go:80-152`
- `arandu-io/hesape/jsonschema/type.go:160-225`
- `arandu-io/hesape/jsonschema/type.go:227-483`
- `arandu-io/hesape/jsonschema/marshal.go:71-194`

OpenAPI media types and component schemas therefore accept a native
`jsonschema.Type` directly. They do not accept `jsonschema.Document`, because a
Document adds standalone `$schema` and `$id` fields around its root. Source:
`arandu-io/hesape/jsonschema/document.go:8-83`.

The only additional schema abstraction is a thin OpenAPI wrapper that holds
either immutable JSON snapshotted from a native Hesape type or a URI reference.
A reference cannot implement `jsonschema.Type` outside Hesape because the
interface intentionally contains an unexported method. This wrapper does not
reproduce primitive schema builders.

Native schemas are mutable pointer builders, so the package serializes and
validates them at the API boundary. Later mutations of the original builder do
not alter the stored snapshot or race with generation. Sources:
`arandu-io/hesape/jsonschema/type.go:33-78` and
`arandu-io/hesape/jsonschema/type.go:170-225`.

Features beyond the native type set, such as arbitrary composition keywords or
schema-valued additional properties, are not silently discarded. They remain
unsupported until represented faithfully, and generation returns an actionable
error when a requested feature cannot be encoded.

## Route interpretation

The public route metadata includes method, full pattern, module, name, prefix,
domain, path parameters, constraints, fallback status, and deprecation. Sources:

- `arandu-io/hesape/routing/name.go:14-41`
- `arandu-io/hesape/routing/route.go:123-235`
- `arandu-io/hesape/routing/route.go:678-690`
- `arandu-io/hesape/routing/route.go:719-812`
- `arandu-io/hesape/routing/deprecation.go:36-69`

The following rules preserve that runtime contract.

| Route shape | OpenAPI behavior |
| --- | --- |
| Explicit GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS, or TRACE | Generate the corresponding operation when included. |
| `Match` with several methods | Generate one operation per method, using the actual table row for each method. Operation identifiers must remain unique per operation. |
| GET that also answers HEAD through `net/http` | Document GET only unless HEAD was registered explicitly. The route table does not declare an implicit HEAD operation. |
| `ANY` | Exclude by default. If explicitly selected, return an actionable unsupported-method error rather than expanding it into invented operations. |
| Custom method such as CONNECT or PROPFIND | Return an unsupported-method error when included. |
| Fallback route | Exclude unconditionally through `Route.IsFallback`. |
| Optional path parameter such as `{id?}` | Reject. OpenAPI requires path parameters to be required, and the normal Go `ServeMux` registration does not accept `?` in a wildcard name. |
| Multi-segment wildcard such as `{path...}` | Reject. OpenAPI path templates cannot express the route's multi-segment semantics. |
| Raw qualified binding such as `{post:slug}` | Reject. Hesape declares a normalizer to `{post}`, but its current registration path does not call it and the Go mux panics on the raw wildcard. Once normalized upstream, the ordinary `{post}` path needs no special Swagger behavior. |
| Root anchor `{$}` | Remove the anchor from the OpenAPI path and never expose `$` as a parameter. |
| Unnamed route | Allow explicit documentation; derive no operation identifier from a missing name. |
| Duplicate method and path | Return a conflict error when encountered in an externally supplied route slice. Normal router registration already rejects the same conflict. |

The implicit GET-to-HEAD behavior and wildcard-name restrictions come from the
Go 1.26 standard library at `net/http/routing_tree.go:141` and
`net/http/pattern.go:79-83,158-177`. Hesape exposes optional-parameter metadata
in `arandu-io/hesape/routing/route_scheme.go:44-73`. The special root anchor is
handled as a non-parameter by Hesape URL generation in
`arandu-io/hesape/routing/name.go:182-190`, while the generic parameter scanner
only strips `...` and `?` in `arandu-io/hesape/routing/route.go:825-842`; the
Swagger normalization therefore handles `{$}` explicitly.

`Match` registers one table row per method, returns the first route, and links
the remaining rows as siblings. `Methods` on the returned first row collects
them. Sources:

- `arandu-io/hesape/routing/router.go:165-192`
- `arandu-io/hesape/routing/route.go:123-134`
- `arandu-io/hesape/routing/name.go:132-145`

Some fluent metadata propagates to siblings and some does not. `Name` propagates
in `arandu-io/hesape/routing/name.go:132-145`, while direct `Where` and `Domain`
mutate only the receiver in `arandu-io/hesape/routing/route.go:159-167` and
`arandu-io/hesape/routing/route.go:719-778`. The generator therefore indexes the
route-table snapshot by method and pattern, expands the documented registration,
and reads constraints and domain from the concrete row for each method. It marks
consumed method/path pairs to avoid generating sibling rows twice.

### Parameters and constraints

Every discovered path parameter is emitted automatically as a required OpenAPI
path parameter. Explicit documentation may add a description or a schema but
may not name a parameter absent from the route.

Hesape enforces route constraints with Go regular expressions and
`MatchString`. Its convenience patterns are not anchored, `WhereIn` joins raw
values with `|`, and `GetWheres` exposes only the final expression, not which
helper created it. Sources:

- `arandu-io/hesape/routing/route.go:50-76`
- `arandu-io/hesape/routing/route.go:719-778`
- `arandu-io/hesape/routing/route.go:781-812`

Consequently, inferring `integer`, `uuid`, or `enum` from expression equality
would narrow the documented contract. The safe automatic representation is a
string schema carrying the original pattern only when that Go expression is
compatible with the JSON Schema regular-expression dialect. An incompatible
generic expression causes a clear error. A caller may provide an explicit
schema when it intentionally declares a narrower API contract.

### Domain metadata

`Route.Domain` is metadata used by URL generation and route matching helpers;
the underlying `http.ServeMux` does not dispatch by host. Sources:
`arandu-io/hesape/routing/name.go:68-70` and
`arandu-io/hesape/routing/route.go:159-175`.

Domain is therefore valid as a route filter and as explicit documentation input,
but it is not inferred as an enforced security boundary or host-specific server.

## Inclusion and filtering

Only routes present in the instance registry are included by default.
`IncludeUndocumented` is an explicit opt-in for basic route inventory.

Filtering may use full path prefix, route name, module, method, domain, or a
caller-provided predicate. Independently of those filters, the default safety
rules remove:

- framework routes under `/_arandu/`;
- fallback routes;
- the Swagger UI, specification, and asset routes;
- operations explicitly marked hidden or ignored.

The framework tags routes with the owning module through `ForModule`, so deriving
a tag from a non-empty module field is unambiguous. Source:
`arandu-io/framework/http/router.go:99-105`.

## HTTP endpoints and security

Documentation is disabled unless configuration explicitly enables it. UI and
public specification serving can be enabled independently, while the in-process
generation API remains available. Authentication and authorization are supplied
as ordinary application middleware; Swagger implements no separate identity
system.

Swagger endpoints must not use the `/_arandu/` namespace. The framework removes
the application's middleware pipeline for every request under that prefix,
including recovery, security headers, rate limiting, and CSRF protection.
Source: `arandu-io/framework/foundation/application.go:405-453`.

`hesape/http.Context.JSON` is not used for the OpenAPI document because it takes
a resource and wraps the response under `data`. The specification handler writes
the already validated JSON bytes directly. Source:
`arandu-io/hesape/http/context.go:272-335`.

## Swagger UI assets and CSP

Swagger UI is a vendored third-party distribution served from the application
origin. The binary embeds a fixed, reviewed asset set. It performs no download
at boot or request time and exposes no embedded directory through a generic file
server.

Each asset has a version-pinned URL, an integrity hash recorded in
`THIRD_PARTY.md`, an explicit content type, `X-Content-Type-Options: nosniff`,
and immutable long-lived caching. Changing asset bytes therefore requires a new
vendored version and URL. The local Hesape asset handler establishes this pattern in
`arandu-io/hesape/view/assets.go:14-26` and
`arandu-io/hesape/view/assets.go:217-260`. Joaju independently uses the same
self-origin, fixed-name approach in `arandu-io/joaju/client/client.go:12-38` and
`arandu-io/joaju/client/client.go:116-159`.

The framework CSP allows scripts, styles, fonts, and connections from `self`.
The package keeps scripts and style blocks on that origin. Swagger UI 5.x still
uses runtime style attributes, so the served policy adds the narrow CSP3
`style-src-attr 'unsafe-inline'` exception while keeping `script-src` strict. Source:
`arandu-io/hesape/http/middleware/headers.go:9-40`. The UI shell therefore loads
an external initializer script and stylesheet; it contains no inline script or
style. Try-it-out against a non-self server may require an application-specific
CSP and CORS decision and is not silently enabled by this package.

Kyse remains available for an application-owned shell, but rewriting the
third-party Swagger UI as Kyse components would duplicate an external runtime
without improving the OpenAPI integration. The Kyse repository describes its
surface as compiled Go functions returning `template.HTML` and keeps stylesheet
rules in the application. Source: `arandu-io/kyse/README.md:16-35`,
`arandu-io/kyse/README.md:67-72`, and `arandu-io/kyse/README.md:123-147`.

Joaju is not a dependency of the first release. If live document reload is
introduced later, it should use the existing Joaju transport rather than create
another WebSocket server.

## Aru integration and capabilities

Aru's command list is compiled into the CLI, while commands it does not know are
delegated to the current application's binary. There is no contract by which an
imported package registers a new Aru command. Sources:

- `arandu-io/aru/commands.go:18-31`
- `arandu-io/aru/main.go:50-69`
- `arandu-io/aru/delegate.go:18-71`

The first release exposes programmatic generation and does not add a parallel
CLI or modify Aru. An application may later wire its own command around that API,
and a future official integration requires an official extension contract.

Aru audits capabilities from AST calls. Server-side `net/http` use is not
outbound network access; filesystem, process execution, outbound HTTP, and any
receiver method named `Migrations` are detected separately. Source:
`arandu-io/aru/internal/doctor/rules.go:1476-1604`.

With embedded assets and no runtime download, file write, process, database, or
migration method, the package declares:

```toml
network = false
filesystem = false
exec = false
migrations = false
```

`aru doctor` runs against an application that installs the package, not against
this library repository. The package itself is verified by its build, vet, race,
manifest, and integration tests.

## Third-party Swagger references

Architecture and legal provenance are separate records. A reported project name
or license is not enough to establish that code may be copied, and this document
does not invent authors, copyright holders, license terms, or reused files.

The detailed legal audit of every Swagger-related reference belongs in
`THIRD_PARTY.md`. That file must identify the exact source and version, the
license and required notices found in the source distribution, and every file or
substantial fragment reused. It must also record the exact vendored Swagger UI
assets and notices. If no code is reused from a reference, it should say so
plainly.

## Known trade-offs and limitations

- Lazy generation is the only lifecycle point that can observe the complete
  route table without changing the framework. Startup-time validation remains
  available through an explicit generation call by the application.
- `framework/http` is a compatibility bridge scheduled for removal at framework
  v1 (`arandu-io/framework/http/doc.go:1-21`), but the current
  `foundation.Module` contract still requires its Router. The Route alias keeps
  the documentation model on the Hesape type.
- The framework envelope has no multi-method or `ANY` convenience method, though
  `Action` can register any one method. The generator still supports Hesape route
  metadata supplied directly.
- Match registrations require per-method operation identity and concrete-row
  metadata; treating the first row as all methods would be incorrect.
- Route-constraint helper provenance is not public, and current expressions are
  unanchored. Automatic semantic type inference is intentionally conservative.
- Optional parameters are not valid registered Go wildcard names, and OpenAPI
  cannot express optional path parameters directly.
- Multi-segment wildcards have no faithful OpenAPI path-template equivalent.
- Domain metadata is not host dispatch and cannot be documented as enforcement.
- Native Hesape schemas do not expose every JSON Schema/OpenAPI keyword. Missing
  features fail explicitly instead of degrading to weaker schemas.
- The first release emits JSON only and has no Aru plug-in, live reload, database,
  migrations, or runtime asset fetch.

## Verification rules

The package follows the configured package repository gates and test layout from
`arandu-io/package-skeleton/AGENTS.md:66-105` and
`arandu-io/package-skeleton/AGENTS.md:176-194`:

```sh
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

Unit tests cover the typed model, registry, inference, validation, determinism,
and cache. Feature tests mount the module through `framework/http`, drive the
UI/spec/assets with `httptest`, and prove boot-order independence, middleware
compatibility, instance isolation, and the absence of migrations.
