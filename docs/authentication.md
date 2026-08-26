# Authentication documentation

Security schemes describe authentication in OpenAPI. They do not authenticate
the documentation endpoints or change application middleware. Arandu Swagger
never infers security from middleware names.

Register schemes once on the module instance and reference them from each
operation:

```go
if err := docs.SecurityScheme("bearerAuth", swagger.HTTPBearer("JWT")); err != nil {
	return err
}

docs.Route(route).Security("bearerAuth")
```

Missing security-scheme references are generation errors.

## HTTP and API keys

```go
_ = docs.SecurityScheme("bearerAuth", swagger.HTTPBearer("JWT"))
_ = docs.SecurityScheme("basicAuth", swagger.HTTPBasic())
_ = docs.SecurityScheme("headerKey", swagger.APIKey("X-API-Key", swagger.APIKeyInHeader))
_ = docs.SecurityScheme("queryKey", swagger.APIKey("api_key", swagger.APIKeyInQuery))
_ = docs.SecurityScheme("cookieKey", swagger.APIKey("session", swagger.APIKeyInCookie))
_ = docs.SecurityScheme("clientCertificate", swagger.MutualTLS())
```

API key locations and required names are validated. The examples omit error
handling only to show the scheme constructors; production bootstrap should
return every registration error.

## OAuth 2.0

```go
oauth := swagger.OAuth2(swagger.OAuthFlows{
	AuthorizationCode: &swagger.OAuthFlow{
		AuthorizationURL: "https://identity.example.test/oauth/authorize",
		TokenURL:         "https://identity.example.test/oauth/token",
		Scopes: map[string]string{
			"users:read":  "Read users",
			"users:write": "Create and update users",
		},
	},
})
if err := docs.SecurityScheme("oauth", oauth); err != nil {
	return err
}

docs.Route(route).Security("oauth", "users:read")
```

Implicit, password, client-credentials, and authorization-code flow objects
are represented. Required authorization and token URLs must be absolute
HTTP(S) URLs, and every flow must declare a scopes object even when empty.

The embedded BaseLayout does not add a package-owned OAuth callback, client
registration, or secret handling. Those remain application and identity
provider concerns. Do not put client secrets, access tokens, or real API keys
in schemes or examples.

## OpenID Connect

```go
if err := docs.SecurityScheme(
	"openID",
	swagger.OpenIDConnect("https://identity.example.test/.well-known/openid-configuration"),
); err != nil {
	return err
}
```

The discovery URL must be absolute HTTP(S).

## OR and AND requirements

Calling `Security` twice creates two OpenAPI alternatives, meaning OR:

```go
docs.Route(route).
	Security("bearerAuth").
	Security("headerKey")
```

Use `SecurityAll` for schemes required together, meaning AND:

```go
docs.Route(route).SecurityAll(swagger.SecurityRequirement{
	"clientCertificate": nil,
	"headerKey":         nil,
})
```

`SecurityRequirementFor` is the convenience constructor for one named scheme.
An empty scope slice is encoded as the OpenAPI-required empty JSON array.

## Protecting `/docs`

Use `UIMiddleware` and `SpecMiddleware` to protect the documentation itself.
That access control is independent of the schemes described in the document;
see [production](production.md).
