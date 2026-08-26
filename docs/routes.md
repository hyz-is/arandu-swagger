# Documenting routes

Create the Arandu route first, keep its returned pointer, and pass that pointer
to `Documenter.Route`. The documentation remains next to the route without
being stored in the route action map or defaults.

```go
route := r.Post("/api/users", http.HandlerFunc(m.store)).Name("users.store")

m.docs.Route(route).
	OperationID("users.store").
	Summary("Create a user").
	Description("Creates a user in the authenticated tenant.").
	Tags("Users").
	RequestBody(swagger.JSONRef("CreateUser").Required()).
	Response(http.StatusCreated, "User created", swagger.JSONRef("User")).
	Security("bearerAuth")
```

`Route` can be called again with the same pointer to continue the same draft.
Builders snapshot schemas, examples, component values, and extensions at their
registration boundary. Builder errors are retained and returned with route
context during generation.

## Operation identity

- `OperationID` sets one explicit identifier for a single-method route.
- Without it, the route name is used when available.
- An unnamed route has no `operationId` unless one is set explicitly.
- Duplicate identifiers are generation errors.
- A multi-method route gets method-qualified automatic identifiers, such as
  `search.get` and `search.post`.
- Use `OperationIDFor(http.MethodGet, "search.read")` to name one method of a
  multi-method route explicitly.

## Multiple and unusual methods

Hesape `Match` routes produce one OpenAPI operation per concrete method. A
documented `ANY` route is ambiguous and fails generation until the application
makes an explicit choice:

```go
docs.Route(anyRoute).
	Methods(http.MethodPost, http.MethodPut).
	OperationIDFor(http.MethodPost, "hook.create").
	OperationIDFor(http.MethodPut, "hook.replace").
	Response(http.StatusAccepted, "Hook accepted")
```

OpenAPI operations are supported for `GET`, `PUT`, `POST`, `DELETE`, `OPTIONS`,
`HEAD`, `PATCH`, and `TRACE`. An implicit `HEAD` accepted by Go for a `GET`
route is not invented in the document. A custom method such as `CONNECT`
returns an actionable error.

## Metadata and visibility

Explicit tags take precedence. Otherwise, an unequivocal Arandu module name is
used as the operation tag. Route deprecation produces `deprecated: true` and
preserves public dates in `x-arandu-deprecated-since` and `x-arandu-sunset`.
Domain metadata is preserved as `x-arandu-domain`; it is not promoted to an
OpenAPI server because current routing metadata does not prove host enforcement.

Use `Deprecated()` to mark only the documented operation. Use `Hidden()` or
its alias `Ignore()` to exclude a sensitive route from every generated
document. `Extension("x-example-audience", "partner")` adds a JSON-snapshotted
operation extension; extension keys must start with `x-`.

Fallback routes and Swagger's own routes are always excluded. Undocumented
routes are excluded unless `IncludeUndocumented` is enabled.
