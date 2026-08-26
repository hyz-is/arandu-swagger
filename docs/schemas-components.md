# Schemas and components

Arandu Swagger uses the native `github.com/arandu-io/hesape/jsonschema`
builders. It does not define another string, number, object, array, enum,
required-property, bound, or pattern builder.

```go
userSchema := jsonschema.Object(
	jsonschema.Prop("id", jsonschema.String().Format("uuid").Required()),
	jsonschema.Prop("name", jsonschema.String().Min(1).Required()),
	jsonschema.Prop("roles", jsonschema.Array().Items(jsonschema.String()).Unique()),
)

if err := docs.Schema("User", userSchema); err != nil {
	return err
}
```

For an existing OpenAPI contract that uses Schema Object keywords outside the
native Hesape type set, import one JSON-encoded schema through the validated
boundary:

```go
messageSchema, err := swagger.SchemaFromJSON([]byte(`{
	"type": "object",
	"properties": {
		"id": {"type": "integer", "format": "int64"}
	},
	"oneOf": [
		{"required": ["id"]},
		{"required": ["externalId"]}
	]
}`))
if err != nil {
	return err
}
if err := docs.SchemaComponent("Message", messageSchema); err != nil {
	return err
}
```

`SchemaFromJSON` is an import boundary, not a second builder. It accepts one
Schema Object, rejects malformed structures and duplicate keys, validates
nested schemas, preserves exact JSON numbers, and snapshots the caller's
bytes. Use native Hesape builders whenever they can express the contract.

`Schema` serializes and validates a structural JSON Schema snapshot at
registration time, catches schema marshal panics as errors, and stores immutable
JSON. It rejects duplicate object keys, invalid types and URI references,
non-portable patterns, empty or duplicate `enum`/`type` values, invalid or
duplicate `required` entries, negative or non-integer size bounds, and
`multipleOf` values at or below zero. Cross-keyword relationships such as a
minimum exceeding a maximum are not checked. A later mutation of the original
builder cannot change the registered component. Registration of semantically
equal set-like content under the same name is idempotent. Different content
under the same name is a conflict and remains a generation diagnostic.

The package also validates references and OpenAPI placement. It does not run the
complete JSON Schema meta-schema or implement every cross-keyword semantic rule.
Schema construction remains owned by Hesape for the native subset; use
`SchemaFromJSON` only to import an existing Schema Object that subset cannot
represent faithfully.

Component names must contain only letters, digits, `.`, `_`, and `-`.

## References and inline schemas

Use `JSON(native)` for an inline JSON Schema and `JSONRef(name)` for a local
component. For typed model construction:

```go
inline, err := swagger.SchemaFrom(jsonschema.String().Format("uuid"))
if err != nil {
	return err
}

local := swagger.SchemaRef("User")
external := swagger.SchemaReference("https://schemas.example.test/problem.json")
```

`SchemaRef` escapes the component name as a JSON Pointer token. Local schema
references are checked against the registry during generation. They may point
to the component root or to a nested Schema Object such as
`#/components/schemas/User/properties/id`; every JSON Pointer token and the
final target are validated. Exact external references are emitted as supplied
and remain the caller's responsibility. URI-reference syntax is checked when a
`Schema` is marshaled as well as during generation. The embedded UI's default
CSP uses `connect-src 'self'`, so a cross-origin schema reference is intended
for programmatic consumers unless the application makes an explicit CSP and
network-exposure decision.

The generated document declares OpenAPI `3.1.0` and JSON Schema dialect
`https://json-schema.org/draft/2020-12/schema`.

## Component kinds

One module instance can register:

| Method | OpenAPI component section |
| --- | --- |
| `Schema` | `schemas` |
| `SchemaComponent` | `schemas` imported through `SchemaFromJSON` |
| `Parameter` | `parameters` |
| `Response` | `responses` |
| `RequestBody` | `requestBodies` |
| `Header` | `headers` |
| `ExampleComponent` | `examples` |
| `SecurityScheme` | `securitySchemes` |

The public typed model additionally represents callbacks and reusable path
items, although the first registry API does not provide dedicated registration
methods for those two advanced sections.

## Examples and extensions

`Example.Value` is `json.RawMessage`; callers constructing it directly must
provide valid JSON. Specification extensions use `swagger.Extensions`, also
with raw JSON values:

```go
document.Extensions = swagger.Extensions{
	"x-audience": json.RawMessage(`"partners"`),
}
```

The route builder's `Extension` method is safer for ordinary values because it
performs JSON encoding and snapshots the result.
