# Parameters

Arandu Swagger supports path, query, header, and cookie parameters. Parameter
schemas are native Hesape JSON Schemas and are snapshotted when documented.

## Path parameters

Path parameters are discovered from the route and always emitted with
`required: true`, as OpenAPI requires. An omitted documentation call still
produces a conservative string schema:

```go
route := r.Get("/users/{id}", handler).Name("users.show").WhereUuid("id")
docs.Route(route).Response(http.StatusOK, "User found")
```

Add explicit description, schema, and example metadata when useful:

```go
docs.Route(route).
	PathParameter(
		"id",
		swagger.Description("User identifier."),
		swagger.ParameterSchemaRef("UserID"),
		swagger.ExampleValue("01900000-0000-7000-8000-000000000000"),
	).
	Response(http.StatusOK, "User found", swagger.JSONRef("User"))
```

An explicitly documented path parameter that is not present in the route is a
generation error. The Hesape route constraint is preserved as a string
`pattern` when it is portable to OpenAPI's ECMAScript regex dialect. Current
public route metadata exposes the final pattern but not whether it came from
`WhereNumber`, `WhereUuid`, `WhereUlid`, `WhereIn`, or a generic `Where` call.
The generator therefore does not narrow it to integer, format, or enum and risk
changing the runtime contract.

Optional placeholders such as `{id?}` cannot be represented as an optional
OpenAPI path parameter and are rejected. Register two concrete routes when the
application needs both shapes. Multi-segment wildcards such as `{path...}` are
also rejected because one OpenAPI path parameter represents one segment.

Hesape's qualified binding syntax, such as `{post:slug}`, is expected to be
normalized by the router to `{post}` while retaining `slug` as binding metadata.
If the raw qualified placeholder reaches generation, Arandu Swagger rejects it
instead of publishing a path template whose parameter name disagrees with the
runtime route. This is an upstream routing defect, not a second binding syntax
owned by this package.

## Query, header, and cookie parameters

These locations require an explicit native schema:

```go
docs.Route(route).
	QueryParameter(
		"include",
		jsonschema.String().Enum("profile", "permissions"),
		swagger.Description("Relations to include."),
	).
	HeaderParameter(
		"X-Trace-ID",
		jsonschema.String(),
		swagger.Description("Caller trace identifier."),
		swagger.ParameterRequired(),
	).
	CookieParameter(
		"locale",
		jsonschema.String().Enum("en", "pt-BR"),
		swagger.ParameterDeprecated(),
	)
```

Available parameter options are:

| Option | Effect |
| --- | --- |
| `Description(text)` | sets parameter description |
| `ExampleValue(value)` | snapshots one JSON example |
| `ParameterSchema(schema)` | replaces the schema with a native Hesape schema |
| `ParameterSchemaRef(name)` | references `#/components/schemas/<name>` |
| `ParameterRequired()` | marks a non-path parameter required |
| `ParameterDeprecated()` | marks the parameter deprecated |

## Reusable parameters

Create an immutable schema snapshot, register a typed parameter component, and
reference it from operations:

```go
traceID, err := swagger.SchemaFrom(jsonschema.String())
if err != nil {
	return err
}
if err := docs.Parameter("TraceID", swagger.Parameter{
	Name:        "X-Trace-ID",
	In:          swagger.ParameterInHeader,
	Description: "Caller trace identifier.",
	Schema:      &traceID,
}); err != nil {
	return err
}

docs.Route(route).ParameterRef("TraceID")
```

Local references are validated during generation.
