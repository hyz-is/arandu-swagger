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

## Components derived from Go types

The payload an application answers with is usually a Go type it hands to
`encoding/json`. Describing that payload again with builders is a second copy
of the contract, and the copy drifts: a field renamed in the struct and not in
the schema is a client generated against a property the server stopped
sending. `Models` derives the component from the type itself:

```go
type Card struct {
	Brand string  `json:"brand"`
	Last4 *string `json:"last4"`
}

type PaymentMethod struct {
	ID        string    `json:"id"`
	Type      string    `json:"type" enum:"card"`
	Card      Card      `json:"card"`
	Amount    int64     `json:"amount_cents" doc:"Amount in cents."`
	CreatedAt time.Time `json:"created_at"`
	Note      string    `json:"note,omitempty"`
}

type Envelope[T any] struct {
	Data T `json:"data"`
}

if err := docs.Models(
	swagger.ModelOf[Card]("Card"),
	swagger.ModelOf[PaymentMethod]("PaymentMethod"),
	swagger.ModelOf[Envelope[PaymentMethod]]("PaymentMethodEnvelope"),
); err != nil {
	return err
}
```

`PaymentMethod` becomes:

```json
{"type":"object","required":["id","type","card","amount_cents","created_at"],"properties":{
  "id":{"type":"string"},"type":{"type":"string","enum":["card"]},
  "card":{"$ref":"#/components/schemas/Card"},
  "amount_cents":{"type":"integer","description":"Amount in cents."},
  "created_at":{"type":"string","format":"date-time"},"note":{"type":"string"}}}
```

The derivation follows what `encoding/json` writes:

| Go | schema |
| --- | --- |
| a struct | an object of its exported fields, named and skipped by their `json` tags, in declaration order; an embedded struct's fields are promoted |
| a field without `omitempty` or `omitzero` | required, because it is always written |
| `string`, `bool`, every integer, `float32`, `float64` | `string`, `boolean`, `integer`, `number` |
| `time.Time` | a `string` of format `date-time` |
| a slice or an array | an `array` of its element |
| a map with string keys | an `object` whose `additionalProperties` is its value |
| `json.RawMessage`, an interface | any value, `{}` |
| a pointer | its element or null: `["string","null"]`, or `anyOf` with `{"type":"null"}` for an object, an array or a reference |
| a struct type another model names | a `$ref` to that component, wherever it appears below the root |

Three struct tags say what the type cannot: `doc` is the description,
`format` the format, and `enum` the values, comma-separated. Keywords are
written in one fixed order, so the same types derive the same bytes.

A type the derivation cannot say -- a channel, a function, a map with keys
that are not strings, `[]byte`, an unnamed struct that contains itself -- and
a `json` option it does not follow, such as `string`, are errors naming the
model and the field. Every derived schema passes the validation
`SchemaFromJSON` applies. `SchemasFromModels` answers the same schemas without
registering them.

A nil slice or map is written by `encoding/json` as `null`, while the schema
says `array` or `object`: fill them, or declare the field as a pointer.

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
| `Models` | `schemas` derived from Go types |
| `Parameter` | `parameters` |
| `Response` | `responses` |
| `RequestBody` | `requestBodies` |
| `Header` | `headers` |
| `ExampleComponent` | `examples` |
| `SecurityScheme` | `securitySchemes` |

The public typed model additionally represents callbacks and reusable path
items, although the registry API does not provide dedicated registration
methods for those two advanced sections. Webhooks, the document's top-level
`webhooks` section, are registered with `Webhooks`; see
[Webhooks](webhooks.md).

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
