# Responses

Every explicitly documented operation needs at least one response. A response
description is required even for an empty response such as `204 No Content`.
The package never guesses status codes from handlers.

```go
docs.Route(route).
	Response(http.StatusOK, "User found", swagger.JSONRef("User")).
	Response(http.StatusNotFound, "User not found", swagger.JSONRef("Problem")).
	DefaultResponse("Unexpected problem", swagger.JSONRef("Problem"))
```

Omit media for a response with no body:

```go
docs.Route(route).Response(http.StatusNoContent, "User deleted")
```

The same `JSON`, `JSONRef`, `MediaOf`, and `MediaRef` helpers used for request
bodies declare response representations. `Example` and `NamedExample` are
available on each media value.

## Response headers

Add a typed header to a concrete response:

```go
requestID, err := swagger.SchemaFrom(jsonschema.String())
if err != nil {
	return err
}

docs.Route(route).
	Response(http.StatusOK, "User found", swagger.JSONRef("User")).
	ResponseHeader(http.StatusOK, "X-Request-ID", swagger.Header{
		Description: "Identifier assigned to this request.",
		Schema:      &requestID,
	})
```

Calling `ResponseHeader` before `Response` is accepted by the builder so calls
can be composed, but generation still reports a missing response description
if no response declaration completes that status.

## Reusable responses

Register a component and refer to it by status:

```go
problemSchema := swagger.SchemaRef("Problem")
problem := swagger.Response{
	Description: "Problem response",
	Content: swagger.Content{
		"application/json": {Schema: &problemSchema},
	},
}
if err := docs.Response("Problem", problem); err != nil {
	return err
}

docs.Route(route).ResponseRef(http.StatusBadRequest, "Problem")
```

For ordinary JSON responses, route-local `JSONRef` is shorter. `ResponseRef`
is useful when description, headers, and all media metadata are shared. Missing
local references are generation errors.
