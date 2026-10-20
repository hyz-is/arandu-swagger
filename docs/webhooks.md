# Webhooks

An OpenAPI 3.1 document describes, beside the requests an API answers, the
requests it sends: the top-level `webhooks` section, keyed by a name and
holding one Path Item per event. Swagger UI 5 shows them in their own section
below the operations.

A webhook is not a route of the application's router, so the route table --
the source of every path -- cannot describe it. Arandu Swagger takes webhooks
as typed `Webhook` values registered on the same instance-owned registry as
the routes and components. The values come from the application's own
catalogue of the events it sends, the same list a subscription is validated
against, so the document cannot name an event a subscription refuses or miss
one it accepts, and no second, hand-written specification is kept.

## Registering

```go
// The payload is a Go type the application already encodes.
if err := docs.Models(
	swagger.ModelOf[resources.Order]("Order"),
	swagger.ModelOf[resources.OrderEvent]("OrderEvent"),
); err != nil {
	return App{}, err
}

webhooks := make([]swagger.Webhook, 0, len(events.Names()))
for _, name := range events.Names() {
	webhooks = append(webhooks, swagger.Webhook{
		Name:        name,
		OperationID: "webhooks:" + name,
		Summary:     "Sent when " + name + " happens.",
		Description: "Signed with Standard Webhooks; answer 2xx.",
		Tags:        []string{"webhooks"},
		Body:        []*swagger.Media{swagger.JSONRef("OrderEvent")},
		Responses:   swagger.Responses{"2XX": {Description: "Received."}},
		Deprecated:  events.Retired(name),
	})
}
if err := docs.Webhooks(webhooks...); err != nil {
	return App{}, err
}
```

`Module.Webhooks` and `Registry.Webhooks` are the same operation; the standalone
`Generate` reads the registry, so a static contract export and the served
`/docs` page carry the same webhooks when both are fed by one function of the
application.

When a body needs a schema per event -- for example an envelope whose `type`
is the event's own name and whose `data` references a component -- snapshot it
with `SchemaFromJSON` and pass it with `MediaSchema("application/json", schema)`.

## Fields

| Field | Meaning |
| --- | --- |
| `Name` | key in `webhooks`, usually the event name (`order.paid`); not blank, no surrounding whitespace |
| `Method` | HTTP method the provider sends with; empty means `POST`; one name may carry several methods |
| `OperationID` | optional; unique across webhooks and route operations |
| `Summary`, `Description`, `Tags`, `ExternalDocs`, `Deprecated` | operation metadata |
| `Parameters` | header, query, or cookie values the provider sends (a signature header); path parameters are refused |
| `Body` | representations of the request body; a declared body is always `required: true` |
| `Responses` | at least one; keys are `default`, a status from 100 through 599, or a range `1XX` through `5XX` |
| `Security` | requirements naming registered security schemes |
| `Extensions` | `x-*` specification extensions |

## Rules

- Values are snapshotted when registered; later changes to the caller's maps
  and slices do not reach the document.
- Registering the same webhook again with the same content is idempotent and
  does not invalidate the cache; a different webhook under the same name and
  method is an error.
- A call registers all of its webhooks or none of them. A refused registration
  is recorded and makes generation fail, as a refused component does.
- Generation validates body, parameter, response, and security references
  against the registered components and reports errors with the webhook's name
  and method.
- `Config.Filter`, `IncludeUndocumented`, and `IncludeInternal` select routes;
  they never remove a webhook.
- A document with no registered webhooks has no `webhooks` key.
