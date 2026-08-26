# Configuration

`swagger.New` accepts one typed `swagger.Config` and returns an error when an
enabled configuration cannot be served safely. `Config{}` is a valid complete
opt-out: it creates a disabled module and registers no HTTP routes.

## Identity and document metadata

| Field | Required when enabled | Meaning |
| --- | --- | --- |
| `Enabled` | — | registers endpoints only when true |
| `Title` | yes | OpenAPI `info.title` |
| `Version` | yes | documented API version, not the package version |
| `Summary` | no | short API summary |
| `Description` | no | full API description |
| `TermsOfService` | no | absolute HTTP(S) URL |
| `Contact` | no | `Name`, absolute `URL`, and valid `Email` |
| `License` | no | required `Name`, plus either SPDX `Identifier` or absolute `URL` |
| `Servers` | no | server URLs and validated template variables |
| `Tags` | no | document-level tag descriptions; names must be unique |
| `ExternalDocs` | no | description and absolute HTTP(S) URL |

OpenAPI 3.1 does not permit both `License.Identifier` and `License.URL` on the
same license object. Server URL variables must be declared exactly once, must
have a default, and, when an enum is supplied, that enum must be non-empty and
contain the default.

All static metadata in this table is validated by `Config.Validate` and
`swagger.New`, so a malformed URL, contact, license, server, tag, or external
documentation link fails application wiring instead of the first spec request.

```go
config := swagger.Config{
	Enabled:     true,
	Title:       "Billing API",
	Version:     "2026-08",
	Summary:     "Invoice and payment operations",
	Description: "The tenant-scoped billing HTTP API.",
	Contact: &swagger.Contact{
		Name:  "API team",
		Email: "api@example.test",
	},
	License: &swagger.License{Name: "MIT", Identifier: "MIT"},
	Servers: []swagger.Server{{
		URL: "https://{tenant}.example.test",
		Variables: map[string]swagger.ServerVariable{
			"tenant": {Default: "demo", Description: "Tenant subdomain"},
		},
	}},
	Tags: []swagger.Tag{{Name: "Invoices", Description: "Invoice operations"}},
}
```

## Endpoints

| Field | Default | Meaning |
| --- | --- | --- |
| `UIPath` | `/docs` | exact UI route |
| `SpecPath` | `<UIPath>/openapi.json` | exact OpenAPI JSON route |
| `DisableUI` | false | keeps JSON but removes UI, initializer, and assets |
| `DisableSpec` | false | removes the public JSON endpoint |

Endpoint paths must be absolute, clean, static paths. They cannot claim `/`,
end with `/`, contain query strings, fragments, whitespace, parameters,
wildcards, or use the reserved `/_arandu` namespace. UI and specification paths
must be distinct, and `SpecPath` cannot collide with the UI initializer or asset
namespace.

`DisableSpec` requires `DisableUI`, because the embedded UI reads the public
specification endpoint. Set both switches to true for programmatic-only
generation. Set only `DisableUI` to expose JSON without an interactive UI.

## Selection, cache, and UI behavior

| Field | Default | Meaning |
| --- | --- | --- |
| `IncludeUndocumented` | false | publishes basic operations for eligible routes that were not explicitly documented |
| `IncludeInternal` | false | allows `/_arandu` routes to pass the package default filter |
| `Filter` | empty | include/exclude filters by route metadata and predicates |
| `CacheSpec` | false | caches serialized JSON until registry or route-table metadata changes |
| `PersistAuthorization` | false | asks Swagger UI to retain authorization data in the browser |
| `DisableTryItOut` | false | removes all interactive submit methods from Swagger UI |
| `UIMiddleware` | empty | wraps the UI, initializer, redirect, and asset handlers |
| `SpecMiddleware` | empty | wraps only the OpenAPI JSON handler |

Nil middleware values are rejected. See [filters](filters.md) and
[production](production.md) for the security and lifecycle implications.
