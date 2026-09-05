# Adapting from the unidentified Bolt Swagger reference

A project named “Bolt Swagger” informed the design of this package. No
verifiable upstream for it was found, so its license, authors, and copyright
notices are unknown, and a package cannot carry an attribution it cannot state.

Nothing from it was copied — no file, no code fragment. It is a design
reference and nothing more, which is why this package needs no notice for it.
[`THIRD_PARTY.md`](../THIRD_PARTY.md) records that separately from the vendored
Swagger UI distribution, which is Apache-2.0 and does carry its notice.

This guide is consequently an adaptation checklist for applications that use a
tool under that name, not a claim of API compatibility or a source migration.

## Replace hidden discovery with explicit composition

Construct one `swagger.Module` in `bootstrap/app.go`, pass its small
`swagger.Documenter` interface to modules that declare routes, and register the
Swagger module with the application. There is no provider, annotation scanner,
global registry, or package `init` hook.

## Move operation metadata beside route declarations

Replace comments, annotations, or a central YAML path entry with a fluent draft
attached to the exact route pointer:

```go
route := r.Post("/api/users", handler).Name("users.store")
docs.Route(route).
	Summary("Create a user").
	RequestBody(swagger.JSONRef("CreateUser").Required()).
	Response(http.StatusCreated, "User created", swagger.JSONRef("User"))
```

Keep descriptions, request bodies, responses, examples, and security explicit.
Allow the Arandu route table to supply only reliable routing metadata.

## Replace schema reflection with Hesape schemas

Build schemas through `hesape/jsonschema` and register reusable components on
the module instance:

```go
if err := docs.Schema("User", userSchema); err != nil {
	return err
}
```

Do not introduce a second schema DSL or runtime DTO reflection layer merely to
preserve another integration's syntax.

## Replace global definitions with instance components

Register schemas, parameters, responses, request bodies, headers, examples,
and security schemes through the module. Duplicate identical registration is
safe; different content under the same name is an error. This prevents tests
and multiple application instances from leaking documentation into one another.

## Replace generated files or boot snapshots with lazy generation

The HTTP endpoint reads the shared route table after application boot and
generates JSON lazily. Enable `CacheSpec` for an immutable production route
table. Use `Generate` or `GenerateJSON` directly for tests and static export;
there is no package CLI.

## Replace remote UI assets

Remove CDN tags and runtime downloads. Arandu Swagger serves a fixed, embedded,
same-origin Swagger UI distribution with an external initializer and strict
CSP. If the old integration called a cross-origin API directly from its UI,
choose an application proxy or deliberately configure CSP and CORS; the
built-in policy permits only same-origin connections.

## Review unsupported route shapes

Before switching an endpoint, check documented `ANY` routes, optional path
parameters, multi-segment wildcards, custom methods, and Go-only regex
constraints. Arandu Swagger returns errors for shapes it cannot represent
faithfully instead of emitting a misleading OpenAPI document. The complete
matrix is in [compatibility.md](compatibility.md).
