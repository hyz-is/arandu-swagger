---
name: swagger-security
description: Review or change Arandu Swagger exposure security, including Enabled defaults, route filters, sensitive operations, middleware, authorization persistence, Try it out, CSP, and public errors.
license: MIT
---

# Securing documentation exposure

This package has no policy, repository, session integration, or authentication
system. Access control is application-owned middleware passed through typed
configuration.

## Safe defaults

- `Config{}` is disabled.
- Only routes explicitly attached to the instance registry are included.
- Fallback routes, `/_arandu`, Swagger's own endpoints, and hidden operations
  are excluded.
- Public generation errors contain no internal route or schema details;
  diagnostics retain the actionable error for operators.
- UI assets are embedded, same-origin, allowlisted, and served with `nosniff`.
- The UI CSP permits no inline scripts and limits connections to self.

`IncludeUndocumented`, `IncludeInternal`, and `PersistAuthorization` widen
exposure or retention and require an explicit application decision.
`DisableTryItOut` removes interactive submission. Prefer it for documentation
that is intended only for reading.

## Protecting endpoints

Attach authentication and authorization middleware independently:

```go
swagger.Config{
	Enabled:        true,
	Title:          "Internal API",
	Version:        "1.0.0",
	UIMiddleware:   []swagger.Middleware{requireAdmin},
	SpecMiddleware: []swagger.Middleware{requireAdmin},
}
```

Protect both surfaces when the schema itself is sensitive. Disabling the UI
does not protect the JSON endpoint. Disabling the public specification requires
disabling the UI because the embedded UI reads that endpoint.

Use route filters as defense in depth, not as identity checks. Predicates
receive static route metadata, not a request or user. Mark especially sensitive
operations with `Hidden` or `Ignore` at registration.

## Data rules

Never derive examples from requests, databases, sessions, or configuration.
Never publish real passwords, tokens, API keys, cookies, authorization headers,
or user data. Examples and server URLs are explicit code-owned documentation.

Review generated JSON in CI with the public generation API when documentation
is part of a release contract. A generation error must fail the check rather
than be replaced by a partial document.

Cross-origin Try it out is intentionally blocked by the default CSP. Supporting
it requires an application-level CSP and CORS design; do not weaken the package
default globally for one deployment.
