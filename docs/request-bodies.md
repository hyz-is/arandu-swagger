# Request bodies

Request bodies are always explicit. The package does not inspect handlers,
decode DTOs, or infer media types.

## JSON with a native schema

```go
createUser := jsonschema.Object(
	jsonschema.Prop("name", jsonschema.String().Min(1).Required()),
	jsonschema.Prop("email", jsonschema.String().Format("email").Required()),
)

docs.Route(route).
	RequestBody(
		swagger.JSON(createUser).Required().
			Example(map[string]any{
				"name":  "Ada",
				"email": "ada@example.test",
			}),
	)
```

`JSON` snapshots a native `hesape/jsonschema.Type`. `JSONRef("CreateUser")`
references a registered schema component. `Required` belongs to the media
builder for fluent use; if any supplied representation is marked required, the
containing OpenAPI request body is required.

## Multiple content types

```go
docs.Route(route).RequestBody(
	swagger.JSONRef("ImportRequest").Required(),
	swagger.MediaRef("application/vnd.example.import+json", "ImportRequest"),
)
```

Use `MediaOf(contentType, nativeSchema)` for a native schema and
`MediaRef(contentType, component)` for a reference. Content types must parse as
valid media types; normalized equivalents, including case and parameter-order
variants, cannot appear twice in one body.

## Examples and descriptions

One inline example uses `Example`. Named examples use `NamedExample`:

```go
docs.Route(route).
	RequestBody(
		swagger.JSONRef("CreateUser").Required().
			NamedExample(
				"minimal",
				"Minimal user",
				"Only required fields are present.",
				map[string]any{"name": "Ada", "email": "ada@example.test"},
			),
	).
	RequestBodyDescription("The user to create.")
```

Examples are data written by the developer. Never insert captured requests,
database records, credentials, tokens, or production user data.

The typed OpenAPI model also exposes `RequestBody`, `Content`, `MediaType`, and
`Encoding` for reusable components. Register one on the module and reference it
from an operation when several routes share the same body contract:

```go
createUserRef := swagger.SchemaRef("CreateUser")
if err := docs.RequestBody("CreateUserBody", swagger.RequestBody{
	Required: true,
	Content: swagger.Content{
		"application/json": &swagger.MediaType{Schema: &createUserRef},
	},
}); err != nil {
	return err
}

docs.Route(route).RequestBodyRef("CreateUserBody")
```

Route-local fluent builders cover the common API case.

## Property encodings

`MediaType.Encoding` is available only for request bodies whose media type is
`application/x-www-form-urlencoded` or a `multipart/*` type. Each entry is
validated as an Encoding Object: its key must name a property that can be
resolved from the inline or local schema, `ContentType` accepts one media type,
a wildcard, or a comma-separated list, `Style` accepts the query serialization
styles, and nested headers must be valid Header Objects. Property membership in
an external schema reference cannot be inspected locally and remains the
caller's responsibility.

```go
formSchema, err := swagger.SchemaFrom(jsonschema.Object(
	jsonschema.Prop("avatar", jsonschema.String().Required()),
))
if err != nil {
	return err
}

if err := docs.RequestBody("AvatarForm", swagger.RequestBody{
	Content: swagger.Content{
		"multipart/form-data": &swagger.MediaType{
			Schema: &formSchema,
			Encoding: map[string]*swagger.Encoding{
				"avatar": {ContentType: "image/png, image/jpeg"},
			},
		},
	},
}); err != nil {
	return err
}
```

An encoding on JSON, response, parameter, or header content is rejected rather
than emitted in a location where OpenAPI ignores or forbids it.
